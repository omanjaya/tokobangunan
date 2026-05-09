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

// ListReturPenjualanFilter - filter list retur.
type ListReturPenjualanFilter struct {
	From    *time.Time
	To      *time.Time
	Page    int
	PerPage int
}

func (f *ListReturPenjualanFilter) Normalize() {
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

// ReturPenjualanRepo akses tabel retur_penjualan.
type ReturPenjualanRepo struct {
	pool *pgxpool.Pool
}

func NewReturPenjualanRepo(pool *pgxpool.Pool) *ReturPenjualanRepo {
	return &ReturPenjualanRepo{pool: pool}
}

func (r *ReturPenjualanRepo) Pool() *pgxpool.Pool { return r.pool }

const returColumns = `id, nomor_retur, penjualan_id, penjualan_tanggal, mitra_id,
	gudang_id, tanggal, alasan, COALESCE(catatan, ''), subtotal_refund, user_id, created_at`

func scanRetur(row pgx.Row, p *domain.ReturPenjualan) error {
	return row.Scan(&p.ID, &p.NomorRetur, &p.PenjualanID, &p.PenjualanTanggal,
		&p.MitraID, &p.GudangID, &p.Tanggal, &p.Alasan, &p.Catatan,
		&p.SubtotalRefund, &p.UserID, &p.CreatedAt)
}

// CreateInTx - insert header retur + items dalam tx existing.
// Caller harus sudah lock penjualan FOR UPDATE & sudah set p.NomorRetur.
func (r *ReturPenjualanRepo) CreateInTx(ctx context.Context, tx pgx.Tx, p *domain.ReturPenjualan) error {
	const insertHeader = `INSERT INTO retur_penjualan
		(nomor_retur, penjualan_id, penjualan_tanggal, mitra_id, gudang_id,
		 tanggal, alasan, catatan, subtotal_refund, user_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING id, created_at`
	var catatan *string
	if s := strings.TrimSpace(p.Catatan); s != "" {
		catatan = &s
	}
	if err := tx.QueryRow(ctx, insertHeader,
		p.NomorRetur, p.PenjualanID, p.PenjualanTanggal, p.MitraID, p.GudangID,
		p.Tanggal, p.Alasan, catatan, p.SubtotalRefund, p.UserID,
	).Scan(&p.ID, &p.CreatedAt); err != nil {
		return fmt.Errorf("insert retur header: %w", err)
	}

	const insertItem = `INSERT INTO retur_penjualan_item
		(retur_id, penjualan_item_id, produk_id, qty, qty_konversi,
		 satuan_id, harga_satuan, subtotal)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id`
	for i := range p.Items {
		it := &p.Items[i]
		it.ReturID = p.ID
		if err := tx.QueryRow(ctx, insertItem,
			p.ID, it.PenjualanItemID, it.ProdukID, it.Qty, it.QtyKonversi,
			it.SatuanID, it.HargaSatuan, it.Subtotal,
		).Scan(&it.ID); err != nil {
			return fmt.Errorf("insert retur item[%d]: %w", i, err)
		}
	}
	return nil
}

// GetByID lookup retur by id.
func (r *ReturPenjualanRepo) GetByID(ctx context.Context, id int64) (*domain.ReturPenjualan, error) {
	const sql = `SELECT ` + returColumns + ` FROM retur_penjualan WHERE id = $1`
	var p domain.ReturPenjualan
	if err := scanRetur(r.pool.QueryRow(ctx, sql, id), &p); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrReturPenjualanNotFound
		}
		return nil, fmt.Errorf("get retur: %w", err)
	}
	return &p, nil
}

// LoadItems populate p.Items + ProdukNama + SatuanKode via JOIN.
func (r *ReturPenjualanRepo) LoadItems(ctx context.Context, p *domain.ReturPenjualan) error {
	const sql = `SELECT i.id, i.retur_id, i.penjualan_item_id, i.produk_id,
		COALESCE(pr.nama, ''), i.qty, i.qty_konversi, i.satuan_id,
		COALESCE(s.kode, ''), i.harga_satuan, i.subtotal
		FROM retur_penjualan_item i
		LEFT JOIN produk pr ON pr.id = i.produk_id
		LEFT JOIN satuan s  ON s.id = i.satuan_id
		WHERE i.retur_id = $1
		ORDER BY i.id ASC`
	rows, err := r.pool.Query(ctx, sql, p.ID)
	if err != nil {
		return fmt.Errorf("load retur items: %w", err)
	}
	defer rows.Close()
	out := make([]domain.ReturPenjualanItem, 0, 8)
	for rows.Next() {
		var it domain.ReturPenjualanItem
		if err := rows.Scan(&it.ID, &it.ReturID, &it.PenjualanItemID, &it.ProdukID,
			&it.ProdukNama, &it.Qty, &it.QtyKonversi, &it.SatuanID,
			&it.SatuanKode, &it.HargaSatuan, &it.Subtotal); err != nil {
			return fmt.Errorf("scan retur item: %w", err)
		}
		out = append(out, it)
	}
	p.Items = out
	return rows.Err()
}

// List retur paginated; filter tanggal optional.
func (r *ReturPenjualanRepo) List(ctx context.Context, f ListReturPenjualanFilter) ([]domain.ReturPenjualan, int, error) {
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
	where := "WHERE " + strings.Join(conds, " AND ")

	var total int
	if err := r.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM retur_penjualan "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count retur: %w", err)
	}

	offset := (f.Page - 1) * f.PerPage
	listSQL := fmt.Sprintf(
		`SELECT %s FROM retur_penjualan %s ORDER BY tanggal DESC, id DESC LIMIT $%d OFFSET $%d`,
		returColumns, where, idx, idx+1,
	)
	args = append(args, f.PerPage, offset)

	rows, err := r.pool.Query(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query retur: %w", err)
	}
	defer rows.Close()
	out := make([]domain.ReturPenjualan, 0, f.PerPage)
	for rows.Next() {
		var p domain.ReturPenjualan
		if err := scanRetur(rows, &p); err != nil {
			return nil, 0, fmt.Errorf("scan retur: %w", err)
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}

// SumQtyByPenjualanItemTx - total qty (small unit) yang sudah diretur untuk
// satu penjualan_item, dipakai untuk cek qty available di tx yang sama.
func (r *ReturPenjualanRepo) SumQtyByPenjualanItemTx(ctx context.Context, tx pgx.Tx, penjualanItemID int64) (float64, error) {
	const sql = `SELECT COALESCE(SUM(qty_konversi), 0)
		FROM retur_penjualan_item WHERE penjualan_item_id = $1`
	var v float64
	if err := tx.QueryRow(ctx, sql, penjualanItemID).Scan(&v); err != nil {
		return 0, fmt.Errorf("sum retur qty: %w", err)
	}
	return v, nil
}

// NextNomor - generate nomor retur RTR-<gudang_kode>-<YYMMDD>-<seq>.
func (r *ReturPenjualanRepo) NextNomor(ctx context.Context, gudangKode string, tanggal time.Time) (string, error) {
	prefix := fmt.Sprintf("RTR-%s-%s-", gudangKode, tanggal.Format("060102"))
	const sql = `SELECT nomor_retur FROM retur_penjualan
		WHERE nomor_retur LIKE $1
		ORDER BY nomor_retur DESC LIMIT 1`
	var last string
	err := r.pool.QueryRow(ctx, sql, prefix+"%").Scan(&last)
	if errors.Is(err, pgx.ErrNoRows) {
		return prefix + "001", nil
	}
	if err != nil {
		return "", fmt.Errorf("next nomor retur: %w", err)
	}
	parts := strings.Split(last, "-")
	if len(parts) < 4 {
		return prefix + "001", nil
	}
	var seq int
	_, _ = fmt.Sscanf(parts[len(parts)-1], "%d", &seq)
	seq++
	return fmt.Sprintf("%s%03d", prefix, seq), nil
}

// ListWithRelations - retur + nama mitra + nomor invoice + kode gudang.
type ReturWithRelations struct {
	Retur         domain.ReturPenjualan
	NomorKwitansi string
	MitraNama     string
	GudangKode    string
}

func (r *ReturPenjualanRepo) ListWithRelations(ctx context.Context, f ListReturPenjualanFilter) ([]ReturWithRelations, int, error) {
	f.Normalize()
	conds := []string{"1=1"}
	args := []any{}
	idx := 1
	if f.From != nil {
		conds = append(conds, fmt.Sprintf("r.tanggal >= $%d", idx))
		args = append(args, *f.From)
		idx++
	}
	if f.To != nil {
		conds = append(conds, fmt.Sprintf("r.tanggal <= $%d", idx))
		args = append(args, *f.To)
		idx++
	}
	where := "WHERE " + strings.Join(conds, " AND ")

	var total int
	if err := r.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM retur_penjualan r "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count retur: %w", err)
	}

	offset := (f.Page - 1) * f.PerPage
	listSQL := fmt.Sprintf(
		`SELECT r.id, r.nomor_retur, r.penjualan_id, r.penjualan_tanggal, r.mitra_id,
			r.gudang_id, r.tanggal, r.alasan, COALESCE(r.catatan, ''), r.subtotal_refund,
			r.user_id, r.created_at,
			COALESCE(p.nomor_kwitansi, ''), COALESCE(m.nama, ''), COALESCE(g.kode, '')
		 FROM retur_penjualan r
		 LEFT JOIN penjualan p ON p.id = r.penjualan_id AND p.tanggal = r.penjualan_tanggal
		 LEFT JOIN mitra m     ON m.id = r.mitra_id
		 LEFT JOIN gudang g    ON g.id = r.gudang_id
		 %s
		 ORDER BY r.tanggal DESC, r.id DESC
		 LIMIT $%d OFFSET $%d`,
		where, idx, idx+1,
	)
	args = append(args, f.PerPage, offset)
	rows, err := r.pool.Query(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query retur: %w", err)
	}
	defer rows.Close()
	out := make([]ReturWithRelations, 0, f.PerPage)
	for rows.Next() {
		var row ReturWithRelations
		p := &row.Retur
		if err := rows.Scan(&p.ID, &p.NomorRetur, &p.PenjualanID, &p.PenjualanTanggal,
			&p.MitraID, &p.GudangID, &p.Tanggal, &p.Alasan, &p.Catatan,
			&p.SubtotalRefund, &p.UserID, &p.CreatedAt,
			&row.NomorKwitansi, &row.MitraNama, &row.GudangKode); err != nil {
			return nil, 0, fmt.Errorf("scan retur: %w", err)
		}
		out = append(out, row)
	}
	return out, total, rows.Err()
}
