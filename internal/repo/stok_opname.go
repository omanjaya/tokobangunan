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

// StokSnapshot row stok per produk untuk pre-fill opname draft.
type StokSnapshot struct {
	ProdukID   int64
	ProdukNama string
	QtySistem  float64
}

// ListStokOpnameFilter filter list opname.
type ListStokOpnameFilter struct {
	GudangID      int64
	Status        string
	DariTanggal   *time.Time
	SampaiTanggal *time.Time
	Page          int
	PerPage       int
}

// StokOpnameRepo akses tabel stok_opname + stok_opname_item.
type StokOpnameRepo struct {
	pool *pgxpool.Pool
}

// NewStokOpnameRepo konstruktor.
func NewStokOpnameRepo(pool *pgxpool.Pool) *StokOpnameRepo {
	return &StokOpnameRepo{pool: pool}
}

// Create insert header opname (status default = draft).
func (r *StokOpnameRepo) Create(ctx context.Context, o *domain.StokOpname) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx opname: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const insHeader = `
		INSERT INTO stok_opname (nomor, gudang_id, tanggal, user_id, status, catatan)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, created_at, updated_at`
	var catatan *string
	if strings.TrimSpace(o.Catatan) != "" {
		c := o.Catatan
		catatan = &c
	}
	if err := tx.QueryRow(ctx, insHeader,
		o.Nomor, o.GudangID, o.Tanggal, o.UserID, string(o.Status), catatan,
	).Scan(&o.ID, &o.CreatedAt, &o.UpdatedAt); err != nil {
		return fmt.Errorf("insert stok_opname: %w", err)
	}

	if len(o.Items) > 0 {
		const insItem = `
			INSERT INTO stok_opname_item (opname_id, produk_id, produk_nama,
				qty_sistem, qty_fisik, keterangan)
			VALUES ($1,$2,$3,$4,$5,$6)
			RETURNING id`
		for i := range o.Items {
			it := &o.Items[i]
			it.OpnameID = o.ID
			if err := tx.QueryRow(ctx, insItem,
				o.ID, it.ProdukID, it.ProdukNama, it.QtySistem, it.QtyFisik, it.Keterangan,
			).Scan(&it.ID); err != nil {
				return fmt.Errorf("insert stok_opname_item: %w", err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit stok_opname: %w", err)
	}
	return nil
}

// GetByID load header + nama gudang/user.
func (r *StokOpnameRepo) GetByID(ctx context.Context, id int64) (*domain.StokOpname, error) {
	const sql = `
		SELECT o.id, o.nomor, o.gudang_id, o.tanggal, o.user_id, o.status,
			COALESCE(o.catatan,''), o.created_at, o.updated_at,
			g.nama, u.nama_lengkap
		FROM stok_opname o
		JOIN gudang g ON g.id = o.gudang_id
		JOIN "user" u ON u.id = o.user_id
		WHERE o.id = $1`
	var o domain.StokOpname
	var status string
	err := r.pool.QueryRow(ctx, sql, id).Scan(
		&o.ID, &o.Nomor, &o.GudangID, &o.Tanggal, &o.UserID, &status,
		&o.Catatan, &o.CreatedAt, &o.UpdatedAt,
		&o.GudangNama, &o.UserNama,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrOpnameTidakDitemukan
	}
	if err != nil {
		return nil, fmt.Errorf("get stok_opname: %w", err)
	}
	o.Status = domain.StatusOpname(status)
	return &o, nil
}

// LoadItems isi o.Items dari DB.
func (r *StokOpnameRepo) LoadItems(ctx context.Context, o *domain.StokOpname) error {
	const sql = `
		SELECT id, opname_id, produk_id, produk_nama, qty_sistem, qty_fisik, selisih,
			COALESCE(keterangan,'')
		FROM stok_opname_item
		WHERE opname_id = $1
		ORDER BY produk_nama ASC`
	rows, err := r.pool.Query(ctx, sql, o.ID)
	if err != nil {
		return fmt.Errorf("load opname items: %w", err)
	}
	defer rows.Close()

	items := make([]domain.StokOpnameItem, 0, 32)
	for rows.Next() {
		var it domain.StokOpnameItem
		if err := rows.Scan(&it.ID, &it.OpnameID, &it.ProdukID, &it.ProdukNama,
			&it.QtySistem, &it.QtyFisik, &it.Selisih, &it.Keterangan); err != nil {
			return fmt.Errorf("scan opname item: %w", err)
		}
		items = append(items, it)
	}
	o.Items = items
	return rows.Err()
}

// List paginasi.
func (r *StokOpnameRepo) List(ctx context.Context, f ListStokOpnameFilter) ([]domain.StokOpname, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PerPage <= 0 {
		f.PerPage = 25
	}
	conds := []string{"1=1"}
	args := []any{}
	idx := 1
	if f.GudangID > 0 {
		conds = append(conds, fmt.Sprintf("o.gudang_id = $%d", idx))
		args = append(args, f.GudangID)
		idx++
	}
	if s := strings.TrimSpace(f.Status); s != "" {
		conds = append(conds, fmt.Sprintf("o.status = $%d", idx))
		args = append(args, s)
		idx++
	}
	if f.DariTanggal != nil {
		conds = append(conds, fmt.Sprintf("o.tanggal >= $%d", idx))
		args = append(args, *f.DariTanggal)
		idx++
	}
	if f.SampaiTanggal != nil {
		conds = append(conds, fmt.Sprintf("o.tanggal <= $%d", idx))
		args = append(args, *f.SampaiTanggal)
		idx++
	}
	where := strings.Join(conds, " AND ")

	var total int
	if err := r.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM stok_opname o WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count opname: %w", err)
	}

	offset := (f.Page - 1) * f.PerPage
	listSQL := fmt.Sprintf(`
		SELECT o.id, o.nomor, o.gudang_id, o.tanggal, o.user_id, o.status,
			COALESCE(o.catatan,''), o.created_at, o.updated_at,
			g.nama
		FROM stok_opname o
		JOIN gudang g ON g.id = o.gudang_id
		WHERE %s
		ORDER BY o.tanggal DESC, o.id DESC
		LIMIT $%d OFFSET $%d`, where, idx, idx+1)
	args = append(args, f.PerPage, offset)

	rows, err := r.pool.Query(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list opname: %w", err)
	}
	defer rows.Close()

	out := make([]domain.StokOpname, 0, f.PerPage)
	for rows.Next() {
		var o domain.StokOpname
		var status string
		if err := rows.Scan(
			&o.ID, &o.Nomor, &o.GudangID, &o.Tanggal, &o.UserID, &status,
			&o.Catatan, &o.CreatedAt, &o.UpdatedAt,
			&o.GudangNama,
		); err != nil {
			return nil, 0, fmt.Errorf("scan opname: %w", err)
		}
		o.Status = domain.StatusOpname(status)
		out = append(out, o)
	}
	return out, total, rows.Err()
}

// UpdateStatus ubah status (trigger akan adjust stok kalau jadi approved).
func (r *StokOpnameRepo) UpdateStatus(ctx context.Context, id int64, newStatus domain.StatusOpname) error {
	const sql = `UPDATE stok_opname SET status = $2 WHERE id = $1`
	tag, err := r.pool.Exec(ctx, sql, id, string(newStatus))
	if err != nil {
		return fmt.Errorf("update status opname: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrOpnameTidakDitemukan
	}
	return nil
}

// UpsertItem update qty_fisik & keterangan satu item (ON CONFLICT do update).
func (r *StokOpnameRepo) UpsertItem(ctx context.Context, opnameID, produkID int64, qtySistem, qtyFisik float64, keterangan string) error {
	const sql = `
		INSERT INTO stok_opname_item (opname_id, produk_id, produk_nama, qty_sistem, qty_fisik, keterangan)
		VALUES ($1, $2,
			COALESCE((SELECT nama FROM produk WHERE id = $2), ''),
			$3, $4, $5)
		ON CONFLICT (opname_id, produk_id) DO UPDATE SET
			qty_fisik = EXCLUDED.qty_fisik,
			keterangan = EXCLUDED.keterangan`
	if _, err := r.pool.Exec(ctx, sql, opnameID, produkID, qtySistem, qtyFisik, keterangan); err != nil {
		return fmt.Errorf("upsert opname item: %w", err)
	}
	return nil
}

// LoadCurrentStokForGudang ambil snapshot stok produk aktif di gudang
// untuk pre-fill items opname draft. Asume tabel `stok` sudah ada (Fase 3).
func (r *StokOpnameRepo) LoadCurrentStokForGudang(ctx context.Context, gudangID int64) ([]StokSnapshot, error) {
	const sql = `
		SELECT p.id, p.nama, COALESCE(s.qty, 0)
		FROM produk p
		LEFT JOIN stok s ON s.produk_id = p.id AND s.gudang_id = $1
		WHERE p.deleted_at IS NULL AND p.is_active = TRUE
		ORDER BY p.nama ASC`
	rows, err := r.pool.Query(ctx, sql, gudangID)
	if err != nil {
		return nil, fmt.Errorf("load stok snapshot: %w", err)
	}
	defer rows.Close()

	out := make([]StokSnapshot, 0, 64)
	for rows.Next() {
		var s StokSnapshot
		if err := rows.Scan(&s.ProdukID, &s.ProdukNama, &s.QtySistem); err != nil {
			return nil, fmt.Errorf("scan stok snapshot: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// NextNomor generate OPN/YYYY/MM/NNNN.
func (r *StokOpnameRepo) NextNomor(ctx context.Context, tanggal time.Time) (string, error) {
	year := tanggal.Year()
	month := int(tanggal.Month())
	prefix := fmt.Sprintf("OPN/%04d/%02d/", year, month)

	const sql = `
		SELECT COALESCE(MAX(
			CAST(SUBSTRING(nomor FROM '([0-9]+)$') AS INTEGER)
		), 0)
		FROM stok_opname
		WHERE nomor LIKE $1`
	var maxN int
	if err := r.pool.QueryRow(ctx, sql, prefix+"%").Scan(&maxN); err != nil {
		return "", fmt.Errorf("next nomor opname: %w", err)
	}
	return fmt.Sprintf("%s%04d", prefix, maxN+1), nil
}
