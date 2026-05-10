# Stok Adjustment

## Apa Ini?

Menu **Stok Adjustment** dipakai untuk koreksi stok manual di luar transaksi penjualan/pembelian. Setiap adjustment wajib punya **kategori** dan **alasan** — biar audit trail jelas dan ada akuntabilitas.

Akses **terbatas owner/admin** (kasir biasa tidak bisa adjust stok demi mencegah penyalahgunaan).

## Kapan Pakai?

- Hasil opname fisik beda dengan sistem (koreksi)
- Barang rusak/pecah selama penyimpanan
- Barang hilang/dicuri
- Kasih sample ke customer (gratis)
- Hadiah promo (giveaway)
- Retur ke supplier (barang cacat dikembalikan)
- Retur dari customer (barang dikembalikan)
- Input stok awal saat sistem baru di-setup

## 8 Kategori Adjustment

| Kategori | Qty | Use Case |
|----------|-----|----------|
| `initial` | + | Stok awal saat setup sistem pertama kali |
| `koreksi` | +/- | Hasil opname, beda hitung |
| `rusak` | - | Barang pecah/rusak di gudang |
| `hilang` | - | Stok hilang tanpa jejak |
| `sample` | - | Kasih sample ke customer |
| `hadiah` | - | Promo/giveaway |
| `return_supplier` | - | Retur ke supplier (barang cacat) |
| `return_customer` | + | Customer retur (kalau bukan via cancel invoice) |

## Cara Pakai

### Langkah 1: Buka Form Adjustment

1. Menu `Stok` → submenu `Adjustment`
2. Klik `+ Adjustment Baru`
3. Form terbuka

`[screenshot: stok-adjust-form.png]`

### Langkah 2: Pilih Gudang

1. Dropdown `Gudang` — pilih lokasi yang mau di-adjust
2. Stok current per produk akan muncul setelah pilih gudang

### Langkah 3: Pilih Kategori

1. Dropdown `Kategori` — pilih dari 8 di atas
2. Field `Alasan` muncul (wajib isi)
3. **Tulis alasan jelas:**
   - ❌ "koreksi"
   - ✅ "Hasil opname 4 Jan 2026, stok fisik 48 sak vs sistem 50 sak"

### Langkah 4: Tambah Item

1. Search produk → pilih
2. Stok saat ini muncul (read-only)
3. Input `Qty Adjustment`:
   - **Positif** (+5) untuk nambah
   - **Negatif** (-3) untuk kurang
4. Stok setelah adjustment auto-preview

**Untuk kategori tertentu, qty otomatis dibatasi:**
- `rusak`, `hilang`, `sample`, `hadiah`, `return_supplier` → hanya negatif
- `initial`, `return_customer` → hanya positif
- `koreksi` → bebas

### Langkah 5: Submit

1. Klik `Simpan`
2. Konfirmasi muncul
3. Stok update real-time
4. Entry masuk ke history (immutable, gak bisa edit)

### Langkah 6: Cek History

1. Menu `Stok` → `Adjustment` → tab `History`
2. URL: `/stok/adjust/history`
3. Filter by:
   - Tanggal
   - Kategori
   - Gudang
   - User yang record
   - Produk
4. Export CSV untuk laporan owner

## Tips & Trick

- **Alasan detail = aset audit** — tulis tanggal opname, no berita acara, dll
- Untuk opname masal (>20 produk), pakai **Stok Opname Session** (menu terpisah), bukan adjustment satu-satu
- **Mutasi antar gudang** ≠ adjustment. Pakai menu `Mutasi` (kurangi gudang A + tambah gudang B otomatis)
- Adjustment sample/hadiah dipisah dari rusak biar laporan biaya marketing jelas

## Adjustment vs Stok Opname vs Mutasi

| Fitur | Adjustment | Stok Opname | Mutasi |
|-------|-----------|-------------|--------|
| Scope | 1-beberapa produk | Full session, semua produk | Antar gudang |
| Use case | Koreksi targeted | Audit fisik berkala | Pindah lokasi |
| Wajib alasan | Ya | Ya (per session) | Optional |
| Akses | Owner/admin | Owner/admin | Kasir+ |

## Common Mistake

- ❌ Adjust stok rusak tapi kategori "koreksi"
- ✅ Pilih kategori `rusak` biar laporan biaya operasional akurat
- ❌ Alasan kosong/asal-asalan
- ✅ Sebut tanggal, sumber data (opname/lapor), no referensi
- ❌ Pakai adjustment buat mutasi antar gudang
- ✅ Pakai menu `Mutasi` — sekali submit, dua gudang ter-update

## Edge Case

- **Hasil adjust jadi stok minus** → System reject default. Owner bisa override dengan flag `Allow Negative` (jangan abuse)
- **Adjust produk yang lagi jadi item draft transaksi** → Lock sementara, tunggu transaksi commit/cancel dulu
- **Salah input qty adjustment** → Adjustment immutable, bikin adjustment baru dengan kategori `koreksi` dan qty kebalikannya
- **Adjust di tanggal mundur (back-date)** → Disabled default. Owner only, dengan reason wajib panjang.

## Related

- [Penjualan POS](./01-penjualan-pos.md) — kalau stok minus karena penjualan, koreksi via adjust kategori `koreksi`
- [Pembelian](./02-pembelian.md) — barang masuk normal, bukan adjust
- [Edit/Cancel Kwitansi](./05-edit-cancel-kwitansi.md) — cancel invoice auto-rollback stok, gak perlu adjust manual
