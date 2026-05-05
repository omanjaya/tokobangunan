# 01 — Plan Eksekusi & Roadmap

## Tujuan Project

Membangun web aplikasi manajemen toko bangunan untuk menggantikan sistem Excel eksisting yang sudah tidak skalabel.

### Masalah Sistem Excel Saat Ini

Berdasarkan analisis 7 file Excel (~46 MB total, ~250.000 baris transaksi):

1. **Performance degrade** — sheet `MAIN` di file Canggu mencapai 96.199 baris; pivot table dan formula mulai lambat
2. **Cross-sheet reference rapuh** — formula seperti `=Canggu!E28` patah ketika user insert/delete row
3. **Konversi satuan hardcoded** — angka `/5.5` dan `/4` ditulis langsung di formula, susah diubah konsisten
4. **Tidak ada master data** — nama produk dan mitra ditulis sebagai string bebas, banyak duplikasi (case-sensitive)
5. **Manual reconciliation antar gudang** — saldo dihitung manual via `=H2+K2+N2-B3-B4-B5`, error-prone
6. **Tidak ada audit trail** — siapa edit kapan tidak tercatat
7. **Versi ganda** — file `SAYAN` dan `SAYAN (1)` ambigu, tidak jelas mana yang current
8. **Tidak ada multi-user** — file shared, semua orang bisa edit semua sel
9. **Tidak ada backup terstruktur** — manual copy-paste antar bulan ke `Rekapan`

### Tujuan Spesifik Aplikasi

1. Replikasi seluruh workflow Excel ke web app dengan database relasional
2. Output kwitansi rangkap 2 (putih asli + pink tembusan) via dot matrix Epson LX-310 + kertas NCR carbonless
3. Fallback PDF A5 untuk email/WhatsApp ke mitra
4. Hybrid online (default) + offline-fallback per cabang
5. Multi-cabang (5 gudang) dengan stok real-time terkonsolidasi
6. UI/UX kualitas tinggi — keyboard-first, density tinggi, premium icon
7. Migrasi 250.000 transaksi historis dari Excel
8. Audit log lengkap untuk compliance pajak

## Stack Teknologi

| Layer | Pilihan | Alasan |
|-------|---------|--------|
| Bahasa | Go 1.22+ | Binary tunggal, performa tinggi, deploy mudah |
| Web framework | Echo v4 | Ringan, middleware lengkap |
| Template | Templ | Type-safe, compile-time check |
| Frontend | HTMX + Alpine.js | Server-rendered, no SPA complexity |
| CSS | Tailwind CSS (standalone build) | Cepat, konsisten |
| Icon | Lucide (SVG inline) | Premium, konsisten, lightweight |
| Font | Inter + JetBrains Mono | Profesional, terbaca |
| DB | PostgreSQL 16 | Window function, partition, JSONB |
| Query | sqlc | Type-safe SQL, zero-cost abstraction |
| Migration | golang-migrate | Standar industri Go |
| Auth | Session + Argon2id | Aman, modern |
| Excel I/O | excelize v2 | Mature, support semua fitur |
| PDF | gofpdf | Reliable untuk layout fixed |
| Dot matrix | ESC/P raw via CUPS | Standar Epson LX-310 |
| Dev | Docker Compose + Air | Hot reload, isolated env |

Lihat `09-dev-environment.md` untuk konfigurasi dev lengkap.

## Roadmap Fase

| Fase | Durasi | Output Utama |
|------|--------|--------------|
| 0. Setup folder + docs + dev env | 1-2 hari | Repo siap dengan dokumentasi & Docker |
| 1. Master data + auth | 2 minggu | Login, RBAC, CRUD produk/mitra/gudang/satuan/user, seed |
| 2. Penjualan + kwitansi rangkap | 3 minggu | Form penjualan, dot matrix + PDF, terbilang Indonesia |
| 3. Stok + mutasi antar gudang | 2 minggu | Stok per cabang, transfer auto-saldo |
| 4. Piutang + pembayaran | 2 minggu | Tracking piutang, aging report 30/60/90+, reminder |
| 5. Stok opname + hutang supplier | 1 minggu | Modul pelengkap |
| 6. Laporan & dashboard | 2 minggu | LR per cabang, laporan penjualan/mutasi/piutang |
| 7. Migrasi data Excel | 1 minggu | Importer + verifikasi totals |
| 8. Offline mode + UAT lokal | 2 minggu | Service worker, paralel run lokal, training |
| 9. Deployment ke VPS | 1 minggu | Tertunda — setelah UAT lokal selesai |
| **Total dev** | **~15 minggu** | Sistem siap UAT lokal |

## Strategi Migrasi Data Excel

Script Go satu kali (`cmd/migrate-excel/main.go`):

### Langkah Migrasi

1. **Audit `SAYAN` vs `SAYAN (1)`** — bandingkan checksum sheet `DETAIL`, lapor ke owner mana yang current
2. **Build master produk** — kumpulkan distinct nama produk lintas semua sheet `DETAIL`, lakukan cleanup duplikat (case-insensitive, fuzzy match), import ke tabel `produk`
3. **Build master mitra** — kumpulkan distinct nama mitra dari sheet `DETAIL` & `PIUTANG`, import ke tabel `mitra`
4. **Mapping satuan & faktor konversi** — klarifikasi ke owner: konversi `/5.5` itu produk apa? `/4` itu apa? Set `produk.faktor_konversi` per produk
5. **Import transaksi** — sheet `DETAIL` per cabang ke `penjualan` + `penjualan_item`
6. **Import mutasi** — file `Antar Gudang 2025.xlsx` ke `mutasi_gudang` + `mutasi_item`
7. **Import piutang awal** — sheet `PIUTANG` ke saldo opening per mitra
8. **Verifikasi** — total nilai per cabang per bulan di app harus match Excel persis (toleransi 0)

### Mode Eksekusi

- Migrasi dijalankan ke staging DB dulu
- Owner approve hasil verifikasi
- Promote ke prod DB

## Verifikasi End-to-End

### Test Wajib Sebelum UAT

1. **Unit test** — terbilang, hitung saldo piutang, konversi satuan
2. **Integration test** — flow penjualan → stok berkurang → piutang naik → pembayaran masuk → piutang turun
3. **Migrasi test** — total Excel == total DB untuk Februari 2025 (sample)
4. **Print test fisik** — cetak via Epson LX-310 dengan kertas NCR pre-printed, verifikasi alignment
5. **Print test PDF** — cetak A5, verifikasi terbilang, layout, kop
6. **Concurrency test** — 5 kasir input bersamaan, stok tidak race condition (`SELECT ... FOR UPDATE`)
7. **Offline test** — matikan internet, transaksi tetap masuk, sync otomatis saat connect
8. **UAT** — owner + 1 kasir per cabang pakai 1 minggu paralel dengan Excel

## Pertanyaan Terbuka

1. Konversi `/5.5` & `/4` — produk dan satuan apa persisnya
2. `SAYAN` vs `SAYAN (1)` — file mana current
3. Sheet `Tabungan` (Canggu, Sayan) — fungsi spesifik
4. Branding: nama aplikasi final, logo, warna primer, kop kwitansi
5. Apakah perlu mobile native (Android) untuk owner monitoring atau cukup responsive web

Item 1-3 dijawab saat audit migrasi. Item 4 ditanyakan ke owner sebelum mulai Fase 1 (untuk lock primary color & kop kwitansi). Item 5 menunggu input owner di Fase 8.

## Risiko & Mitigasi

| Risiko | Dampak | Mitigasi |
|--------|--------|----------|
| Owner ubah scope di tengah jalan | Delay 2-4 minggu | Sign-off plan setiap akhir fase, change request formal |
| Konversi satuan ambigu di data Excel | Migrasi data salah | Workshop dengan owner, sample 10 produk dulu |
| Printer dot matrix tidak tersedia di cabang | Output kwitansi tidak optimal | Mode PDF A5 sebagai fallback, beli printer bertahap |
| Internet putus di cabang | Transaksi gagal | Offline mode (Fase 8), service worker + IndexedDB |
| Resistance dari kasir lama | Adopsi lambat | Training, paralel run 1 bulan, UI sederhana |
