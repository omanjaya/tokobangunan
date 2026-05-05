package domain

import (
	"errors"
	"time"
)

// Sentinel error tabungan mitra.
var (
	ErrTabunganInvalid     = errors.New("transaksi tabungan tidak valid")
	ErrTabunganSaldoKurang = errors.New("saldo tabungan tidak cukup")
)

// TabunganMitra - 1 baris ledger setor/tarik tabungan titip mitra.
// Salah satu antara Debit (setor) atau Kredit (tarik) > 0, tidak boleh dua-duanya.
type TabunganMitra struct {
	ID        int64
	MitraID   int64
	Tanggal   time.Time
	Debit     int64 // cents (setor)
	Kredit    int64 // cents (tarik)
	Saldo     int64 // running balance setelah baris ini
	Catatan   string
	UserID    int64
	CreatedAt time.Time
}

// Validate cek invariant baris tabungan.
func (t *TabunganMitra) Validate() error {
	if t.MitraID <= 0 {
		return ErrTabunganInvalid
	}
	if t.Debit < 0 || t.Kredit < 0 {
		return ErrTabunganInvalid
	}
	if t.Debit == 0 && t.Kredit == 0 {
		return ErrTabunganInvalid
	}
	if t.Debit > 0 && t.Kredit > 0 {
		return ErrTabunganInvalid
	}
	if t.Tanggal.IsZero() {
		return ErrTabunganInvalid
	}
	return nil
}
