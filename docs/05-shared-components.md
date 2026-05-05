# 05 — Shared UI Components

Library komponen reusable di `internal/view/components/`. Setiap komponen ditulis sebagai Templ component. Goal: konsistensi visual + reusable cepat di seluruh modul.

## Konvensi Komponen

- File: `<component_name>.templ` (snake_case)
- Function: `PascalCase` (exported)
- Props: struct `<ComponentName>Props`
- Variant: pakai parameter, bukan multiple component
- State: handled by HTMX (loading via `hx-indicator`, error via swap target)
- Icon: dari Lucide, render inline SVG via `internal/view/icon`

## Layout Components

### `AppShell`

Wrapper utama setiap halaman. Berisi sidebar, topbar, content area.

```templ
templ AppShell(props AppShellProps) {
    <html lang="id">
        <head>
            <title>{ props.Title } - Tokobangunan</title>
            <link rel="stylesheet" href="/static/css/app.css"/>
            <script src="/static/js/htmx.min.js" defer></script>
            <script src="/static/js/alpine.min.js" defer></script>
        </head>
        <body class="bg-zinc-50 text-zinc-900">
            @Sidebar(props.Nav)
            <main class="ml-64">
                @TopBar(props.User, props.Breadcrumb)
                <div class="p-6">
                    { children... }
                </div>
            </main>
            @ToastContainer()
        </body>
    </html>
}
```

### `Sidebar`

Navigasi vertikal kiri. Collapsible. Highlight active route.

Item: Dashboard, Penjualan, Stok, Mutasi, Mitra, Supplier, Laporan, Setting.

### `TopBar`

Bar atas: breadcrumb di kiri, search global di tengah, profil user + logout di kanan.

### `Breadcrumb`

Path navigasi: `Dashboard > Penjualan > #INV-2025-0001`. Last item bold, tidak clickable.

### `PageHeader`

Header tiap page: judul + subtitle + action button (di kanan).

```templ
templ PageHeader(title, subtitle string) {
    <div class="flex items-start justify-between mb-6">
        <div>
            <h1 class="text-2xl font-semibold tracking-tight">{ title }</h1>
            if subtitle != "" {
                <p class="text-sm text-zinc-500 mt-1">{ subtitle }</p>
            }
        </div>
        <div class="flex gap-2">
            { children... }
        </div>
    </div>
}
```

## Form Components

### `FormField`

Wrapper field: label + input + helper + error.

```templ
templ FormField(label string, required bool, error string) {
    <div class="space-y-1.5">
        <label class="text-sm font-medium text-zinc-700">
            { label }
            if required {
                <span class="text-rose-500 ml-0.5">*</span>
            }
        </label>
        { children... }
        if error != "" {
            <p class="text-xs text-rose-600 mt-1">{ error }</p>
        }
    </div>
}
```

### `Input`

Text input. Variant: text, email, password, tel, number.

Props: name, value, placeholder, disabled, autocomplete, prefix (e.g. "Rp"), suffix (e.g. "kg").

### `Combobox`

Autocomplete dropdown. Untuk pilih produk, mitra, supplier.

Behavior:
- Type 2+ char → fetch suggestion via HTMX
- Arrow up/down navigate
- Enter pilih
- Esc tutup
- Tab move ke field berikut

### `NumberInput`

Input angka dengan format ribuan otomatis (Alpine.js mask). Support prefix `Rp`, decimal, increment/decrement button.

### `DatePicker`

Native `<input type="date">` di mobile, custom di desktop dengan kalender popup. Default Indonesia locale.

### `Select`

Dropdown. Pakai native `<select>` untuk simplisitas + accessibility.

### `Checkbox`, `Radio`, `Textarea`

Standar form elements dengan styling konsisten.

### `Toggle`

Switch on/off. Untuk setting preferences.

## Data Components

### `DataTable`

Tabel utama untuk list data. Fitur:
- Sortable header (click header untuk sort)
- Filter inline per kolom (toggle)
- Pagination 25 / 50 / 100 / 200
- Sticky header saat scroll
- Row click → detail (HTMX swap)
- Row hover highlight
- Selected row checkbox (untuk bulk action)
- Empty state built-in
- Skeleton loading

Props:
```go
type DataTableProps struct {
    Columns      []Column
    Rows         []Row
    Total        int
    Page         int
    PerPage      int
    SortBy       string
    SortDir      string
    EmptyTitle   string
    EmptyAction  templ.Component
}
```

### `EmptyState`

Tampilan saat data kosong. Ilustrasi (SVG) + judul + deskripsi + CTA button.

```templ
templ EmptyState(props EmptyStateProps) {
    <div class="flex flex-col items-center justify-center py-16 text-center">
        @icon.Inbox(48, "text-zinc-300")
        <h3 class="text-lg font-medium mt-4">{ props.Title }</h3>
        <p class="text-sm text-zinc-500 mt-1 max-w-sm">{ props.Description }</p>
        if props.Action != nil {
            <div class="mt-4">
                @props.Action
            </div>
        }
    </div>
}
```

### `Skeleton`

Loading placeholder. Pakai shimmer animation (Tailwind `animate-pulse`).

Variant: `SkeletonText`, `SkeletonCard`, `SkeletonTable`.

### `Badge`

Status indicator. Variant: `default`, `success`, `warning`, `danger`, `info`.

```templ
templ Badge(text string, variant string) {
    <span class={ "inline-flex items-center px-2 py-0.5 rounded-md text-xs font-medium", badgeColor(variant) }>
        { text }
    </span>
}
```

### `StatCard`

KPI card untuk dashboard. Title, value besar, delta (up/down vs previous), icon.

```templ
templ StatCard(props StatCardProps) {
    <div class="bg-white rounded-xl p-5 shadow-sm border border-zinc-100">
        <div class="flex items-center justify-between">
            <span class="text-sm text-zinc-500">{ props.Title }</span>
            @icon.Render(props.Icon, 20, "text-zinc-400")
        </div>
        <div class="mt-2 text-2xl font-semibold tabular-nums">{ props.Value }</div>
        if props.Delta != "" {
            <div class={ "mt-1 text-xs", deltaColor(props.DeltaDir) }>
                { props.Delta }
            </div>
        }
    </div>
}
```

### `Pagination`

Standar pagination component. First, prev, page numbers, next, last + per-page selector.

## Feedback Components

### `Toast`

Notifikasi temporer top-right. Variant: success, error, info, warning. Auto-dismiss 5 detik (configurable).

Trigger via HTMX header `HX-Trigger: showToast` dengan payload JSON.

### `Modal`

Dialog modal center. Backdrop blur, ESC close, click outside close.

### `ConfirmDialog`

Modal konfirmasi destructive. Untuk delete: type nama untuk confirm.

### `Drawer`

Side panel slide-in dari kanan. Untuk form quick edit, detail view.

### `Spinner`

Loading indicator kecil. Untuk button loading state.

### `ProgressBar`

Untuk import data, upload file, batch operation.

### `Alert`

Inline alert dalam form atau page. Variant: info, success, warning, danger.

## Navigation Components

### `NavItem`

Item sidebar dengan icon + label. Active state highlight.

### `Tabs`

Tab horizontal. Untuk segmen detail page (overview, transaksi, piutang per mitra).

### `Stepper`

Multi-step progress. Untuk form wizard (mutasi gudang multi-step, stok opname).

## Domain-Specific Components

### `ProductPicker`

Kombinasi `Combobox` + display thumbnail/SKU/stok per gudang. Untuk form penjualan.

Behavior:
- Search by nama atau SKU
- Tampil stok di gudang aktif (warna: hijau cukup, kuning rendah, merah habis)
- Pilih → auto-fill harga dan satuan default

### `MitraPicker`

`Combobox` dengan info mitra: nama, kontak, sisa limit kredit, status piutang.

### `UnitConverter`

Widget konversi satuan. Input: qty + satuan asal → output: qty di satuan target.

Untuk form penjualan, tampilkan konversi otomatis (5 sak = 27.5 kg jika faktor 5.5).

### `KwitansiPreview`

Live preview kwitansi sebelum dicetak. Render layout PDF A5 di iframe atau canvas.

Props: data penjualan + template ID. Update real-time saat user edit form.

### `StokBadge`

Badge dengan warna sesuai level stok:
- Hijau: stok normal (≥ minimum)
- Kuning: stok mendekati minimum
- Merah: stok habis atau di bawah minimum

### `PiutangAgingBadge`

Visual aging piutang:
- Hijau: current
- Kuning: 1-30 hari
- Oranye: 31-60 hari
- Merah: 61-90 hari
- Hitam: 90+ hari

### `RoleBadge`

Tampilkan role user dengan warna:
- Owner: ungu
- Admin: biru
- Kasir: hijau
- Gudang: oranye

## Print Components

### `PrintLayoutA5`

Templ template untuk PDF kwitansi A5. Komponen ini di-render Go-side, output bytes via gofpdf.

Section:
- Header kop toko (logo + alamat + telepon)
- Nomor kwitansi + tanggal + tempat
- "Sudah terima dari ___"
- "Uang sejumlah Rp ___"
- "Terbilang: ___"
- "Untuk pembayaran ___"
- Tabel item (no, nama, qty, satuan, harga, total)
- Total + diskon + grand total
- Tanda tangan + tempat materai
- Watermark "ASLI" / "TEMBUSAN"

### `PrintLayoutDotMatrix`

Text template fixed-width untuk dot matrix. Output via ESC/P stream.

Format Sample:

```
====================================================================
              UD. TOKO BANGUNAN XYZ
              Jl. Raya Canggu No. 123, Bali
              Telp: 0361-XXXXXX
====================================================================
No: INV/2025/01/0001                       Tanggal: 03 Mei 2026

Sudah terima dari : PT MAJU JAYA
Uang sejumlah     : Rp     3.400.000,-
Terbilang         : Tiga juta empat ratus ribu rupiah
Untuk pembayaran  : Pembelian Semen Portland 50 sak

------------------------------------------------------------------
No  Item                    Qty  Sat  Harga       Total
------------------------------------------------------------------
1   Semen Portland 50kg     50   Sak  68.000      3.400.000
------------------------------------------------------------------

                                  Canggu, 03 Mei 2026
                                  Penerima,



                                  (___________________)
```

## Icon Library

Komponen `icon` di `internal/view/icon/`:

```go
// internal/view/icon/icon.go
package icon

func Render(name string, size int, className string) templ.Component
```

Daftar icon Lucide yang dipakai (subset, di-bundle inline SVG saat build):
- Navigation: `home`, `shopping-cart`, `package`, `truck`, `users`, `briefcase`, `bar-chart-3`, `settings`
- Action: `plus`, `pencil`, `trash`, `check`, `x`, `download`, `upload`, `printer`, `search`, `filter`, `more-horizontal`
- Status: `check-circle-2`, `x-circle`, `alert-triangle`, `info`, `clock`
- UI: `chevron-down`, `chevron-up`, `chevron-left`, `chevron-right`, `arrow-up`, `arrow-down`
- Feedback: `inbox` (empty), `loader-2` (spinner)

Semua icon ukuran 16/20/24px. Stroke width default 2.

## Testing Komponen

- **Visual regression test** (Fase 8): Playwright screenshot diff
- **Smoke test**: setiap komponen di-render di Storybook-like page (`/dev/components`)
- **Unit test**: Templ component yang ada logic Go (badge color, deltaColor) di-test
