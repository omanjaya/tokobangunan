# Pembelian

## Apa Ini?

Menu **Pembelian** dipakai untuk catat barang masuk dari supplier. Setiap transaksi pembelian otomatis nambahin stok ke gudang yang dipilih, dan kalau status `Belum Lunas` masuk ke daftar hutang ke supplier.

Berbeda dari Stok Adjustment, pembelian punya nilai modal (HPP) yang dipakai buat hitung margin penjualan nanti.

## Kapan Pakai?

- Barang datang dari supplier (semen, besi, cat, dll)
- Restock rutin mingguan/bulanan
- PO khusus untuk proyek tertentu
- Beli barang konsinyasi (titip jual)

## Cara Pakai

### Langkah 1: Buka Form Pembelian

1. Klik menu `Pembelian` di sidebar
2. Klik `+ Pembelian Baru`
3. Form pembelian terbuka

`[screenshot: pembelian-form.png]`

### Langkah 2: Pilih Supplier

1. Klik dropdown `Supplier`
2. **Search nama supplier** atau scroll list
3. Kalau supplier baru:
   - Klik `+ Tambah Supplier`
   - Isi nama, kontak, alamat
   - Klik `Simpan` — auto-select balik ke form

### Langkah 3: Pilih Gudang Tujuan

1. Dropdown `Gudang` — pilih lokasi penyimpanan
2. Default: gudang utama
3. Buat barang yang langsung kirim ke proyek, pilih gudang `Proyek`

### Langkah 4: Tambah Item

Item table di tengah form:
1. Kolom `Produk` — search & pilih
2. Kolom `Qty` — jumlah masuk
3. Kolom `Satuan` — pilih (sak/kg/pcs/dus)
4. Kolom `Harga Modal` — harga beli per satuan
5. Kolom `Subtotal` — auto-calc

**Tambah row** klik `+ Item` atau `Tab` di row terakhir.

### Langkah 5: Status Pembayaran

Pilih di bawah form:
- `Lunas` — bayar cash di tempat
- `Belum Lunas` — masuk hutang ke supplier (tempo)
- `DP` — bayar sebagian, sisa hutang

Kalau `Lunas`, isi metode pembayaran (tunai/transfer/cek/giro).

### Langkah 6: Submit

1. Klik `Simpan` atau `F2`
2. Konfirmasi muncul
3. Setelah submit:
   - Stok auto-increment di gudang tujuan
   - HPP rata-rata produk auto-update (weighted average)
   - Hutang supplier bertambah (kalau belum lunas)

## Tips & Trick

- **F2** submit
- **Tab** pindah kolom item table cepat
- Copy harga modal lama dengan **klik kanan** di field harga
- Kalau import banyak item dari nota supplier, pakai **Import CSV** (icon upload)
- **Total auto-calc** di bawah, jangan input manual

## Common Mistake

- ❌ Lupa pilih gudang — default kemana stok kemana
- ✅ Selalu confirm gudang tujuan, terutama kalau toko punya >1 lokasi
- ❌ Input harga jual di kolom modal
- ✅ Kolom modal = harga beli dari supplier, bukan harga jual ke customer
- ❌ Submit tanpa cek total
- ✅ Cocokin total di sistem dgn nota fisik supplier

## Edge Case

- **Harga modal lebih tinggi dari pembelian sebelumnya** → System tetap accept, tapi HPP rata-rata ikut naik. Pertimbangkan naikin harga jual.
- **Supplier ngasih bonus barang** → Input qty barang asli + 1 row baru qty bonus harga modal Rp 0
- **Retur pembelian** → Pakai menu `Retur Pembelian` (bukan edit pembelian asli)
- **Supplier baru tanpa NPWP** → Boleh skip, isi nama doang cukup

## Cek Hutang Supplier

1. Menu `Mitra` → tab `Supplier`
2. Klik nama supplier
3. Lihat saldo hutang & list invoice belum lunas
4. Tombol `Bayar` untuk lunasin

## Related

- [Pembayaran](./03-pembayaran.md) — lunasin hutang ke supplier
- [Stok Adjustment](./04-stok-adjustment.md) — kalau stok masuk tapi bukan dari pembelian (sample/hadiah)
