# 07 — UI/UX Design System

Design system untuk aplikasi yang dipakai harian oleh kasir, gudang, admin, dan owner. Prioritas: cepat dipakai, sedikit klik, accessible.

## Prinsip Desain

1. **Density tinggi tapi terbaca** — kasir butuh lihat banyak baris transaksi sekaligus tanpa scroll berlebihan
2. **Keyboard-first** — kasir input cepat dengan keyboard; mouse hanya untuk navigasi sesekali
3. **Color-coded status** — piutang aging, stok level, status bayar punya warna konsisten yang accessible (kontras WCAG AA min)
4. **Mobile-friendly** — owner butuh monitor di HP saat di luar; responsive 320px ke atas
5. **No emoji** — pakai Lucide icon untuk semua visual cue
6. **Premium feel** — typography rapi, spacing presisi, shadow halus, transisi smooth

## Design Tokens

### Tipografi

| Penggunaan | Font | Weight | Size |
|------------|------|--------|------|
| Body default | Inter | 400 | 14px |
| Body small | Inter | 400 | 12px |
| UI label | Inter | 500 | 13px |
| Heading H1 | Inter | 600 | 28px |
| Heading H2 | Inter | 600 | 22px |
| Heading H3 | Inter | 600 | 18px |
| Heading H4 | Inter | 500 | 15px |
| Numeric (tabel, KPI) | JetBrains Mono | 400-600 | sesuai konteks |
| Code/SKU | JetBrains Mono | 400 | 13px |

Self-hosted di `/static/font/` untuk privacy + performance.

Line-height: 1.5 untuk body, 1.25 untuk heading.

Tabular nums (`font-variant-numeric: tabular-nums`) untuk angka di tabel agar align rapi.

### Spacing Scale

Base 4px:

`4, 8, 12, 16, 20, 24, 32, 40, 48, 64, 80, 96`

Pakai Tailwind class: `p-1` (4px), `p-2` (8px), dst.

### Color Palette

#### Neutral (Zinc scale)

| Token | Hex | Penggunaan |
|-------|-----|------------|
| `zinc-50` | `#fafafa` | Background page |
| `zinc-100` | `#f4f4f5` | Hover state subtle |
| `zinc-200` | `#e4e4e7` | Border light |
| `zinc-300` | `#d4d4d8` | Border default |
| `zinc-500` | `#71717a` | Text secondary |
| `zinc-700` | `#3f3f46` | Text body |
| `zinc-900` | `#18181b` | Text primary, heading |

#### Brand (TBD setelah branding owner)

Placeholder: **Indigo** scale. Akan diganti setelah owner finalize warna brand.

| Token | Hex | Penggunaan |
|-------|-----|------------|
| `brand-50` | `#eef2ff` | Background subtle |
| `brand-100` | `#e0e7ff` | Hover |
| `brand-500` | `#6366f1` | Primary action default |
| `brand-600` | `#4f46e5` | Primary action hover |
| `brand-700` | `#4338ca` | Primary action pressed |

#### Semantic

| State | Color | Usage |
|-------|-------|-------|
| Success | `emerald-500` `#10b981` | Lunas, stok normal, sukses |
| Warning | `amber-500` `#f59e0b` | Stok rendah, aging 30-60 |
| Danger | `rose-500` `#f43f5e` | Stok habis, overdue, error |
| Info | `sky-500` `#0ea5e9` | Notifikasi netral |
| Purple | `violet-500` `#8b5cf6` | Owner role |

### Border Radius

| Element | Radius |
|---------|--------|
| Button | 8px |
| Input | 8px |
| Card | 12px |
| Modal | 16px |
| Badge / pill | 999px (full) |

### Shadow

| Token | Use |
|-------|-----|
| `shadow-xs` | Card baseline |
| `shadow-sm` | Card hover, dropdown |
| `shadow-md` | Modal, drawer |
| `shadow-lg` | Toast |

### Icon

- Library: **Lucide**
- Ukuran: 16px (inline text), 20px (button), 24px (sidebar nav)
- Stroke width: 2 default
- Color: inherit dari text color, kecuali state khusus

## Layout Pattern

### App Shell

```
+----------+----------------------------------+
| SIDEBAR  | TOPBAR                           |
| (256px)  | breadcrumb     search   profile  |
|          +----------------------------------+
| Logo     |                                  |
|          | PAGE HEADER                      |
| Dashboard|   title       subtitle    [CTA]  |
| Penjualan|                                  |
| Stok     | CONTENT                          |
| Mutasi   |                                  |
| ...      |                                  |
|          |                                  |
| Profile  |                                  |
+----------+----------------------------------+
```

Sidebar: collapsible (icon-only saat collapsed). Width: 256px expanded, 64px collapsed.

### Dashboard Owner

Grid 12-kolom responsive:

```
+--------+--------+--------+--------+
| KPI 1  | KPI 2  | KPI 3  | KPI 4  |
| omset  | piutang| stok   | mitra  |
+--------+--------+--------+--------+
| GRAFIK PENJUALAN 30 HARI          |
| (line chart per gudang)           |
+-----------------+------------------+
| TOP MITRA       | STOK KRITIS     |
| (top 10 list)   | (warning items) |
+-----------------+------------------+
| TRANSAKSI TERAKHIR (mini table)   |
+-----------------------------------+
```

KPI card:
- Title + icon (top right corner)
- Value besar (32px, bold, JetBrains Mono)
- Delta indicator: arrow up/down + persen vs periode sebelumnya

### Form Penjualan (Halaman Kritis)

Layout 2 kolom:

```
+-----------------------------------+----------------+
| HEADER                            | RINGKASAN      |
|   tanggal   gudang  [F2: simpan]  |   Subtotal     |
+-----------------------------------+   Diskon       |
| MITRA                             |   Total        |
|   [autocomplete]                  |   Status bayar |
+-----------------------------------+                |
| ITEM                              |                |
| +-------------------------------+ |                |
| | produk | qty | sat | hrg | tot| |                |
| +-------------------------------+ |                |
| [+ Tambah Item: F3]               |                |
+-----------------------------------+                |
| CATATAN (opsional)                |                |
+-----------------------------------+----------------+
| [F4: Bayar Sekarang]   [F2: Kredit]  [F8: Cetak]   |
+----------------------------------------------------+
```

**Keyboard shortcut:**

| Key | Action |
|-----|--------|
| `F2` | Simpan & cetak |
| `F3` | Tambah item baru |
| `F4` | Set status: lunas (bayar tunai) |
| `F8` | Preview & cetak kwitansi |
| `F9` | Hapus item terpilih |
| `Esc` | Batal |
| `Tab` | Field berikutnya |
| `Enter` di combobox | Pilih + lanjut Tab |
| `Ctrl+S` | Simpan |
| `Ctrl+P` | Print preview |

### Tabel Data (DataTable)

```
+------------------------------------------------------+
| FILTER BAR                                           |
|   [search]  [tanggal range]  [gudang]  [status]      |
+------------------------------------------------------+
| HEADER (sticky)                                      |
|   No.  Tanggal▼  Mitra  Total       Status   Aksi    |
+------------------------------------------------------+
| ROWS (zebra striping)                                |
|   ...                                                 |
+------------------------------------------------------+
| FOOTER                                               |
|   Showing 1-25 of 1.250    [25 ▾]    [<] 1 2 3 [>]   |
+------------------------------------------------------+
```

## Pattern Detail

### Empty State

Saat tabel/list kosong:

```
        [icon: inbox 48px]
        
        Belum ada penjualan
        Mulai catat transaksi pertama Anda
        
        [+ Buat Penjualan Baru]
```

### Loading State

- **Initial load**: skeleton placeholder (animate-pulse)
- **Inline action**: spinner kecil di button + disable button
- **Background fetch**: progress bar di top of page (HTMX `hx-indicator`)

Tidak pakai full-page spinner — selalu progressive enhancement.

### Error State

- **Form validation**: inline di bawah field, warna rose
- **API error**: toast top-right, 5 detik auto-dismiss
- **Critical error**: modal block dengan retry button
- **Empty data after filter**: "Tidak ada data sesuai filter" + tombol clear filter

### Confirmation Destructive

Untuk delete data dengan transaksi terkait, modal dengan ketik nama:

```
Hapus mitra "PT Maju Jaya"?

Mitra ini punya 245 transaksi dan piutang Rp 12.500.000.
Penghapusan akan membuat soft-delete; data transaksi tetap ada.

Ketik nama mitra untuk konfirmasi:
[___________________________]

[Batal]              [Hapus] (disabled sampai cocok)
```

### Color-coded Status

#### Status Bayar

| Status | Badge Color | Background |
|--------|-------------|------------|
| Lunas | emerald-700 | emerald-50 |
| Sebagian | amber-700 | amber-50 |
| Kredit | sky-700 | sky-50 |

#### Aging Piutang

| Range | Badge | Severity |
|-------|-------|----------|
| Current (belum jatuh tempo) | emerald | Normal |
| 1-30 hari | amber light | Watch |
| 31-60 hari | amber | Warning |
| 61-90 hari | rose | Danger |
| 90+ hari | rose dark | Critical |

#### Stok Level

| Status | Badge |
|--------|-------|
| Habis (qty = 0) | rose dark |
| Kritis (qty < min) | amber |
| Normal | zinc default |
| Surplus (qty > 2x min) | sky |

## Wireframe Utama

Akan dibuat sebagai PNG/SVG di `docs/assets/`:

1. `assets/wf-01-login.svg` — Login page minimalis (logo + form + remember me)
2. `assets/wf-02-dashboard-owner.svg` — Dashboard dengan KPI + chart
3. `assets/wf-03-penjualan-form.svg` — Form input penjualan (2 kolom)
4. `assets/wf-04-mitra-list.svg` — List mitra + aging summary
5. `assets/wf-05-mitra-detail.svg` — Detail mitra: profile + tabs (transaksi/piutang/pembayaran/tabungan)
6. `assets/wf-06-stok-list.svg` — Tabel stok per cabang dengan filter
7. `assets/wf-07-mutasi-form.svg` — Form mutasi gudang (asal → tujuan + items)
8. `assets/wf-08-mutasi-list.svg` — List mutasi dengan status pipeline (draft → dikirim → diterima)
9. `assets/wf-09-laporan-lr.svg` — Laporan laba-rugi per cabang
10. `assets/wf-10-setting-printer.svg` — Setting koordinat dot matrix per template

## Responsive Breakpoint

| Name | Min Width | Use |
|------|-----------|-----|
| Mobile | 320px | Owner monitor |
| Tablet | 640px | Tablet kasir |
| Desktop sm | 768px | Layar kasir kecil |
| Desktop md | 1024px | Default desktop |
| Desktop lg | 1280px | Layar besar |

Sidebar otomatis collapse ke drawer di mobile (< 768px).
Form penjualan tetap 2 kolom di tablet, 1 kolom di mobile.

## Accessibility

- Semua interactive element punya focus ring (`ring-2 ring-brand-500 ring-offset-2`)
- Color contrast minimal **4.5:1** untuk text body, **3:1** untuk text large
- Form: label terhubung dengan input via `for`/`id`
- Error message terhubung via `aria-describedby`
- Tombol icon-only punya `aria-label`
- Navigasi keyboard penuh (Tab, Shift+Tab, Enter, Esc)
- Screen reader test dengan VoiceOver / NVDA

## Animation

Subtle & purposeful:

- Transition default: `150ms ease-out` untuk hover, focus
- Modal/drawer enter: `200ms ease-out`
- Toast slide-in: `250ms cubic-bezier(0.16, 1, 0.3, 1)`
- Skeleton: `1.5s ease-in-out infinite` shimmer

Hindari animasi gratuitous (bouncing, parallax, dll).

## Dark Mode

Tidak prioritas Fase 1. Akan ditambah Fase 6+ jika user request. Token color di-mirror sebagai CSS variable agar mudah switch.

## Komponen Spesifik (Linked)

Detail implementasi komponen reusable: lihat `05-shared-components.md`.
