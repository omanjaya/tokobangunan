package repo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/tokobangunan/internal/domain"
)

// ListPenjualanFilter - parameter listing penjualan.
type ListPenjualanFilter struct {
	From     *time.Time
	To       *time.Time
	GudangID *int64
	MitraID  *int64
	Status   *string
	Query    string // search by nomor kwitansi
	Page     int
	PerPage  int
}

// Normalize set default Page=1, PerPage=25 (max 100).
func (f *ListPenjualanFilter) Normalize() {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PerPage <= 0 {
		f.PerPage = 25
	}
	if f.PerPage > 100 {
		f.PerPage = 100
	}
}

// PenjualanRepo akses tabel penjualan + penjualan_item (raw pgx + transaction).
type PenjualanRepo struct {
	pool *pgxpool.Pool
}

func NewPenjualanRepo(pool *pgxpool.Pool) *PenjualanRepo {
	return &PenjualanRepo{pool: pool}
}

const penjualanColumns = `id, nomor_kwitansi, tanggal, mitra_id, gudang_id, user_id,
	subtotal, diskon, dpp, ppn_persen, ppn_amount, total,
	status_bayar, jatuh_tempo, catatan, client_uuid,
	created_at, updated_at`

func scanPenjualan(row pgx.Row, p *domain.Penjualan) error {
	var status string
	var catatan *string
	if err := row.Scan(&p.ID, &p.NomorKwitansi, &p.Tanggal, &p.MitraID, &p.GudangID,
		&p.UserID, &p.Subtotal, &p.Diskon, &p.DPP, &p.PPNPersen, &p.PPNAmount, &p.Total,
		&status, &p.JatuhTempo,
		&catatan, &p.ClientUUID, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return err
	}
	p.StatusBayar = domain.StatusBayar(status)
	if catatan != nil {
		p.Catatan = *catatan
	}
	return nil
}

// Create insert header + items dalam 1 transaction.
func (r *PenjualanRepo) Create(ctx context.Context, p *domain.Penjualan) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const insertHeader = `INSERT INTO penjualan
		(nomor_kwitansi, tanggal, mitra_id, gudang_id, user_id,
		 subtotal, diskon, dpp, ppn_persen, ppn_amount, total,
		 status_bayar, jatuh_tempo, catatan, client_uuid)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		RETURNING id, created_at, updated_at`

	var catatan *string
	if s := strings.TrimSpace(p.Catatan); s != "" {
		catatan = &s
	}
	row := tx.QueryRow(ctx, insertHeader,
		p.NomorKwitansi, p.Tanggal, p.MitraID, p.GudangID, p.UserID,
		p.Subtotal, p.Diskon, p.DPP, p.PPNPersen, p.PPNAmount, p.Total,
		string(p.StatusBayar), p.JatuhTempo,
		catatan, p.ClientUUID,
	)
	if err := row.Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			// nomor_kwitansi atau client_uuid duplikat
			return fmt.Errorf("nomor/uuid duplikat: %w", err)
		}
		return fmt.Errorf("insert penjualan: %w", err)
	}

	const insertItem = `INSERT INTO penjualan_item
		(penjualan_id, penjualan_tanggal, produk_id, produk_nama,
		 qty, satuan_id, satuan_kode, qty_konversi, harga_satuan, diskon, subtotal)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id`

	for i := range p.Items {
		it := &p.Items[i]
		if err := tx.QueryRow(ctx, insertItem,
			p.ID, p.Tanggal, it.ProdukID, it.ProdukNama,
			it.Qty, it.SatuanID, it.SatuanKode, it.QtyKonversi,
			it.HargaSatuan, it.Diskon, it.Subtotal,
		).Scan(&it.ID); err != nil {
			return fmt.Errorf("insert item[%d]: %w", i, err)
		}
	}

	// Decrement stok di gudang per produk dalam tx yang sama dengan invoice.
	// Aggregasi qty_konversi per produk_id supaya multi-line untuk produk
	// yang sama hanya satu UPDATE. SELECT ... FOR UPDATE mengunci row stok
	// agar dua transaksi paralel tidak melewati guard stok masing-masing.
	need := make(map[int64]float64, len(p.Items))
	for _, it := range p.Items {
		need[it.ProdukID] += it.QtyKonversi
	}
	for produkID, qty := range need {
		// Lock row stok (atau wujudkan row 0 bila belum ada) lalu cek+update.
		var current float64
		err := tx.QueryRow(ctx,
			`SELECT qty FROM stok WHERE gudang_id = $1 AND produk_id = $2 FOR UPDATE`,
			p.GudangID, produkID,
		).Scan(&current)
		if errors.Is(err, pgx.ErrNoRows) {
			// Belum ada baris stok → qty=0, butuh decrement berarti error.
			if qty > 0 {
				return fmt.Errorf("stok produk %d tidak cukup (0 < %.4f)", produkID, qty)
			}
			continue
		} else if err != nil {
			return fmt.Errorf("lock stok produk %d: %w", produkID, err)
		}
		if current < qty {
			return fmt.Errorf("stok produk %d tidak cukup (%.4f < %.4f)", produkID, current, qty)
		}
		if _, err := tx.Exec(ctx,
			`UPDATE stok SET qty = qty - $3, updated_at = now()
				WHERE gudang_id = $1 AND produk_id = $2`,
			p.GudangID, produkID, qty,
		); err != nil {
			return fmt.Errorf("update stok produk %d: %w", produkID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// GetByID lookup by id (scan all partitions). Tanggal optional untuk pruning.
func (r *PenjualanRepo) GetByID(ctx context.Context, id int64, tanggal *time.Time) (*domain.Penjualan, error) {
	var sql string
	var args []any
	if tanggal != nil {
		sql = `SELECT ` + penjualanColumns + ` FROM penjualan WHERE id = $1 AND tanggal = $2`
		args = []any{id, *tanggal}
	} else {
		sql = `SELECT ` + penjualanColumns + ` FROM penjualan WHERE id = $1`
		args = []any{id}
	}
	var p domain.Penjualan
	if err := scanPenjualan(r.pool.QueryRow(ctx, sql, args...), &p); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrPenjualanNotFound
		}
		return nil, fmt.Errorf("get penjualan: %w", err)
	}
	return &p, nil
}

// GetByNomor lookup by nomor kwitansi (unique).
func (r *PenjualanRepo) GetByNomor(ctx context.Context, nomor string) (*domain.Penjualan, error) {
	const sql = `SELECT ` + penjualanColumns + ` FROM penjualan WHERE nomor_kwitansi = $1`
	var p domain.Penjualan
	if err := scanPenjualan(r.pool.QueryRow(ctx, sql, nomor), &p); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrPenjualanNotFound
		}
		return nil, fmt.Errorf("get penjualan by nomor: %w", err)
	}
	return &p, nil
}

// GetByClientUUID untuk idempotency check.
func (r *PenjualanRepo) GetByClientUUID(ctx context.Context, u uuid.UUID) (*domain.Penjualan, error) {
	const sql = `SELECT ` + penjualanColumns + ` FROM penjualan WHERE client_uuid = $1`
	var p domain.Penjualan
	if err := scanPenjualan(r.pool.QueryRow(ctx, sql, u), &p); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrPenjualanNotFound
		}
		return nil, fmt.Errorf("get penjualan by uuid: %w", err)
	}
	return &p, nil
}

// LoadItems populate p.Items dari penjualan_item.
func (r *PenjualanRepo) LoadItems(ctx context.Context, p *domain.Penjualan) error {
	const sql = `SELECT id, produk_id, produk_nama, qty, satuan_id, satuan_kode,
		qty_konversi, harga_satuan, diskon, subtotal
		FROM penjualan_item
		WHERE penjualan_id = $1 AND penjualan_tanggal = $2
		ORDER BY id ASC`
	rows, err := r.pool.Query(ctx, sql, p.ID, p.Tanggal)
	if err != nil {
		return fmt.Errorf("load items: %w", err)
	}
	defer rows.Close()

	out := make([]domain.PenjualanItem, 0, 8)
	for rows.Next() {
		var it domain.PenjualanItem
		if err := rows.Scan(&it.ID, &it.ProdukID, &it.ProdukNama, &it.Qty,
			&it.SatuanID, &it.SatuanKode, &it.QtyKonversi, &it.HargaSatuan,
			&it.Diskon, &it.Subtotal); err != nil {
			return fmt.Errorf("scan item: %w", err)
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iter items: %w", err)
	}
	p.Items = out
	return nil
}

// List dengan pagination + filter tanggal/gudang/mitra/status.
func (r *PenjualanRepo) List(ctx context.Context, f ListPenjualanFilter) ([]domain.Penjualan, int, error) {
	f.Normalize()

	conds := []string{"1=1"}
	args := []any{}
	idx := 1

	if f.From != nil {
		conds = append(conds, fmt.Sprintf("tanggal >= $%d", idx))
		args = append(args, *f.From)
		idx++
	}
	if f.To != nil {
		conds = append(conds, fmt.Sprintf("tanggal <= $%d", idx))
		args = append(args, *f.To)
		idx++
	}
	if f.GudangID != nil {
		conds = append(conds, fmt.Sprintf("gudang_id = $%d", idx))
		args = append(args, *f.GudangID)
		idx++
	}
	if f.MitraID != nil {
		conds = append(conds, fmt.Sprintf("mitra_id = $%d", idx))
		args = append(args, *f.MitraID)
		idx++
	}
	if f.Status != nil && *f.Status != "" {
		conds = append(conds, fmt.Sprintf("status_bayar = $%d", idx))
		args = append(args, *f.Status)
		idx++
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		conds = append(conds, fmt.Sprintf("nomor_kwitansi ILIKE $%d", idx))
		args = append(args, "%"+q+"%")
		idx++
	}

	where := "WHERE " + strings.Join(conds, " AND ")

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM penjualan "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count penjualan: %w", err)
	}

	offset := (f.Page - 1) * f.PerPage
	listSQL := fmt.Sprintf(
		`SELECT %s FROM penjualan %s ORDER BY tanggal DESC, id DESC LIMIT $%d OFFSET $%d`,
		penjualanColumns, where, idx, idx+1,
	)
	args = append(args, f.PerPage, offset)

	rows, err := r.pool.Query(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query penjualan: %w", err)
	}
	defer rows.Close()

	out := make([]domain.Penjualan, 0, f.PerPage)
	for rows.Next() {
		var p domain.Penjualan
		if err := scanPenjualan(rows, &p); err != nil {
			return nil, 0, fmt.Errorf("scan penjualan: %w", err)
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}

// PenjualanWithRelations - row penjualan + display name relasi (mitra, gudang).
// Dipakai di list view untuk menghindari N+1 lookup per row.
type PenjualanWithRelations struct {
	Penjualan  domain.Penjualan
	MitraNama  string
	GudangKode string
	GudangNama string
}

// ListWithRelations sama seperti List, tapi sekaligus LEFT JOIN mitra & gudang
// supaya display name (mitra.nama, gudang.kode/nama) ikut ter-fetch dalam 1 query.
// LEFT JOIN dipakai agar baris yang relasi-nya sudah dihapus tidak hilang.
func (r *PenjualanRepo) ListWithRelations(ctx context.Context, f ListPenjualanFilter) ([]PenjualanWithRelations, int, error) {
	f.Normalize()

	conds := []string{"1=1"}
	args := []any{}
	idx := 1

	if f.From != nil {
		conds = append(conds, fmt.Sprintf("p.tanggal >= $%d", idx))
		args = append(args, *f.From)
		idx++
	}
	if f.To != nil {
		conds = append(conds, fmt.Sprintf("p.tanggal <= $%d", idx))
		args = append(args, *f.To)
		idx++
	}
	if f.GudangID != nil {
		conds = append(conds, fmt.Sprintf("p.gudang_id = $%d", idx))
		args = append(args, *f.GudangID)
		idx++
	}
	if f.MitraID != nil {
		conds = append(conds, fmt.Sprintf("p.mitra_id = $%d", idx))
		args = append(args, *f.MitraID)
		idx++
	}
	if f.Status != nil && *f.Status != "" {
		conds = append(conds, fmt.Sprintf("p.status_bayar = $%d", idx))
		args = append(args, *f.Status)
		idx++
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		conds = append(conds, fmt.Sprintf("p.nomor_kwitansi ILIKE $%d", idx))
		args = append(args, "%"+q+"%")
		idx++
	}

	where := "WHERE " + strings.Join(conds, " AND ")

	var total int
	if err := r.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM penjualan p "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count penjualan: %w", err)
	}

	offset := (f.Page - 1) * f.PerPage
	listSQL := fmt.Sprintf(
		`SELECT p.id, p.nomor_kwitansi, p.tanggal, p.mitra_id, p.gudang_id, p.user_id,
			p.subtotal, p.diskon, p.dpp, p.ppn_persen, p.ppn_amount, p.total,
			p.status_bayar, p.jatuh_tempo, p.catatan, p.client_uuid,
			p.created_at, p.updated_at,
			m.nama, g.kode, g.nama
		 FROM penjualan p
		 LEFT JOIN mitra m  ON m.id = p.mitra_id
		 LEFT JOIN gudang g ON g.id = p.gudang_id
		 %s
		 ORDER BY p.tanggal DESC, p.id DESC
		 LIMIT $%d OFFSET $%d`,
		where, idx, idx+1,
	)
	args = append(args, f.PerPage, offset)

	rows, err := r.pool.Query(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query penjualan: %w", err)
	}
	defer rows.Close()

	out := make([]PenjualanWithRelations, 0, f.PerPage)
	for rows.Next() {
		var (
			row        PenjualanWithRelations
			p          = &row.Penjualan
			status     string
			catatan    *string
			mitraNama  *string
			gudangKode *string
			gudangNama *string
		)
		if err := rows.Scan(
			&p.ID, &p.NomorKwitansi, &p.Tanggal, &p.MitraID, &p.GudangID, &p.UserID,
			&p.Subtotal, &p.Diskon, &p.DPP, &p.PPNPersen, &p.PPNAmount, &p.Total,
			&status, &p.JatuhTempo, &catatan, &p.ClientUUID,
			&p.CreatedAt, &p.UpdatedAt,
			&mitraNama, &gudangKode, &gudangNama,
		); err != nil {
			return nil, 0, fmt.Errorf("scan penjualan: %w", err)
		}
		p.StatusBayar = domain.StatusBayar(status)
		if catatan != nil {
			p.Catatan = *catatan
		}
		if mitraNama != nil {
			row.MitraNama = *mitraNama
		}
		if gudangKode != nil {
			row.GudangKode = *gudangKode
		}
		if gudangNama != nil {
			row.GudangNama = *gudangNama
		}
		out = append(out, row)
	}
	return out, total, rows.Err()
}

// NextNomor generate nomor kwitansi `{KODE_GUDANG}/{YYYY}/{MM}/{NNNN}`.
// Increment per gudang per bulan, hitung MAX(nomor) yang prefix-match.
func (r *PenjualanRepo) NextNomor(ctx context.Context, kodeGudang string, tanggal time.Time) (string, error) {
	prefix := fmt.Sprintf("%s/%04d/%02d/", kodeGudang, tanggal.Year(), int(tanggal.Month()))
	// Cari nomor terbesar bulan ini lalu increment.
	const sql = `SELECT nomor_kwitansi FROM penjualan
		WHERE nomor_kwitansi LIKE $1
		ORDER BY nomor_kwitansi DESC
		LIMIT 1`
	var last string
	err := r.pool.QueryRow(ctx, sql, prefix+"%").Scan(&last)
	if errors.Is(err, pgx.ErrNoRows) {
		return prefix + "0001", nil
	}
	if err != nil {
		return "", fmt.Errorf("next nomor: %w", err)
	}
	// Parse 4-digit suffix.
	parts := strings.Split(last, "/")
	if len(parts) != 4 {
		return prefix + "0001", nil
	}
	var seq int
	_, _ = fmt.Sscanf(parts[3], "%d", &seq)
	seq++
	return fmt.Sprintf("%s%04d", prefix, seq), nil
}

// StokInfo - posisi stok 1 produk di 1 gudang + ambang minimum.
type StokInfo struct {
	Qty         float64
	StokMinimum float64
}

// StokInfoOf - lookup qty stok + stok_minimum produk untuk dipakai oleh
// Penjualan handler (badge stok di product picker). Disisipkan disini karena
// hanya butuh akses ke schema stok+produk dan sudah punya pool.
func (r *PenjualanRepo) StokInfoOf(ctx context.Context, gudangID, produkID int64) (StokInfo, error) {
	const sql = `SELECT
			COALESCE(s.qty, 0) AS qty,
			COALESCE(p.stok_minimum, 0) AS stok_minimum
		FROM produk p
		LEFT JOIN stok s ON s.produk_id = p.id AND s.gudang_id = $1
		WHERE p.id = $2`
	var info StokInfo
	err := r.pool.QueryRow(ctx, sql, gudangID, produkID).Scan(&info.Qty, &info.StokMinimum)
	if errors.Is(err, pgx.ErrNoRows) {
		return StokInfo{}, nil
	}
	if err != nil {
		return StokInfo{}, fmt.Errorf("get stok info: %w", err)
	}
	return info, nil
}

// SearchByNomor autocomplete sederhana.
func (r *PenjualanRepo) SearchByNomor(ctx context.Context, q string, limit int) ([]domain.Penjualan, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	const sql = `SELECT ` + penjualanColumns + ` FROM penjualan
		WHERE nomor_kwitansi ILIKE $1
		ORDER BY tanggal DESC, id DESC
		LIMIT $2`
	rows, err := r.pool.Query(ctx, sql, "%"+q+"%", limit)
	if err != nil {
		return nil, fmt.Errorf("search penjualan: %w", err)
	}
	defer rows.Close()
	out := make([]domain.Penjualan, 0, limit)
	for rows.Next() {
		var p domain.Penjualan
		if err := scanPenjualan(rows, &p); err != nil {
			return nil, fmt.Errorf("scan penjualan: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
