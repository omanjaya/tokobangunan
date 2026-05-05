# Dokumentasi Sistem Toko Bangunan

Index dokumentasi lengkap untuk aplikasi manajemen toko bangunan multi-cabang. Sistem ini menggantikan workflow Excel eksisting (5 cabang, 250.000+ transaksi) dengan web app berbasis Go.

## Status Project

| Fase | Nama | Status |
|------|------|--------|
| 0 | Setup folder + docs | In Progress |
| 1 | Master data + auth | Pending |
| 2 | Penjualan + kwitansi rangkap | Pending |
| 3 | Stok + mutasi antar gudang | Pending |
| 4 | Piutang + pembayaran | Pending |
| 5 | Stok opname + hutang supplier | Pending |
| 6 | Laporan & dashboard | Pending |
| 7 | Migrasi data Excel | Pending |
| 8 | Offline mode + UAT lokal | Pending |
| 9 | Deployment ke VPS | Tertunda (setelah MVP) |

## Daftar Dokumen

| # | File | Topik | Untuk Siapa |
|---|------|-------|-------------|
| 01 | [01-plan.md](01-plan.md) | Plan eksekusi & roadmap | Project manager, owner, dev |
| 02 | [02-system-design.md](02-system-design.md) | System design overview | Tech lead, dev |
| 03 | [03-layered-architecture.md](03-layered-architecture.md) | Layered architecture | Backend dev |
| 04 | [04-clean-code-practices.md](04-clean-code-practices.md) | Konvensi clean code | Semua dev |
| 05 | [05-shared-components.md](05-shared-components.md) | Library komponen UI | Frontend dev, designer |
| 06 | [06-database-schema.md](06-database-schema.md) | Skema database PostgreSQL | Backend dev, DBA |
| 07 | [07-ui-ux.md](07-ui-ux.md) | Design system & UX pattern | Designer, frontend dev |
| 08 | [08-security.md](08-security.md) | Auth, RBAC, audit, backup | Tech lead, ops |
| 09 | [09-dev-environment.md](09-dev-environment.md) | Setup Docker + hot reload | Dev baru |

## Urutan Baca Rekomendasi

### Untuk Owner / Non-Teknis
1. `01-plan.md` (skip section teknis)
2. `07-ui-ux.md` (lihat wireframe)

### Untuk Developer Baru
1. `09-dev-environment.md` — setup lokal dulu
2. `01-plan.md` — paham scope & timeline
3. `02-system-design.md` — paham arsitektur high-level
4. `03-layered-architecture.md` — paham layer
5. `04-clean-code-practices.md` — paham konvensi
6. `06-database-schema.md` — paham data model
7. `05-shared-components.md` — saat mulai frontend
8. `08-security.md` — sebelum deploy

### Untuk Designer
1. `07-ui-ux.md`
2. `05-shared-components.md`
3. `01-plan.md` (untuk konteks bisnis)

## Konvensi Dokumentasi

- Tidak ada emoji
- Heading dengan prefix angka
- Diagram pakai Mermaid (text-based)
- Code block dengan language tag
- Asset gambar di `docs/assets/`
- Update doc relevan setiap kali ada perubahan arsitektur atau skema

## Tools yang Dibutuhkan untuk Dev Lokal

- Docker Desktop atau Docker Engine + Docker Compose v2
- Git
- Editor dengan Templ + Go support (VSCode + extension `templ-go.templ` direkomendasikan)
- Browser modern

Lihat `09-dev-environment.md` untuk setup detail.

## Kontak & Lisensi

Internal use. Lisensi ditentukan setelah branding & legal review.
