# Edit & Cancel Kwitansi

## Apa Ini?

Fitur **Edit** & **Cancel** dipakai untuk koreksi invoice penjualan yang salah input. Ada aturan ketat: **hanya bisa edit/cancel kalau invoice belum ada pembayaran**. Kalau sudah dibayar (sebagian/penuh), harus pakai **Retur** (parsial) atau hapus pembayaran dulu.

Cancel akan **rollback stok otomatis** (stok yang berkurang dari penjualan kembali ke gudang). Semua aksi tercatat di **audit trail** — siapa, kapan, alasan apa.

## Kapan Pakai?

**Edit:**
- Salah input qty
- Salah pilih satuan (kg vs sak)
- Salah pilih mitra
- Lupa add item, perlu nambah

**Cancel:**
- Customer batal beli setelah invoice dibuat
- Double-input (transaksi sama 2x)
- Salah create di mitra orang lain

## Aturan Edit vs Cancel

| Kondisi | Edit | Cancel |
|---------|------|--------|
| Belum ada pembayaran | ✅ | ✅ |
| Sudah DP (partial) | ❌ | ❌ |
| Sudah Lunas | ❌ | ❌ |
| > 7 hari | ⚠️ admin only | ⚠️ admin only |

**Solusi kalau sudah ada pembayaran:**
1. Hapus pembayaran dulu (di section Riwayat Pembayaran)
2. Baru bisa edit/cancel
3. Atau pakai **Retur** untuk parsial

## Cara Edit

### Langkah 1: Buka Invoice

1. Menu `Penjualan` → cari invoice (search no nota / mitra)
2. Klik baris → halaman detail

### Langkah 2: Klik Edit

1. Tombol `Edit` di kanan atas (kalau eligible)
2. Kalau disabled, hover untuk lihat alasan ("Sudah ada pembayaran")
3. Form edit muncul, mirip form create POS

### Langkah 3: Modifikasi

- Ubah qty/harga/satuan/mitra/status
- Add atau remove item
- Validasi stok ulang otomatis

### Langkah 4: Simpan

1. Klik `Simpan Perubahan`
2. Konfirmasi muncul + field `Alasan Edit` (wajib)
3. Submit
4. Stok auto-recalculate (selisih lama vs baru)
5. Audit log entry dibuat

`[screenshot: edit-invoice.png]`

## Cara Cancel

### Langkah 1: Buka Invoice

Sama seperti edit.

### Langkah 2: Klik Cancel

1. Tombol `Batalkan` (warna merah) di kanan atas
2. Modal konfirmasi muncul
3. Field `Alasan Pembatalan` wajib

### Langkah 3: Konfirmasi

1. Ketik alasan (min 10 karakter)
2. Klik `Ya, Batalkan`
3. Setelah submit:
   - Status invoice berubah jadi `DIBATALKAN`
   - **Banner merah** muncul di halaman detail: `INVOICE DIBATALKAN — [alasan] — oleh [user] @ [tanggal]`
   - Stok auto-rollback (kembali ke gudang)
   - Invoice tidak dihapus, tetap ada di list (strikethrough + badge merah)

### Langkah 4: Cetak (Optional)

Kwitansi dibatalkan tetap bisa dicetak tapi dengan watermark `BATAL` besar.

## Bulk Cancel

Untuk cancel banyak invoice sekaligus:

1. Menu `Penjualan` → tab `List`
2. URL: `/penjualan/list`
3. **Centang multiple invoice** (checkbox di kiri row)
4. Toolbar atas muncul: `X dipilih`
5. Klik tombol `Bulk Cancel`
6. Modal: input alasan (apply ke semua)
7. Konfirmasi

System akan skip invoice yang gak eligible (sudah ada bayar) + tampilkan list yang berhasil di-cancel.

## Tips & Trick

- **Cancel > Edit** kalau perubahan major. Cancel + create baru lebih clean.
- **Edit hanya untuk koreksi minor** (typo qty, salah satuan)
- Habit cek `Audit Trail` di bawah halaman detail kalau ada keluhan customer
- **Bulk cancel** hanya untuk skenario massal (sistem error, double-input batch) — biasakan cancel one-by-one untuk akuntabilitas

## Common Mistake

- ❌ Cancel padahal customer cuma minta tukar 1 item
- ✅ Pakai **Retur** untuk parsial, biar invoice asli tetap valid
- ❌ Edit invoice yang sudah dibayar dengan "ngakalin" (hapus bayar → edit → bayar ulang)
- ✅ Kalau sudah dibayar, biarin. Pakai retur atau adjustment.
- ❌ Alasan cancel asal-asalan ("salah" doang)
- ✅ Tulis konteks: "Customer Pak Budi batal ambil semen 5 sak via WA jam 10:00"

## Edge Case

- **Cancel invoice tapi stok udah keburu kepakai transaksi lain** → System tetap rollback, stok bisa jadi minus. Adjustment kategori `koreksi` untuk balance.
- **Edit qty dari 5 jadi 10 tapi stok cuma 7** → Validasi stok reject, harus restock dulu atau edit ke max 7
- **Cancel invoice yang udah dicetak ke faktur pajak** → Hati-hati! Koordinasi sama bagian pajak dulu, bikin nota retur resmi
- **Edit mitra dari Eceran ke Mitra Langganan** → Allowed, tapi limit kredit mitra divalidasi ulang

## Retur vs Cancel

| Skenario | Pakai |
|----------|-------|
| Customer batal total | **Cancel** |
| Customer ambil 8 dari 10, sisa balikin | **Retur** parsial |
| Customer udah bayar penuh, balikin semua | **Retur** full + refund |
| Salah input total | **Cancel** + create baru |

## Related

- [POS / Penjualan](./01-penjualan-pos.md) — kalau cancel + create baru
- [Pembayaran](./03-pembayaran.md) — hapus pembayaran dulu kalau perlu cancel invoice yang udah dibayar
- [Stok Adjustment](./04-stok-adjustment.md) — kalau cancel rollback bikin stok minus
- Retur tutorial (TBA)
