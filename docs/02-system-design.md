# 02 — System Design

## Diagram Konteks

```mermaid
graph TB
    Owner[Owner / Pemilik]
    Admin[Admin Pusat]
    Kasir[Kasir Cabang]
    Gudang[Staf Gudang]
    Mitra[Mitra / Customer]

    App[Tokobangunan Web App]

    DotMatrix[Printer Dot Matrix<br/>Epson LX-310]
    PDFPrinter[Printer A4/A5<br/>Inkjet/Laser]
    DB[(PostgreSQL 16)]
    Email[SMTP Server]
    WA[WhatsApp Business API<br/>opsional]

    Owner -->|Monitor multi-cabang| App
    Admin -->|CRUD master data| App
    Kasir -->|Input penjualan| App
    Gudang -->|Mutasi & opname| App
    Mitra -.->|Terima kwitansi/email| App

    App -->|ESC/P raw| DotMatrix
    App -->|PDF| PDFPrinter
    App <-->|Read/write| DB
    App -.->|Kirim PDF/notifikasi| Email
    App -.->|Kirim PDF/notifikasi| WA

    DotMatrix -->|Cetak rangkap 2| Mitra
    PDFPrinter -->|Cetak alternatif| Mitra
```

## Modul Aplikasi

| Modul | Tabel Utama | Fungsi |
|-------|-------------|--------|
| Auth & RBAC | `user`, `session`, `role` | Login, otorisasi per role |
| Master Data | `gudang`, `produk`, `satuan`, `mitra`, `supplier`, `harga_produk` | CRUD master, history harga |
| Penjualan | `penjualan`, `penjualan_item` | Input transaksi, cetak kwitansi |
| Pembayaran | `pembayaran` | Catat pembayaran mitra |
| Stok | `stok` | Stok real-time per cabang per produk |
| Mutasi Gudang | `mutasi_gudang`, `mutasi_item` | Transfer barang antar cabang |
| Stok Opname | `stok_opname`, `stok_opname_item` | Cek fisik vs sistem, adjustment |
| Hutang Supplier | `pembelian`, `pembelian_item`, `pembayaran_supplier` | Tracking hutang |
| Tabungan Mitra | `tabungan_mitra` | Opsional (Canggu, Sayan) |
| Laporan | View / aggregate | LR, penjualan, piutang aging, mutasi |
| Audit | `audit_log` | Track semua perubahan |

## Hybrid Online / Offline

### Strategi

- Default **online** — request langsung ke server, response real-time
- Saat offline — transaksi disimpan di **IndexedDB** browser dengan `client_uuid` (UUIDv7)
- Saat online kembali — service worker auto-sync queue ke server
- Server **idempotent** insert via `ON CONFLICT (client_uuid) DO NOTHING`

### Flow Offline

```mermaid
sequenceDiagram
    participant K as Kasir Browser
    participant SW as Service Worker
    participant API as Server API
    participant DB as PostgreSQL

    Note over K,API: Mode Offline
    K->>SW: POST penjualan (intercept)
    SW->>SW: Simpan ke IndexedDB queue
    SW-->>K: Response 202 Accepted (local)
    K->>K: Cetak kwitansi via dot matrix lokal

    Note over K,API: Internet Kembali
    SW->>API: Sync queue (POST batch)
    API->>DB: INSERT ON CONFLICT (client_uuid)
    API-->>SW: 200 OK + server IDs
    SW->>SW: Hapus queue yang sudah sync
```

### Yang Bisa Offline

| Operasi | Offline-able |
|---------|--------------|
| Login | Tidak (perlu sesi server) |
| Lihat master produk/mitra | Ya (cache) |
| Input penjualan | Ya |
| Cetak kwitansi (dot matrix lokal) | Ya |
| Lihat stok real-time multi-cabang | Tidak (data live dari server) |
| Mutasi gudang | Tidak (perlu approval real-time) |
| Pembayaran | Ya |
| Laporan | Tidak |

## Integrasi Print

### Dot Matrix (Default Produksi)

- Output: stream byte ESC/P raw
- Format: fixed-width text dengan koordinat presisi
- Kertas: NCR pre-printed 1/2 folio (vendor cetak kop & kolom; software hanya isi field)
- Driver Linux: `lp -d EpsonLX310 -o raw`
- Driver Windows (kasir): direct USB via JavaScript (Web USB API) atau spool file
- Konfigurasi koordinat per template di tabel `printer_template`

### PDF A5 (Fallback & Email)

- Library: `gofpdf`
- Layout: A5 portrait, kop toko di header, footer tanda tangan + tempat materai
- Watermark: "ASLI" untuk customer copy, "TEMBUSAN" untuk arsip
- Output: bytes → response HTTP `application/pdf` atau attach email

### Terbilang Indonesia

Library Go custom di `internal/terbilang/`:

```go
package terbilang

func Konversi(n int64) string
// 1_250_000 → "Satu juta dua ratus lima puluh ribu rupiah"
```

Edge case: nol, "se" untuk seribu/seratus, kapitalisasi awal kata.

## Data Flow Utama

### Flow 1: Penjualan ke Mitra

```mermaid
graph LR
    A[Kasir input form] --> B{Validasi stok & limit kredit}
    B -->|Pass| C[Insert penjualan + item]
    C --> D[Trigger update stok]
    C --> E[Insert ke audit_log]
    D --> F[Generate kwitansi]
    F --> G{Online?}
    G -->|Ya| H[Cetak dot matrix / PDF]
    G -->|Tidak| I[Queue offline + cetak lokal]
    B -->|Fail| J[Tampilkan error]
```

### Flow 2: Mutasi Antar Gudang

```mermaid
graph LR
    A[Staf gudang asal: form mutasi] --> B[Status: draft]
    B --> C[Submit: status dikirim]
    C --> D[Trigger: stok asal berkurang]
    D --> E[Notif gudang tujuan]
    E --> F[Staf gudang tujuan: konfirmasi terima]
    F --> G[Status: diterima]
    G --> H[Trigger: stok tujuan bertambah]
    H --> I[Audit log]
```

### Flow 3: Piutang & Pembayaran

```mermaid
graph LR
    A[Penjualan kredit] --> B[Saldo piutang naik]
    B --> C[Aging tracker: current/30/60/90+]
    C --> D[Dashboard owner: warning aging > 60]
    D --> E[Reminder ke mitra]
    F[Pembayaran masuk] --> G[Allocation ke invoice tertua FIFO]
    G --> H[Saldo piutang turun]
    H --> I{Saldo = 0?}
    I -->|Ya| J[Status invoice: lunas]
    I -->|Tidak| K[Status invoice: sebagian]
```

## Deployment Topology (Untuk Referensi, Diaktifkan di Fase 9)

```
                    INTERNET
                       |
                  Caddy (HTTPS)
                       |
          +------------v------------+
          |   Go App (Echo)         |
          |   :8080                 |
          +------------+------------+
                       |
          +------------v------------+
          |   PostgreSQL 16         |
          |   :5432                 |
          +-------------------------+

  Backup harian: pg_dump → object storage
```

Deployment detail di `08-security.md` section Backup & `01-plan.md` Fase 9.

## Skalabilitas

- **Vertikal dulu** — VPS Hostinger 4 vCPU / 8GB cukup untuk 5 cabang × 100 transaksi/hari
- **DB partitioning** — `penjualan` di-partition by tahun (RANGE) sejak awal
- **Indeks** — sudah dirancang untuk query 95th percentile
- **Read replica** — Fase 10+ jika perlu (laporan berat di replica, OLTP di master)

## Observability

- **Logging**: structured JSON via `slog`, level INFO default, DEBUG via env
- **Metrics**: Prometheus endpoint `/metrics` (Fase 9)
- **Health check**: `/healthz` (DB ping) untuk monitoring
- **Error tracking**: Sentry atau self-hosted GlitchTip (Fase 9)
