# Pembayaran

## Apa Ini?

Menu **Pembayaran** dipakai untuk record pelunasan invoice — baik piutang dari mitra (mereka bayar ke kita) maupun hutang ke supplier (kita bayar ke mereka). Support **multi-metode** (split antara tunai + transfer, dll) dan **batch FIFO** (lunasin beberapa invoice sekaligus, otomatis dialokasi dari yang paling lama).

History pembayaran tampil sebagai chip list dengan breakdown per metode, jadi audit gampang.

## Kapan Pakai?

- Mitra datang bayar piutang (cash di kasir)
- Mitra transfer ke rekening, kasir konfirmasi & record
- Bayar nota ke supplier (cash/transfer/cek/giro)
- Customer DP duluan, lunasin sisa nanti
- Lunasin sekaligus 5 invoice mitra (batch FIFO)

## Cara Pakai

### Langkah 1: Buka Halaman Pembayaran

**Dari mitra detail (paling sering):**
1. Menu `Mitra` → klik nama mitra
2. Tab `Piutang` (atau `Hutang` kalau supplier)
3. Klik tombol `Bayar`

**Dari invoice spesifik:**
1. Buka detail penjualan/pembelian
2. Klik `Record Pembayaran`

`[screenshot: pembayaran-form.png]`

### Langkah 2: Single Metode (Default)

1. Field `Jumlah` — auto-fill sesuai sisa invoice (bisa di-edit)
2. Field `Metode` — pilih satu:
   - `Tunai`
   - `Transfer`
   - `QRIS`
   - `EDC` (kartu debit/kredit)
   - `Cek`
   - `Giro`
3. Field `Referensi` — nomor rekening/nomor cek/dll (optional tapi recommended)
4. Field `Tanggal` — default hari ini
5. Klik `Simpan`

### Langkah 3: Multi-Metode (Split)

Kalau customer bayar campur (misal: 500rb cash + 1jt transfer):

1. Toggle switch `Multi-Metode` ke ON
2. Form berubah jadi list rows
3. Row 1: `Tunai` `500.000`
4. Klik `+ Tambah Metode`
5. Row 2: `Transfer` `1.000.000` + referensi `BCA 1234567`
6. **Validasi sum** otomatis di bawah:
   - 🟢 hijau = match invoice
   - 🌹 merah = belum match (kurang/lebih)
7. Klik `Simpan` (disabled kalau sum belum match)

### Langkah 4: Batch FIFO (Multi-Invoice)

Untuk lunasin beberapa invoice sekaligus:

1. Di halaman mitra detail, **centang multiple invoice** (checkbox)
2. Klik `Bayar Terpilih`
3. Form muncul dengan total = sum invoice tercentang
4. Input jumlah bayar (boleh kurang dari total)
5. System auto-allocate **FIFO** (invoice paling lama dilunasin duluan)
6. Preview alokasi muncul:
   - Invoice #001 (3 bulan lalu): Rp 500.000 — LUNAS
   - Invoice #015 (1 bulan lalu): Rp 300.000 — LUNAS
   - Invoice #028 (minggu lalu): Rp 200.000 — DP (sisa Rp 800rb)
7. Klik `Konfirmasi`

### Langkah 5: Cek History

Di halaman invoice detail, scroll bawah ke section `Riwayat Pembayaran`:
- Chip list per pembayaran
- Breakdown metode per row
- Tanggal + user yang record
- Total terbayar vs sisa

## Tips & Trick

- **Auto-fill** field jumlah dari sisa invoice — jangan ubah kalau full payment
- **Referensi** wajib diisi untuk transfer (no rekening) & cek (no cek) — buat audit
- **Batch FIFO** hemat waktu kalau mitra bayar sekaligus puluhan nota
- Cek **chip warna** di history untuk identify metode cepat (tunai = hijau, transfer = biru, dll)

## Common Mistake

- ❌ Record cash padahal customer transfer
- ✅ Selalu konfirmasi metode + cek mutasi rekening sebelum record transfer
- ❌ Lupa input referensi transfer
- ✅ Habit: copy paste no rekening pengirim ke field referensi
- ❌ Multi-metode sum nggak match, force submit
- ✅ Validasi sum otomatis cegah ini, jangan bypass

## Edge Case

- **Customer bayar lebih (overpay)** → Sistem reject, harus split: bayar exact + sisa masuk deposit/saldo mitra
- **Cek/giro belum cair** → Tetap record, tapi flag `Pending Clear`. Setelah cair, ubah ke `Cleared`
- **Pembayaran salah, perlu reverse** → Klik invoice → Riwayat Pembayaran → tombol `Hapus` (audit log tetap)
- **Bayar pakai voucher/diskon** → Pakai metode `Voucher` + referensi nomor voucher

## Related

- [POS / Penjualan](./01-penjualan-pos.md) — sumber piutang dari penjualan kredit
- [Pembelian](./02-pembelian.md) — sumber hutang ke supplier
- [Edit/Cancel Kwitansi](./05-edit-cancel-kwitansi.md) — kalau invoice salah, harus cancel dulu sebelum edit
