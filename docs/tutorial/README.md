# Tutorial Toko Bangunan — Tier 1

Kumpulan tutorial untuk **5 menu paling sering dipakai harian** di sistem toko bangunan. Cocok untuk:

- 📚 **Training kasir baru** — onboarding terstruktur
- 📖 **Reference owner** — quick lookup saat lupa flow

## Daftar Tutorial

| # | Menu | File | Estimasi Baca |
|---|------|------|---------------|
| 1 | POS / Penjualan | [01-penjualan-pos.md](./01-penjualan-pos.md) | 8 menit |
| 2 | Pembelian | [02-pembelian.md](./02-pembelian.md) | 6 menit |
| 3 | Pembayaran | [03-pembayaran.md](./03-pembayaran.md) | 7 menit |
| 4 | Stok Adjustment | [04-stok-adjustment.md](./04-stok-adjustment.md) | 7 menit |
| 5 | Edit & Cancel Kwitansi | [05-edit-cancel-kwitansi.md](./05-edit-cancel-kwitansi.md) | 6 menit |

## Order Rekomendasi Training

Untuk kasir baru, ikuti urutan ini:

```
1. POS / Penjualan         ← core skill, paling sering dipakai
       ↓
2. Edit & Cancel Kwitansi  ← belajar handle salah input duluan
       ↓
3. Pembelian               ← barang masuk dari supplier
       ↓
4. Pembayaran              ← pelunasan piutang/hutang
       ↓
5. Stok Adjustment         ← advanced, owner/admin only
```

**Day 1-2:** Tutorial #1 + #2 (POS + handling error)
**Day 3-4:** Tutorial #3 (Pembelian)
**Day 5-7:** Tutorial #4 (Pembayaran multi-metode)
**Week 2+:** Tutorial #5 (untuk admin/owner)

## Cheat Sheet — 1 Halaman

### Shortcut Keyboard
| Key | Aksi |
|-----|------|
| `F2` | Submit transaksi |
| `F3` | Add item |
| `ESC` | Cancel/clear keranjang |
| `Alt+N` | Transaksi baru |
| `Alt+M` | Fokus ke mitra |
| `Alt+S` | Fokus ke search produk |
| `Tab` | Pindah field |

### Quick Decision Tree

**Customer datang beli barang?**
→ POS (#1)

**Barang datang dari supplier?**
→ Pembelian (#2)

**Customer/supplier bayar?**
→ Pembayaran (#3)

**Stok beda dengan fisik?**
→ Stok Adjustment (#4) — owner only

**Salah input invoice, belum dibayar?**
→ Edit/Cancel (#5)

**Salah input, sudah dibayar?**
→ Hapus pembayaran dulu, baru Edit/Cancel — atau pakai Retur

### Aturan Magic Numbers

- **Stok ≤10** → tile amber (warning)
- **Stok 0** → tile rose (kritis, konfirmasi sebelum jual)
- **Limit kredit mitra** → banner merah kalau over
- **Edit/Cancel** → hanya kalau invoice belum ada pembayaran
- **Adjustment alasan** → wajib min 10 karakter

### Quick Chip POS
- `¼` → 0.25 satuan (split semen, dll)
- `½` → 0.5 satuan
- `1` → reset full

### Status Pembayaran
- 🟢 `Lunas` — bayar penuh di tempat
- 🟡 `DP` — sebagian dibayar, sisa hutang
- 🔴 `Kredit` — tempo, masuk piutang/hutang

### Metode Pembayaran
- Tunai · Transfer · QRIS · EDC · Cek · Giro · Voucher

### Kategori Adjustment (8)
`initial` · `koreksi` · `rusak` · `hilang` · `sample` · `hadiah` · `return_supplier` · `return_customer`

## Catatan

- Semua screenshot (`[screenshot: ...]`) adalah placeholder — generate sendiri saat training session
- Tutorial fokus ke flow umum, bukan exhaustive. Cek `/docs/06-database-schema.md` untuk detail teknis.
- Update tutorial kalau ada perubahan UI mayor (sync ke versi sistem)

## Feedback

Kalau ada step yang membingungkan atau ada flow yang belum di-cover:
- Catat di logbook training
- Diskusi sama owner/admin
- Update tutorial bareng-bareng
