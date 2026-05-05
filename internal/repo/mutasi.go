package repo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/tokobangunan/internal/domain"
)

// ListMutasiFilter - filter listing mutasi.
type ListMutasiFilter struct {
	From           *time.Time
	To             *time.Time
	GudangAsalID   *int64
	GudangTujuanID *int64
	// UserScopeGudangID — kalau non-nil, paksa baris hanya yg
	// gudang_asal_id ATAU gudang_tujuan_id sama dgn nilai ini.
	// Dipakai untuk authorization scoping (kasir/staff per-gudang).
	UserScopeGudangID *int64
	Status            *string
	Page              int
	PerPage           int
}

// Normalize set default Page=1, PerPage=25 (max 100).
func (f *ListMutasiFilter) Normalize() {
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

// MutasiRepo akses tabel mutasi_gudang + mutasi_item.
type MutasiRepo struct {
	pool *pgxpool.Pool
}

func NewMutasiRepo(pool *pgxpool.Pool) *MutasiRepo {
	return &MutasiRepo{pool: pool}
}

const mutasiColumns = `id, nomor_mutasi, tanggal, gudang_asal_id, gudang_tujuan_id,
	status, user_pengirim_id, user_penerima_id, tanggal_kirim, tanggal_terima,
	catatan, client_uuid, created_at, updated_at`

func scanMutasi(row pgx.Row, m *domain.MutasiGudang) error {
	var status string
	var clientUUID uuid.UUID
	if err := row.Scan(&m.ID, &m.NomorMutasi, &m.Tanggal, &m.GudangAsalID, &m.GudangTujuanID,
		&status, &m.UserPengirimID, &m.UserPenerimaID, &m.TanggalKirim, &m.TanggalTerima,
		&m.Catatan, &clientUUID, &m.CreatedAt, &m.UpdatedAt); err != nil {
		return err
	}
	m.Status = domain.StatusMutasi(status)
	m.ClientUUID = clientUUID
	return nil
}

// Create - transactional insert header + line items.
func (r *MutasiRepo) Create(ctx context.Context, m *domain.MutasiGudang) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx mutasi: %w", err)
	}
	defer tx.Rollback(ctx)

	const insertHeader = `
		INSERT INTO mutasi_gudang (nomor_mutasi, tanggal, gudang_asal_id, gudang_tujuan_id,
			status, user_pengirim_id, catatan, client_uuid)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at`
	if err := tx.QueryRow(ctx, insertHeader,
		m.NomorMutasi, m.Tanggal, m.GudangAsalID, m.GudangTujuanID,
		string(m.Status), m.UserPengirimID, m.Catatan, m.ClientUUID,
	).Scan(&m.ID, &m.CreatedAt, &m.UpdatedAt); err != nil {
		return fmt.Errorf("insert mutasi header: %w", err)
	}

	const insertItem = `
		INSERT INTO mutasi_item (mutasi_id, produk_id, produk_nama, qty,
			satuan_id, satuan_kode, qty_konversi, harga_internal, catatan)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id`
	for i := range m.Items {
		it := &m.Items[i]
		it.MutasiID = m.ID
		if err := tx.QueryRow(ctx, insertItem,
			it.MutasiID, it.ProdukID, it.ProdukNama, it.Qty,
			it.SatuanID, it.SatuanKode, it.QtyKonversi, it.HargaInternal, it.Catatan,
		).Scan(&it.ID); err != nil {
			return fmt.Errorf("insert mutasi item: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// GetByID load header (tanpa items, panggil LoadItems terpisah).
func (r *MutasiRepo) GetByID(ctx context.Context, id int64) (*domain.MutasiGudang, error) {
	const sql = `SELECT ` + mutasiColumns + ` FROM mutasi_gudang WHERE id = $1`
	row := r.pool.QueryRow(ctx, sql, id)
	var m domain.MutasiGudang
	if err := scanMutasi(row, &m); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrMutasiNotFound
		}
		return nil, fmt.Errorf("get mutasi: %w", err)
	}
	return &m, nil
}

// LoadItems isi field Items pada mutasi.
func (r *MutasiRepo) LoadItems(ctx context.Context, m *domain.MutasiGudang) error {
	const sql = `
		SELECT id, mutasi_id, produk_id, produk_nama, qty, satuan_id, satuan_kode,
		       qty_konversi, harga_internal, catatan
		FROM mutasi_item
		WHERE mutasi_id = $1
		ORDER BY id ASC`
	rows, err := r.pool.Query(ctx, sql, m.ID)
	if err != nil {
		return fmt.Errorf("query mutasi item: %w", err)
	}
	defer rows.Close()
	out := make([]domain.MutasiItem, 0, 8)
	for rows.Next() {
		var it domain.MutasiItem
		if err := rows.Scan(&it.ID, &it.MutasiID, &it.ProdukID, &it.ProdukNama,
			&it.Qty, &it.SatuanID, &it.SatuanKode, &it.QtyKonversi,
			&it.HargaInternal, &it.Catatan); err != nil {
			return fmt.Errorf("scan mutasi item: %w", err)
		}
		out = append(out, it)
	}
	m.Items = out
	return rows.Err()
}

// List - return rows (tanpa items) + total count.
func (r *MutasiRepo) List(ctx context.Context, f ListMutasiFilter) ([]domain.MutasiGudang, int, error) {
	f.Normalize()

	where := []string{"1=1"}
	args := []any{}
	idx := 1

	if f.From != nil {
		where = append(where, fmt.Sprintf("tanggal >= $%d", idx))
		args = append(args, *f.From)
		idx++
	}
	if f.To != nil {
		where = append(where, fmt.Sprintf("tanggal <= $%d", idx))
		args = append(args, *f.To)
		idx++
	}
	if f.GudangAsalID != nil {
		where = append(where, fmt.Sprintf("gudang_asal_id = $%d", idx))
		args = append(args, *f.GudangAsalID)
		idx++
	}
	if f.GudangTujuanID != nil {
		where = append(where, fmt.Sprintf("gudang_tujuan_id = $%d", idx))
		args = append(args, *f.GudangTujuanID)
		idx++
	}
	if f.UserScopeGudangID != nil {
		where = append(where, fmt.Sprintf("(gudang_asal_id = $%d OR gudang_tujuan_id = $%d)", idx, idx))
		args = append(args, *f.UserScopeGudangID)
		idx++
	}
	if f.Status != nil && strings.TrimSpace(*f.Status) != "" {
		where = append(where, fmt.Sprintf("status = $%d", idx))
		args = append(args, *f.Status)
		idx++
	}

	whereSQL := strings.Join(where, " AND ")

	var total int
	countSQL := "SELECT COUNT(*) FROM mutasi_gudang WHERE " + whereSQL
	if err := r.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count mutasi: %w", err)
	}

	offset := (f.Page - 1) * f.PerPage
	listSQL := fmt.Sprintf(
		`SELECT %s FROM mutasi_gudang WHERE %s
		 ORDER BY tanggal DESC, id DESC LIMIT $%d OFFSET $%d`,
		mutasiColumns, whereSQL, idx, idx+1,
	)
	args = append(args, f.PerPage, offset)

	rows, err := r.pool.Query(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query mutasi: %w", err)
	}
	defer rows.Close()

	out := make([]domain.MutasiGudang, 0, f.PerPage)
	for rows.Next() {
		var m domain.MutasiGudang
		if err := scanMutasi(rows, &m); err != nil {
			return nil, 0, fmt.Errorf("scan mutasi: %w", err)
		}
		out = append(out, m)
	}
	return out, total, rows.Err()
}

// UpdateStatus - update status + isi tanggal_kirim/tanggal_terima + user-nya.
// Trigger DB akan auto-update stok.
// Pakai WHERE current status untuk optimistic concurrency.
func (r *MutasiRepo) UpdateStatus(ctx context.Context, id int64, current, next domain.StatusMutasi, userID int64) error {
	switch next {
	case domain.StatusDikirim:
		const sql = `
			UPDATE mutasi_gudang
			SET status = $3, user_pengirim_id = $4, tanggal_kirim = now()
			WHERE id = $1 AND status = $2`
		tag, err := r.pool.Exec(ctx, sql, id, string(current), string(next), userID)
		if err != nil {
			return fmt.Errorf("update status dikirim: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrTransisiInvalid
		}
		return nil
	case domain.StatusDiterima:
		const sql = `
			UPDATE mutasi_gudang
			SET status = $3, user_penerima_id = $4, tanggal_terima = now()
			WHERE id = $1 AND status = $2`
		tag, err := r.pool.Exec(ctx, sql, id, string(current), string(next), userID)
		if err != nil {
			return fmt.Errorf("update status diterima: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrTransisiInvalid
		}
		return nil
	case domain.StatusDibatalkan:
		const sql = `
			UPDATE mutasi_gudang
			SET status = $3
			WHERE id = $1 AND status = $2`
		tag, err := r.pool.Exec(ctx, sql, id, string(current), string(next))
		if err != nil {
			return fmt.Errorf("update status dibatalkan: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrTransisiInvalid
		}
		return nil
	default:
		return domain.ErrTransisiInvalid
	}
}

// NextNomor generate nomor MUT/YYYY/MM/NNNN global per bulan.
func (r *MutasiRepo) NextNomor(ctx context.Context, tanggal time.Time) (string, error) {
	prefix := fmt.Sprintf("MUT/%04d/%02d/", tanggal.Year(), tanggal.Month())
	const sql = `
		SELECT COALESCE(MAX(NULLIF(regexp_replace(nomor_mutasi, '^MUT/\d{4}/\d{2}/', ''), '')::int), 0)
		FROM mutasi_gudang
		WHERE nomor_mutasi LIKE $1`
	var maxSeq int
	if err := r.pool.QueryRow(ctx, sql, prefix+"%").Scan(&maxSeq); err != nil {
		return "", fmt.Errorf("next nomor mutasi: %w", err)
	}
	return fmt.Sprintf("%s%04d", prefix, maxSeq+1), nil
}

// GetByClientUUID - cek idempotency. Return ErrMutasiNotFound bila belum ada.
func (r *MutasiRepo) GetByClientUUID(ctx context.Context, u uuid.UUID) (*domain.MutasiGudang, error) {
	const sql = `SELECT ` + mutasiColumns + ` FROM mutasi_gudang WHERE client_uuid = $1`
	row := r.pool.QueryRow(ctx, sql, u)
	var m domain.MutasiGudang
	if err := scanMutasi(row, &m); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrMutasiNotFound
		}
		return nil, fmt.Errorf("get mutasi by uuid: %w", err)
	}
	return &m, nil
}
