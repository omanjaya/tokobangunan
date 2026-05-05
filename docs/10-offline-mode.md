# Offline Mode - Tokobangunan

Dokumen ini menjelaskan arsitektur, cara kerja, dan operasional fitur offline-first
untuk aplikasi Tokobangunan multi-cabang.

## Latar Belakang

Beberapa cabang memiliki koneksi internet yang sering putus. Kasir tetap harus
bisa mencatat penjualan dan mencetak kwitansi (printer dot matrix lokal) tanpa
internet. Ketika koneksi pulih, transaksi yang dibuat saat offline harus
otomatis tersinkronisasi ke server tanpa duplikasi.

## Komponen

| File | Peran |
|------|-------|
| `web/static/js/sw.js` | Service worker: cache asset + intercept POST `/penjualan` saat offline |
| `web/static/js/sync.js` | IndexedDB queue + sync engine (auto-flush online) |
| `web/static/js/uuid.js` | Generator UUIDv7 client-side (untuk `client_uuid`) |
| `web/static/js/penjualan-offline.js` | Inject `client_uuid`, autosave draft, intercept submit offline |
| `web/static/js/sw-register.js` | Register service worker pada `load` |
| `web/static/manifest.webmanifest` | PWA manifest (install ke home screen / desktop) |
| `internal/view/components/online_status.templ` | Indikator online/offline + jumlah antrian |

## Strategy Cache (Service Worker)

- **Cache-first** untuk URL yang match `/static/*` atau ekstensi static
  (`.css`, `.js`, font, ikon).
- **Network-first** untuk navigasi dan request lain. Saat gagal network, fallback
  ke cache; untuk navigasi, fallback terakhir adalah `/dashboard`.
- **Khusus `POST /penjualan`** saat offline:
  1. Body request dibaca + disimpan ke IndexedDB store `pending`.
  2. SW return synthetic `202 Accepted` dengan body JSON `{ ok, queued, client_uuid }`.
  3. Background Sync API (jika tersedia) di-register dengan tag `sync-queue`.

Cache versioned: `tokobangunan-v1`. Naikkan versi (`v2`, dst) saat ingin
invalidate semua asset.

## Idempotency (`client_uuid`)

Tabel `penjualan` memiliki kolom `client_uuid UUID NOT NULL` dengan unique index
`uq_penjualan_clientuuid (client_uuid, tanggal)` (lihat migration
`0012_penjualan.up.sql`). Setiap form penjualan di-inject hidden input
`client_uuid` ber-UUIDv7 saat mount.

- Online normal: form post dengan `client_uuid` baru; server insert.
- Offline: SW queue request dengan `client_uuid`; saat replay, server menolak
  duplikat dengan `409 Conflict` atau unique violation; sync engine memperlakukan
  `409` sebagai sukses (sudah ada di server) dan menghapus dari queue.

UUIDv7 dipilih karena urut waktu (k-sortable) sehingga menjaga lokalitas index
B-tree di Postgres, lebih efisien dibanding UUIDv4.

## Skema IndexedDB

- DB: `tokobangunan-sync`, version `1`
- Object store: `pending`, `keyPath: "id"`, `autoIncrement: true`
- Record shape:

```json
{
  "id": 12,
  "url": "https://host/penjualan",
  "method": "POST",
  "headers": { "Content-Type": "application/x-www-form-urlencoded" },
  "body": "nomor_kwitansi=...&client_uuid=...",
  "contentType": "application/x-www-form-urlencoded; charset=UTF-8",
  "client_uuid": "01940b3e-...-7...-8...-...",
  "created_at": 1735689600000
}
```

## Sync Engine

Trigger flush:
1. Event `online` window.
2. Polling tiap 30 detik saat tab visible & navigator.onLine.
3. Pesan `SYNC_FLUSH_REQUEST` dari service worker (Background Sync).
4. Klik manual pada badge online (memanggil `manualSync()`).

Per item, response handling:
- `2xx` → hapus dari queue.
- `409` → hapus dari queue (duplikat = sudah pernah masuk).
- `4xx` lain → drop dari queue (permanen gagal, log toast error).
- `5xx` → keep, lanjut item berikutnya.
- network error → break loop (offline lagi).

Header `X-Offline-Replay: 1` di-set saat replay; server boleh memakai header ini
untuk audit.

## Yang Bisa & Tidak Bisa Offline

| Fitur | Offline? | Catatan |
|-------|----------|---------|
| Penjualan baru (cetak kwitansi) | Ya | Print dot matrix lokal jalan; queue ke server |
| Restore draft form | Ya | localStorage `tb:penjualan:draft` |
| List penjualan terbaru | Tidak (hanya cache halaman terakhir) | Network-first, fallback cache |
| Mutasi stok antar gudang | Tidak | Butuh approval real-time |
| Pembayaran piutang | Tidak | Validasi saldo butuh server |
| Laporan / dashboard real-time | Tidak | Data harus fresh |
| Master data (produk, mitra) | Read-only via cache | Edit butuh online |

## Install PWA

1. Buka aplikasi di Chrome / Edge / Brave.
2. Klik ikon "Install" di address bar (atau menu → "Install Tokobangunan").
3. Aplikasi terbuka standalone, bisa pin ke taskbar.

## Operasional & Troubleshooting

### Cek status service worker
DevTools → Application → Service Workers. Pastikan status `activated and is running`,
scope `/`.

### Simulasi offline
DevTools → Network → throttling dropdown → Offline. Coba submit form penjualan,
verifikasi toast "Tersimpan offline" muncul. Lalu Network → Online → tunggu 30
detik atau klik badge "antri" untuk sync manual.

### Inspect queue
DevTools → Application → IndexedDB → `tokobangunan-sync` → `pending`.

### Manual flush dari console
```js
window.tbSync.size().then(console.log);
window.tbSync.flush().then(console.log);
window.tbSync.list().then(console.log);
```

### Clear queue (hapus residual)
```js
const req = indexedDB.deleteDatabase("tokobangunan-sync");
req.onsuccess = () => console.log("queue cleared");
```

### Force update service worker
Naikkan `CACHE_VERSION` di `sw.js` (mis. `tokobangunan-v2`) lalu deploy. Browser
akan men-download SW baru pada navigasi berikutnya; reload sekali untuk activate.
Atau dari DevTools → Application → Service Workers → "Update" / "Unregister".

### Debug Background Sync
Chrome DevTools → Application → Background Services → Background Sync. Catatan:
Background Sync tidak didukung Firefox / Safari; sync engine fallback polling +
event `online`.

## Deploy Notes

Service worker `/sw.js` HARUS diserve dari root path (bukan `/static/js/sw.js`)
agar scope cover seluruh domain. Manifest juga lebih bersih dari root.

Tambahkan dua baris ini di `cmd/server/main.go` setelah static handler
ter-register:

```go
e.File("/sw.js", "web/static/js/sw.js")
e.File("/manifest.webmanifest", "web/static/manifest.webmanifest")
```

Catatan header (opsional namun direkomendasikan): `Service-Worker-Allowed: /`
dan `Cache-Control: no-cache` untuk `sw.js` agar update cepat ter-pickup.
