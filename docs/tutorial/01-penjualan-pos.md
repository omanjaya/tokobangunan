# Penjualan / POS

## Apa Ini?

Menu **Penjualan** adalah jantung operasional toko. Semua transaksi jual ke pelanggan (mitra langganan maupun pembeli eceran walk-in) dimulai di sini. Mode POS dirancang biar kasir bisa input cepat tanpa banyak klik — tap-tap produk grid, scan barcode, atau ketik kode singkat.

Halaman ini juga handle multi-satuan (sak ⇄ kg, dus ⇄ pcs), split satuan pakai quick chip ¼ ½ 1, dan hitung otomatis konversi modal/jual sesuai satuan dasar.

## Kapan Pakai?

- Customer datang beli barang di toko (eceran walk-in)
- Mitra langganan order, dicatat ke akun mitra (bisa Lunas atau Kredit)
- Penjualan via WA → kasir input manual, lalu cetak faktur
- Setor barang ke proyek dengan invoice tempo

## Cara Pakai

### Langkah 1: Buka Menu POS

1. Klik menu `Penjualan` di sidebar
2. Tekan tombol `+ Transaksi Baru` atau shortcut `Alt+N`
3. Halaman POS terbuka dengan grid produk di kiri, keranjang di kanan

`[screenshot: pos-landing.png]`

### Langkah 2: Pilih Customer

**Opsi A — Mitra Langganan:**
1. Klik dropdown `Mitra` di kanan atas
2. Ketik nama mitra (search realtime)
3. Pilih dari list — saldo hutang & limit kredit langsung muncul

**Opsi B — Eceran Walk-in:**
1. Skip dropdown mitra (default sudah `Eceran`)
2. Lanjut langsung add item

### Langkah 3: Tambah Item

1. **Tap produk di grid** — auto qty 1
2. Atau **scan barcode** (cursor harus di field input atas)
3. Atau ketik kode singkat di search bar lalu `Enter`

**Quick chip ¼ ½ 1** muncul di setiap item:
- Tap `¼` → split jadi 0.25 sak (untuk semen 10kg dari sak 40kg, dll)
- Tap `½` → 0.5 satuan
- Tap `1` → reset full

**Toggle satuan** (sak ⇄ kg ⇄ pcs):
- Klik chip satuan di item row
- Preview konversi muncul di bawah: `1 sak = 40 kg → Rp 65.000/sak = Rp 1.625/kg`

### Langkah 4: Cek Stok

Tile stok di setiap item warna-coded:
- 🟢 **Emerald** — stok aman (>10)
- 🟡 **Amber** — stok menipis (≤10)
- 🌹 **Rose** — stok kritis / 0

Kalau rose, muncul warning. Bisa tetap submit (akan jadi minus), tapi konfirmasi dulu sama owner.

### Langkah 5: Submit

1. Pilih status pembayaran:
   - `Lunas` — langsung dibayar (cash/transfer/qris)
   - `Kredit` — masuk piutang mitra (mode tempo)
2. Tekan `F2` atau klik `Submit`
3. Modal konfirmasi muncul → klik `Ya, Simpan`
4. Auto-redirect ke halaman detail invoice

### Langkah 6: Cetak

Di halaman detail, ada 4 tombol:
- `Cetak 58mm` — printer struk thermal kecil
- `Cetak 80mm` — printer struk thermal besar
- `Cetak PDF` — download PDF A5
- `Cetak Faktur` — A4 untuk mitra/proyek

## Tips & Trick

- **F2** = Submit transaksi (paling sering dipakai)
- **F3** = Add item ke keranjang
- **ESC** = Cancel/clear keranjang
- **Alt+M** = Fokus ke field mitra
- **Alt+S** = Fokus ke search produk
- Ketik **2 huruf awal kode produk** + Enter → auto-add (kalau unique)
- Quick chip ¼ ½ 1 hemat 3-5 detik per item buat split satuan

## Common Mistake

- ❌ Lupa pilih satuan — semen 1 "sak" vs 1 "kg" beda 40x
- ✅ Selalu cek chip satuan aktif sebelum submit
- ❌ Tap produk 2x karena ngira belum kemasukan
- ✅ Lihat keranjang kanan — qty otomatis +1
- ❌ Pakai Lunas padahal mitra mau tempo
- ✅ Tanya dulu, default mitra langganan biasanya Kredit

## Edge Case

- **Stok 0 saat checkout** → Warning rose tile, masih bisa submit dengan konfirmasi (jadi stok minus, harus segera adjust)
- **Mitra exceeds limit kredit** → Banner merah muncul, owner harus approve override
- **Item harga 0** → Disabled submit, harus set harga jual dulu di master produk
- **Konversi pecahan aneh** (1.333 sak) → System auto round ke 0.01 terkecil

## Related

- [Pembayaran](./03-pembayaran.md) — kalau status Kredit, lunasin nanti dari sini
- [Edit/Cancel Kwitansi](./05-edit-cancel-kwitansi.md) — kalau salah input
- [Stok Adjustment](./04-stok-adjustment.md) — kalau stok minus perlu dikoreksi
