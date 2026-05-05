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

// ListPembayaranFilter filter list pembayaran customer (mitra).
type ListPembayaranFilter struct {
	From    *time.Time
	To      *time.Time
	Page    int
	PerPage int
}

// Normalize set default page/perpage.
func (f *ListPembayaranFilter) Normalize() {
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

// PembayaranRepo akses tabel pembayaran (customer).
type PembayaranRepo struct {
	pool *pgxpool.Pool
}

// NewPembayaranRepo konstruktor.
func NewPembayaranRepo(pool *pgxpool.Pool) *PembayaranRepo {
	return &PembayaranRepo{pool: pool}
}

const pembayaranColumns = `id, penjualan_id, penjualan_tanggal, mitra_id, tanggal,
	jumlah, metode, COALESCE(referensi, ''), user_id, COALESCE(catatan, ''),
	client_uuid, created_at`

func scanPembayaran(row pgx.Row, p *domain.Pembayaran) error {
	var metode string
	if err := row.Scan(&p.ID, &p.PenjualanID, &p.PenjualanTanggal, &p.MitraID,
		&p.Tanggal, &p.Jumlah, &metode, &p.Referensi, &p.UserID, &p.Catatan,
		&p.ClientUUID, &p.CreatedAt); err != nil {
		return err
	}
	p.Metode = domain.MetodeBayar(metode)
	return nil
}

// pembayaranQuerier abstraksi pool/tx untuk Create yang reusable.
type pembayaranQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Create insert satu pembayaran. Trigger DB akan recompute status_bayar penjualan.
func (r *PembayaranRepo) Create(ctx context.Context, p *domain.Pembayaran) error {
	return r.createOn(ctx, r.pool, p)
}

// CreateInTx insert satu pembayaran di dalam transaksi. Caller bertanggung
// jawab melakukan FOR UPDATE pada penjualan + sum existing pembayaran lebih
// dulu di tx yang sama supaya pencegahan overpayment race-condition aktif.
func (r *PembayaranRepo) CreateInTx(ctx context.Context, tx pgx.Tx, p *domain.Pembayaran) error {
	return r.createOn(ctx, tx, p)
}

func (r *PembayaranRepo) createOn(ctx context.Context, q pembayaranQuerier, p *domain.Pembayaran) error {
	const sql = `INSERT INTO pembayaran
		(penjualan_id, penjualan_tanggal, mitra_id, tanggal, jumlah, metode,
		 referensi, user_id, catatan, client_uuid)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING id, created_at`
	var ref *string
	if v := strings.TrimSpace(p.Referensi); v != "" {
		ref = &v
	}
	var catatan *string
	if v := strings.TrimSpace(p.Catatan); v != "" {
		catatan = &v
	}
	err := q.QueryRow(ctx, sql,
		p.PenjualanID, p.PenjualanTanggal, p.MitraID, p.Tanggal, p.Jumlah,
		string(p.Metode), ref, p.UserID, catatan, p.ClientUUID,
	).Scan(&p.ID, &p.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("client_uuid duplikat: %w", err)
		}
		return fmt.Errorf("create pembayaran: %w", err)
	}
	return nil
}

// GetByID lookup by id.
func (r *PembayaranRepo) GetByID(ctx context.Context, id int64) (*domain.Pembayaran, error) {
	const sql = `SELECT ` + pembayaranColumns + ` FROM pembayaran WHERE id = $1`
	var p domain.Pembayaran
	if err := scanPembayaran(r.pool.QueryRow(ctx, sql, id), &p); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrPembayaranNotFound
		}
		return nil, fmt.Errorf("get pembayaran: %w", err)
	}
	return &p, nil
}

// GetByClientUUID untuk idempotency check.
func (r *PembayaranRepo) GetByClientUUID(ctx context.Context, u uuid.UUID) (*domain.Pembayaran, error) {
	const sql = `SELECT ` + pembayaranColumns + ` FROM pembayaran WHERE client_uuid = $1`
	var p domain.Pembayaran
	if err := scanPembayaran(r.pool.QueryRow(ctx, sql, u), &p); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrPembayaranNotFound
		}
		return nil, fmt.Errorf("get pembayaran by uuid: %w", err)
	}
	return &p, nil
}

// ListByMitra paginasi pembayaran per mitra (filter tanggal optional).
func (r *PembayaranRepo) ListByMitra(ctx context.Context, mitraID int64, f ListPembayaranFilter) ([]domain.Pembayaran, int, error) {
	f.Normalize()
	conds := []string{"mitra_id = $1"}
	args := []any{mitraID}
	idx := 2
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
	where := strings.Join(conds, " AND ")

	var total int
	if err := r.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM pembayaran WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count pembayaran: %w", err)
	}

	offset := (f.Page - 1) * f.PerPage
	listSQL := fmt.Sprintf(`SELECT %s FROM pembayaran WHERE %s
		ORDER BY tanggal DESC, id DESC LIMIT $%d OFFSET $%d`,
		pembayaranColumns, where, idx, idx+1)
	args = append(args, f.PerPage, offset)

	rows, err := r.pool.Query(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list pembayaran: %w", err)
	}
	defer rows.Close()

	out := make([]domain.Pembayaran, 0, f.PerPage)
	for rows.Next() {
		var p domain.Pembayaran
		if err := scanPembayaran(rows, &p); err != nil {
			return nil, 0, fmt.Errorf("scan pembayaran: %w", err)
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}

// ListByPenjualan semua pembayaran utk satu penjualan.
func (r *PembayaranRepo) ListByPenjualan(ctx context.Context, penjualanID int64, penjualanTanggal time.Time) ([]domain.Pembayaran, error) {
	const sql = `SELECT ` + pembayaranColumns + ` FROM pembayaran
		WHERE penjualan_id = $1 AND penjualan_tanggal = $2
		ORDER BY tanggal ASC, id ASC`
	rows, err := r.pool.Query(ctx, sql, penjualanID, penjualanTanggal)
	if err != nil {
		return nil, fmt.Errorf("list pembayaran by penjualan: %w", err)
	}
	defer rows.Close()
	out := make([]domain.Pembayaran, 0, 4)
	for rows.Next() {
		var p domain.Pembayaran
		if err := scanPembayaran(rows, &p); err != nil {
			return nil, fmt.Errorf("scan pembayaran: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// SumByPenjualan total pembayaran utk satu penjualan (cents).
func (r *PembayaranRepo) SumByPenjualan(ctx context.Context, penjualanID int64, penjualanTanggal time.Time) (int64, error) {
	const sql = `SELECT COALESCE(SUM(jumlah), 0) FROM pembayaran
		WHERE penjualan_id = $1 AND penjualan_tanggal = $2`
	var v int64
	if err := r.pool.QueryRow(ctx, sql, penjualanID, penjualanTanggal).Scan(&v); err != nil {
		return 0, fmt.Errorf("sum pembayaran: %w", err)
	}
	return v, nil
}

// SumByMitra total pembayaran mitra sampai tanggal `until` (inclusive).
// Bila until.IsZero() → tanpa batas.
func (r *PembayaranRepo) SumByMitra(ctx context.Context, mitraID int64, until time.Time) (int64, error) {
	if until.IsZero() {
		const sql = `SELECT COALESCE(SUM(jumlah), 0) FROM pembayaran WHERE mitra_id = $1`
		var v int64
		if err := r.pool.QueryRow(ctx, sql, mitraID).Scan(&v); err != nil {
			return 0, fmt.Errorf("sum pembayaran mitra: %w", err)
		}
		return v, nil
	}
	const sql = `SELECT COALESCE(SUM(jumlah), 0) FROM pembayaran
		WHERE mitra_id = $1 AND tanggal <= $2`
	var v int64
	if err := r.pool.QueryRow(ctx, sql, mitraID, until).Scan(&v); err != nil {
		return 0, fmt.Errorf("sum pembayaran mitra: %w", err)
	}
	return v, nil
}
