// Package audit provides helpers untuk capture state row sebelum mutasi (UPDATE/DELETE),
// sehingga audit log dapat menyimpan payload_before yang akurat.
//
// Pemakaian (di handler / service):
//
//	before, _ := audit.FetchBefore(ctx, pool, "produk", id)
//	// ... lakukan mutasi ...
//	// simpan `before` ke kolom payload_before pada tabel audit_log.
//
// FetchBefore memanfaatkan whitelist nama tabel untuk mencegah SQL injection.
package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// allowedTables memetakan logical name -> identifier SQL aman.
// Tambahkan entri di sini saat modul baru ingin memanfaatkan FetchBefore.
var allowedTables = map[string]string{
	"produk":        "produk",
	"mitra":         "mitra",
	"supplier":      "supplier",
	"gudang":        "gudang",
	"user":          `"user"`,
	"penjualan":     "penjualan",
	"harga_produk":  "harga_produk",
	"mutasi_gudang": "mutasi_gudang",
	"pembelian":     "pembelian",
}

// FetchBefore mengembalikan snapshot row sebagai JSON (row_to_json).
// Mengembalikan nil tanpa error jika row tidak ditemukan.
func FetchBefore(ctx context.Context, pool *pgxpool.Pool, table string, id int64) (json.RawMessage, error) {
	ident, ok := allowedTables[table]
	if !ok {
		return nil, fmt.Errorf("audit: tabel %q tidak diizinkan untuk FetchBefore", table)
	}
	q := fmt.Sprintf("SELECT row_to_json(t) FROM %s t WHERE id = $1", ident)
	rows, err := pool.Query(ctx, q, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	var raw json.RawMessage
	if err := rows.Scan(&raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// IsAllowed mengembalikan true jika tabel sudah masuk whitelist FetchBefore.
func IsAllowed(table string) bool {
	_, ok := allowedTables[table]
	return ok
}
