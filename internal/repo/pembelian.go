package repo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/tokobangunan/internal/domain"
)

// ListPembelianFilter filter list pembelian.
type ListPembelianFilter struct {
	SupplierID  int64
	GudangID    int64
	StatusBayar string
	DariTanggal *time.Time
	SampaiTanggal *time.Time
	Page    int
	PerPage int
}

// PembelianRepo akses tabel pembelian + pembelian_item.
type PembelianRepo struct {
	pool *pgxpool.Pool
}

// NewPembelianRepo konstruktor.
func NewPembelianRepo(pool *pgxpool.Pool) *PembelianRepo {
	return &PembelianRepo{pool: pool}
}

const pembelianColumns = `id, nomor_pembelian, tanggal, supplier_id, gudang_id, user_id,
	subtotal, diskon, dpp, ppn_persen, ppn_amount, total,
	status_bayar, jatuh_tempo, catatan, created_at, updated_at`

func scanPembelian(row pgx.Row, p *domain.Pembelian) error {
	var status string
	var catatan *string
	if err := row.Scan(&p.ID, &p.NomorPembelian, &p.Tanggal, &p.SupplierID, &p.GudangID, &p.UserID,
		&p.Subtotal, &p.Diskon, &p.DPP, &p.PPNPersen, &p.PPNAmount, &p.Total,
		&status, &p.JatuhTempo, &catatan,
		&p.CreatedAt, &p.UpdatedAt); err != nil {
		return err
	}
	p.StatusBayar = domain.StatusBayarPembelian(status)
	if catatan != nil {
		p.Catatan = *catatan
	}
	return nil
}

// Create insert header + items dalam transaction. Trigger akan tambah stok.
func (r *PembelianRepo) Create(ctx context.Context, p *domain.Pembelian) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx pembelian: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const insHeader = `
		INSERT INTO pembelian (nomor_pembelian, tanggal, supplier_id, gudang_id, user_id,
			subtotal, diskon, dpp, ppn_persen, ppn_amount, total,
			status_bayar, jatuh_tempo, catatan)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		RETURNING id, created_at, updated_at`
	var catatan *string
	if strings.TrimSpace(p.Catatan) != "" {
		c := p.Catatan
		catatan = &c
	}
	if err := tx.QueryRow(ctx, insHeader,
		p.NomorPembelian, p.Tanggal, p.SupplierID, p.GudangID, p.UserID,
		p.Subtotal, p.Diskon, p.DPP, p.PPNPersen, p.PPNAmount, p.Total,
		string(p.StatusBayar), p.JatuhTempo, catatan,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return fmt.Errorf("insert pembelian: %w", err)
	}

	const insItem = `
		INSERT INTO pembelian_item (pembelian_id, produk_id, produk_nama, qty,
			satuan_id, satuan_kode, qty_konversi, harga_satuan, subtotal)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id`
	for i := range p.Items {
		it := &p.Items[i]
		it.PembelianID = p.ID
		if err := tx.QueryRow(ctx, insItem,
			p.ID, it.ProdukID, it.ProdukNama, it.Qty,
			it.SatuanID, it.SatuanKode, it.QtyKonversi, it.HargaSatuan, it.Subtotal,
		).Scan(&it.ID); err != nil {
			return fmt.Errorf("insert pembelian_item: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit pembelian: %w", err)
	}
	return nil
}

// GetByID load header pembelian + join nama supplier/gudang/user.
func (r *PembelianRepo) GetByID(ctx context.Context, id int64) (*domain.Pembelian, error) {
	const sql = `
		SELECT p.id, p.nomor_pembelian, p.tanggal, p.supplier_id, p.gudang_id, p.user_id,
			p.subtotal, p.diskon, p.dpp, p.ppn_persen, p.ppn_amount, p.total,
			p.status_bayar, p.jatuh_tempo, p.catatan,
			p.created_at, p.updated_at,
			s.nama, g.nama, u.nama_lengkap
		FROM pembelian p
		JOIN supplier s ON s.id = p.supplier_id
		JOIN gudang g ON g.id = p.gudang_id
		JOIN "user" u ON u.id = p.user_id
		WHERE p.id = $1`
	var p domain.Pembelian
	var status string
	var catatan *string
	err := r.pool.QueryRow(ctx, sql, id).Scan(
		&p.ID, &p.NomorPembelian, &p.Tanggal, &p.SupplierID, &p.GudangID, &p.UserID,
		&p.Subtotal, &p.Diskon, &p.DPP, &p.PPNPersen, &p.PPNAmount, &p.Total,
		&status, &p.JatuhTempo, &catatan,
		&p.CreatedAt, &p.UpdatedAt,
		&p.SupplierNama, &p.GudangNama, &p.UserNama,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrPembelianTidakDitemukan
	}
	if err != nil {
		return nil, fmt.Errorf("get pembelian: %w", err)
	}
	p.StatusBayar = domain.StatusBayarPembelian(status)
	if catatan != nil {
		p.Catatan = *catatan
	}
	return &p, nil
}

// LoadItems isi p.Items dari DB.
func (r *PembelianRepo) LoadItems(ctx context.Context, p *domain.Pembelian) error {
	const sql = `
		SELECT id, pembelian_id, produk_id, produk_nama, qty, satuan_id, satuan_kode,
			qty_konversi, harga_satuan, subtotal
		FROM pembelian_item
		WHERE pembelian_id = $1
		ORDER BY id ASC`
	rows, err := r.pool.Query(ctx, sql, p.ID)
	if err != nil {
		return fmt.Errorf("load pembelian items: %w", err)
	}
	defer rows.Close()

	items := make([]domain.PembelianItem, 0, 8)
	for rows.Next() {
		var it domain.PembelianItem
		if err := rows.Scan(&it.ID, &it.PembelianID, &it.ProdukID, &it.ProdukNama, &it.Qty,
			&it.SatuanID, &it.SatuanKode, &it.QtyKonversi, &it.HargaSatuan, &it.Subtotal); err != nil {
			return fmt.Errorf("scan pembelian item: %w", err)
		}
		items = append(items, it)
	}
	p.Items = items
	return rows.Err()
}

// List paginasi + filter.
func (r *PembelianRepo) List(ctx context.Context, f ListPembelianFilter) ([]domain.Pembelian, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PerPage <= 0 {
		f.PerPage = 25
	}
	conds := []string{"1=1"}
	args := []any{}
	idx := 1
	if f.SupplierID > 0 {
		conds = append(conds, fmt.Sprintf("p.supplier_id = $%d", idx))
		args = append(args, f.SupplierID)
		idx++
	}
	if f.GudangID > 0 {
		conds = append(conds, fmt.Sprintf("p.gudang_id = $%d", idx))
		args = append(args, f.GudangID)
		idx++
	}
	if s := strings.TrimSpace(f.StatusBayar); s != "" {
		conds = append(conds, fmt.Sprintf("p.status_bayar = $%d", idx))
		args = append(args, s)
		idx++
	}
	if f.DariTanggal != nil {
		conds = append(conds, fmt.Sprintf("p.tanggal >= $%d", idx))
		args = append(args, *f.DariTanggal)
		idx++
	}
	if f.SampaiTanggal != nil {
		conds = append(conds, fmt.Sprintf("p.tanggal <= $%d", idx))
		args = append(args, *f.SampaiTanggal)
		idx++
	}
	where := strings.Join(conds, " AND ")

	var total int
	if err := r.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM pembelian p WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count pembelian: %w", err)
	}

	offset := (f.Page - 1) * f.PerPage
	listSQL := fmt.Sprintf(`
		SELECT p.id, p.nomor_pembelian, p.tanggal, p.supplier_id, p.gudang_id, p.user_id,
			p.subtotal, p.diskon, p.dpp, p.ppn_persen, p.ppn_amount, p.total,
			p.status_bayar, p.jatuh_tempo, p.catatan,
			p.created_at, p.updated_at,
			s.nama, g.nama
		FROM pembelian p
		JOIN supplier s ON s.id = p.supplier_id
		JOIN gudang g ON g.id = p.gudang_id
		WHERE %s
		ORDER BY p.tanggal DESC, p.id DESC
		LIMIT $%d OFFSET $%d`, where, idx, idx+1)
	args = append(args, f.PerPage, offset)

	rows, err := r.pool.Query(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list pembelian: %w", err)
	}
	defer rows.Close()

	out := make([]domain.Pembelian, 0, f.PerPage)
	for rows.Next() {
		var p domain.Pembelian
		var status string
		var catatan *string
		if err := rows.Scan(
			&p.ID, &p.NomorPembelian, &p.Tanggal, &p.SupplierID, &p.GudangID, &p.UserID,
			&p.Subtotal, &p.Diskon, &p.DPP, &p.PPNPersen, &p.PPNAmount, &p.Total,
			&status, &p.JatuhTempo, &catatan,
			&p.CreatedAt, &p.UpdatedAt,
			&p.SupplierNama, &p.GudangNama,
		); err != nil {
			return nil, 0, fmt.Errorf("scan pembelian: %w", err)
		}
		p.StatusBayar = domain.StatusBayarPembelian(status)
		if catatan != nil {
			p.Catatan = *catatan
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}

// NextNomor generate nomor pembelian BLI/YYYY/MM/NNNN per bulan.
func (r *PembelianRepo) NextNomor(ctx context.Context, tanggal time.Time) (string, error) {
	year := tanggal.Year()
	month := int(tanggal.Month())
	prefix := fmt.Sprintf("BLI/%04d/%02d/", year, month)

	const sql = `
		SELECT COALESCE(MAX(
			CAST(SUBSTRING(nomor_pembelian FROM '([0-9]+)$') AS INTEGER)
		), 0)
		FROM pembelian
		WHERE nomor_pembelian LIKE $1`
	var maxN int
	if err := r.pool.QueryRow(ctx, sql, prefix+"%").Scan(&maxN); err != nil {
		return "", fmt.Errorf("next nomor pembelian: %w", err)
	}
	return fmt.Sprintf("%s%04d", prefix, maxN+1), nil
}

// OutstandingBySupplier hitung sisa hutang per supplier (total - sum bayar).
func (r *PembelianRepo) OutstandingBySupplier(ctx context.Context, supplierID int64) (int64, error) {
	const sql = `
		SELECT COALESCE((
			SELECT SUM(total) FROM pembelian
			WHERE supplier_id = $1 AND status_bayar IN ('kredit','sebagian')
		), 0) - COALESCE((
			SELECT SUM(jumlah) FROM pembayaran_supplier
			WHERE supplier_id = $1
		), 0)`
	var v int64
	if err := r.pool.QueryRow(ctx, sql, supplierID).Scan(&v); err != nil {
		return 0, fmt.Errorf("outstanding supplier: %w", err)
	}
	if v < 0 {
		v = 0
	}
	return v, nil
}

// UpdateStatusBayar update status pembelian (dipakai service saat record payment).
func (r *PembelianRepo) UpdateStatusBayar(ctx context.Context, id int64, status domain.StatusBayarPembelian) error {
	const sql = `UPDATE pembelian SET status_bayar = $2 WHERE id = $1`
	tag, err := r.pool.Exec(ctx, sql, id, string(status))
	if err != nil {
		return fmt.Errorf("update status pembelian: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrPembelianTidakDitemukan
	}
	return nil
}
