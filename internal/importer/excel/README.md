# Excel Importer (Fase 7)

One-shot tool untuk migrasi data dari sistem Excel lama (Mitra Usaha + Antar
Gudang) ke skema PostgreSQL Toko Bangunan.

Lokasi binary: `cmd/migrate-excel`.

## Cara Pakai

Tool ini dirun **native di host** (bukan di dalam container) supaya bisa
akses folder Excel di laptop tanpa edit `docker-compose.yml`.

### 1. Build binary

```bash
go build -o ./bin/migrate-excel ./cmd/migrate-excel
```

### 2. Mode Audit (NO DB ACCESS)

Analisis isi file Excel saja, tidak menyentuh database.

```bash
./bin/migrate-excel \
    --source "/Users/omanjaya/Downloads/PROJECT UNTUK TOKOBANGUNAN" \
    --mode audit
```

Output:
- Daftar file + ukuran + sheet + row count
- Bandingkan SAYAN.xlsx vs SAYAN(1).xlsx (hash + row count + rekomendasi)
- Top 10 produk distinct (berdasarkan occurrence di MAIN)
- Top 10 mitra distinct
- Faktor konversi yang terdeteksi (5.5, 4)
- Anomali parse

### 3. Mode Dry Run

Validasi penuh (parse semua sheet + insert ke DB) tapi rollback transaksi.

```bash
export DATABASE_URL="postgres://dev:dev@localhost:5432/tokobangunan?sslmode=disable"
./bin/migrate-excel \
    --source "/Users/omanjaya/Downloads/PROJECT UNTUK TOKOBANGUNAN" \
    --mode dry-run \
    --confirm-sayan SAYAN
```

### 4. Mode Import (REAL)

```bash
export DATABASE_URL="postgres://dev:dev@localhost:5432/tokobangunan?sslmode=disable"
./bin/migrate-excel \
    --source "/Users/omanjaya/Downloads/PROJECT UNTUK TOKOBANGUNAN" \
    --mode import \
    --confirm-sayan SAYAN \
    --year 2025 \
    --batch-size 1000 \
    --opening-date 2025-01-01 \
    --log-file migrate-$(date +%s).log
```

## Asumsi & Pemetaan Data

### Sheet MAIN (master transaction log)

Source of truth untuk seluruh penjualan; ~96K rows untuk Canggu, ~94K untuk
Sayan, dst. Layout kolom (per inspeksi langsung):

| Idx | Kolom        | Pemakaian                                |
| --- | ------------ | ---------------------------------------- |
| A   | Tanggal      | `penjualan.tanggal`                      |
| B   | Bulan/Tahun  | (dipakai oleh sheet pivot, di-skip)     |
| C   | ITEM         | `penjualan_item.produk_nama`             |
| D   | IN           | qty masuk (pembelian) -> di-skip         |
| E   | OUT          | qty keluar (penjualan) -> `qty`          |
| F   | HPP          | (di-skip; tidak ada kolom skema)         |
| G   | HJ           | `penjualan_item.harga_satuan`            |
| H   | Sisa Stock   | (di-skip; stok dihitung di app)          |
| I   | L/R          | (di-skip)                                |
| J   | Penjualan    | `penjualan.total` (Rp)                   |
| K   | Stat         | BON/CASH -> derive `status_bayar`        |
| L   | Nama         | mitra (lookup ke `mitra.id`)             |
| M   | Bon          | (di-skip)                                |
| N   | Nominal      | (di-skip)                                |
| O   | STATUS       | LUNAS/BON -> override `status_bayar`     |

Hanya baris dengan OUT > 0 yang di-import sebagai penjualan.

### Sheet PIUTANG

Layout: A=NAMA, B=UTANG (saldo). Diimport sebagai phantom penjualan kredit
dengan tanggal `--opening-date` dan `catatan = "Saldo piutang awal (migrasi)"`.

### Sheet Pembayaran

Layout: A=Tanggal, B=Nama, C=Piutang Awal, D=Pembayaran, E=Keterangan.
D digunakan sebagai jumlah pembayaran. Di-import ke `pembayaran` tanpa
link ke penjualan tertentu (`penjualan_id` NULL).

### Sheet Stok Gudang

Layout: A=ITEM, E=Stok Akhir. E dipakai sebagai qty stok awal.

### Sheet Tabungan

Format wide-pivot (kolom = bulan, baris = hari). **Tidak diimport otomatis.**
Reshape manual ke format normalized (tanggal, mitra, debit/kredit) lalu
import via tool lain atau handler pembayaran.

### Sheet Hutang

Format pivot per supplier. **Tidak diimport otomatis** (perlu user pilih
supplier mapping). Skema `pembelian` masih bisa di-extend nanti.

### Antar Gudang 2025.xlsx

Tiap sheet (Canggu, Sayan, dll) berisi multiple block "Tgl|Asal-Tujuan|
Qty|Harga|Total". Parser auto-detect block dari header row 3 dan
generate `mutasi_gudang` + `mutasi_item` dengan status `diterima`.

## Idempotency

Setiap insert pakai `client_uuid` deterministic dari hash
`(file, sheet, row_idx)` — re-run tidak akan duplicate.

## User `migrate`

Tool akan auto-create user khusus `username=migrate, role=admin,
is_active=FALSE` saat first run. User ini di-set sebagai `created_by`
(via `user_id`) untuk semua row hasil migrasi, jadi audit trail tetap
ada walaupun tidak ada user real.

## Pertanyaan yang Perlu User Jawab

1. **Faktor konversi**: rumus `/5.5` & `/4` muncul di sheet Antar Gudang.
   Apakah memang per produk berbeda (semen 50kg vs batang 5.5m vs lainnya)?
   Untuk sekarang `produk.faktor_konversi = 1` default semua produk.
2. **Tipe mitra default**: semua diimport sebagai `tipe='eceran'`,
   `limit_kredit=0`. Perlu re-classify grosir/proyek?
3. **Satuan default produk**: `sak` jadi default kalau tidak ditemukan
   di kolom satuan MAIN. Apakah benar?
4. **SAYAN vs SAYAN(1)**: hash file identik (audit confirm), pilih
   `--confirm-sayan SAYAN` saja.
5. **Tabungan & Hutang**: format pivot — perlu reshape manual sebelum
   import, atau dibuatkan tool terpisah.

## Verifikasi

Setelah `import`, tool print total `penjualan` per gudang per bulan dari
DB. Bandingkan manual dengan Excel untuk audit tambahan.
