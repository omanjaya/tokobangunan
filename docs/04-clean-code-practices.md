# 04 — Clean Code Practices

Konvensi yang berlaku untuk seluruh codebase. Tujuan: codebase mudah dibaca, di-review, dan di-maintain oleh developer baru.

## Naming Convention

### Go

| Element | Convention | Contoh |
|---------|------------|--------|
| Package | `lowercase`, satu kata, no underscore | `penjualan`, `escpos` |
| Type exported | `PascalCase` | `Penjualan`, `MutasiService` |
| Type unexported | `camelCase` | `penjualanRepo` |
| Function exported | `PascalCase`, verb | `Create`, `HitungTotal` |
| Function unexported | `camelCase` | `validateInput` |
| Variable | `camelCase`, descriptive | `mitraID`, `totalKwitansi` |
| Constant | `PascalCase` atau `ALL_CAPS` untuk public | `MaxKredit`, `DefaultPort` |
| Interface | `PascalCase` + suffix `er` atau noun | `Repository`, `Printer`, `Encoder` |
| Receiver | 1-2 huruf | `func (p *Penjualan)`, `func (s *Service)` |
| Test | `TestXxx` | `TestPenjualan_HitungTotal_DenganDiskon` |

### SQL & File

| Element | Convention | Contoh |
|---------|------------|--------|
| Tabel | `snake_case`, singular | `penjualan`, `mutasi_gudang` |
| Kolom | `snake_case` | `nomor_kwitansi`, `created_at` |
| Index | `idx_table_columns` | `idx_penjualan_tanggal_gudang` |
| Foreign key | `fk_table_target` | `fk_penjualan_mitra` |
| File Go | `snake_case.go` | `penjualan_service.go` |
| File migration | `NNNN_description.{up,down}.sql` | `0001_init.up.sql` |
| File Templ | `snake_case.templ` | `penjualan_form.templ` |

### Bahasa untuk Identifier

- **Domain term dalam Bahasa Indonesia** karena context bisnis lokal: `Penjualan`, `Mitra`, `Gudang`, `Kwitansi`
- **Technical term dalam Bahasa Inggris**: `Service`, `Repository`, `Handler`, `Context`
- Konsisten — jangan campur `Sale` dan `Penjualan`

## Struktur File

- **Maksimal 300 baris per file** — split ke file lain kalau lebih
- **1 file = 1 type utama** + helper-nya yang erat terkait
- **Test file** sebelahan dengan source: `penjualan.go` ↔ `penjualan_test.go`
- Order dalam file: package doc → import → const → var → type → func receiver method → func helper

## Error Handling

### Wrap dengan Context

Selalu wrap error dengan context yang membantu debugging:

```go
// Bad
return err

// Good
return fmt.Errorf("ambil mitra ID %d: %w", mitraID, err)
```

### Sentinel Error di Domain

Error yang berarti business meaning didefinisikan sebagai sentinel di domain:

```go
// internal/domain/error.go
var (
    ErrMitraTidakDitemukan      = errors.New("mitra tidak ditemukan")
    ErrLimitKreditTerlampaui    = errors.New("limit kredit terlampaui")
    ErrStokTidakCukup           = errors.New("stok tidak cukup")
    ErrPenjualanKosong          = errors.New("penjualan tidak boleh kosong")
)
```

Service return sentinel error → handler `errors.Is` → translate ke HTTP code.

### Translation di Handler

```go
func (h *PenjualanHandler) translateError(c echo.Context, err error) error {
    switch {
    case errors.Is(err, domain.ErrMitraTidakDitemukan):
        return c.Render(http.StatusNotFound, view.ErrorAlert(err.Error()))
    case errors.Is(err, domain.ErrLimitKreditTerlampaui):
        return c.Render(http.StatusUnprocessableEntity, view.ErrorAlert(err.Error()))
    case errors.Is(err, domain.ErrStokTidakCukup):
        return c.Render(http.StatusConflict, view.ErrorAlert(err.Error()))
    default:
        slog.ErrorContext(c.Request().Context(), "internal error", "error", err)
        return c.Render(http.StatusInternalServerError, view.ErrorAlert("Terjadi kesalahan, silakan coba lagi"))
    }
}
```

### Tidak Pernah Panic di Production Code

- `panic` hanya di `main.go` setup phase atau benar-benar unrecoverable
- Recover di middleware untuk amankan request lain
- Test boleh pakai `t.Fatal`

## Testing

### Unit Test

- Domain layer: 100% coverage untuk business rule
- Service layer: 80%+ dengan mock repository
- Table-driven test untuk multiple case

```go
func TestPenjualan_HitungTotal(t *testing.T) {
    tests := []struct {
        name     string
        items    []PenjualanItem
        diskon   Uang
        expected Uang
    }{
        {"satu item tanpa diskon", []PenjualanItem{{Subtotal: Rp(100_000)}}, Rp(0), Rp(100_000)},
        {"dua item dengan diskon", []PenjualanItem{{Subtotal: Rp(50_000)}, {Subtotal: Rp(50_000)}}, Rp(10_000), Rp(90_000)},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            p := &Penjualan{Items: tt.items, Diskon: tt.diskon}
            p.HitungTotal()
            if !p.Total.Equal(tt.expected) {
                t.Errorf("got %v, want %v", p.Total, tt.expected)
            }
        })
    }
}
```

### Integration Test

- Repository test dengan `testcontainers-go` (Postgres real)
- Handler test dengan service + repo real
- Setup/teardown di `TestMain`

### Coverage Target

| Layer | Target Coverage |
|-------|-----------------|
| Domain | 100% |
| Service | 80%+ |
| Repository | 70%+ (covered by integration) |
| Handler | 60%+ |
| Overall | 70%+ |

CI gate: PR ditolak kalau coverage turun > 2%.

## Logging

### Pakai `log/slog` Standar Library

```go
slog.InfoContext(ctx, "penjualan dibuat",
    "id", penjualan.ID,
    "mitra_id", penjualan.MitraID,
    "total", penjualan.Total)
```

### Level

| Level | Kapan |
|-------|-------|
| DEBUG | Detail flow (off di produksi) |
| INFO | Operasi normal sukses |
| WARN | Masalah recoverable |
| ERROR | Operasi gagal yang perlu investigasi |

### Format

- Format **JSON** untuk produksi (mudah di-parse log aggregator)
- Format **text** untuk dev (mudah dibaca)
- Setting via env `LOG_FORMAT=json|text`, `LOG_LEVEL=debug|info|warn|error`

### Yang Tidak Boleh Di-Log

- Password (raw atau hash)
- Token session
- Detail kartu / pembayaran
- PII tanpa kebutuhan jelas

## Validation

- **Format validation** di handler (struct tag + `validator/v10`)
- **Business validation** di service layer (cek limit, stok, dll)
- Validation error return list of field error, bukan single string

```go
type PenjualanCreateInput struct {
    MitraID  int64                  `json:"mitra_id" validate:"required,min=1"`
    GudangID int64                  `json:"gudang_id" validate:"required,min=1"`
    Items    []PenjualanItemInput   `json:"items" validate:"required,min=1,dive"`
    Diskon   int64                  `json:"diskon" validate:"min=0"`
}
```

## Konvensi Lain

### Comment

- **Default: tidak ada komentar** kecuali alasan WHY non-obvious
- Jangan duplikasi nama fungsi di komentar
- Komentar untuk: workaround bug, hidden constraint, invariant subtle

### TODO

- Tidak ada `TODO` tanpa issue tracker linked
- Format: `// TODO(gh-123): handle ...` atau `// FIXME(linear-456): race condition`

### Dead Code

- Tidak commit code yang di-comment-out
- Hapus, simpan di git history kalau perlu lihat lagi

### Premature Abstraction

- Rule of three: bikin abstraksi kalau ada **3 use case** nyata
- 3 baris similar lebih baik dari 1 abstraction yang salah

### Magic Number

- Extract ke const dengan nama deskriptif

```go
// Bad
if percobaan > 5 { ... }

// Good
const MaxPercobaanLogin = 5
if percobaan > MaxPercobaanLogin { ... }
```

## Konvensi Commit

Format: **Conventional Commits**

```
<type>(<scope>): <subject>

<body>

<footer>
```

Type: `feat`, `fix`, `refactor`, `docs`, `test`, `chore`, `style`, `perf`

Contoh:

```
feat(penjualan): tambah validasi limit kredit mitra

Mitra dengan tipe "proyek" punya limit kredit yang harus dicek
sebelum penjualan kredit dibuat. Cek dilakukan di service layer
agar konsisten dengan flow lain.

Closes #45
```

Subject ≤ 72 karakter, body wrap 80 karakter, gunakan present tense ("tambah", bukan "menambahkan").

## Pre-Commit Hook

Wajib pass sebelum commit:

```bash
go fmt ./...
go vet ./...
golangci-lint run
templ generate
go test ./... -short
```

Setup via `.git/hooks/pre-commit` atau `lefthook` (file `lefthook.yml` ditambahkan di Fase 1).

## Code Review Checklist

Reviewer cek:

- Naming jelas dan konsisten
- Error di-wrap dengan context
- Tidak ada commented-out code
- Test ditambahkan untuk logic baru
- Tidak ada `time.Now()` langsung di service (pakai `clock.Clock` interface untuk testability)
- Tidak ada SQL string concat
- Validasi input ada di handler, validasi business ada di service
- Tidak ada panic di production path
