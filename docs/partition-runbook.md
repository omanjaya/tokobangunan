# Penjualan Partition Runbook

Tabel `penjualan` adalah RANGE-partitioned per tahun (kolom `tanggal`).
Partisi yang sudah ada di production:

- `penjualan_2020` ... `penjualan_2030` (eksplisit)
- `penjualan_2031` ... `penjualan_2040` (dari migrasi 0032)
- `penjualan_default` (catch-all, dari migrasi 0032)

INSERT akan gagal jika tahun berada di luar range partisi yang terdaftar
**dan** DEFAULT tidak ada. Karena DEFAULT sudah ada, kegagalan silent
(jatuh ke default) lebih mungkin terjadi — dan default partition mengganggu
constraint exclusion. Maka best practice: tetap buat partisi tahun-spesifik
sebelum tahun berjalan.

## Auto-rollover

Script `scripts/partition-rollover.sh` membuat partisi tahun depan
(`penjualan_<YYYY>`) jika belum ada. Idempotent.

### Manual run

```bash
bash scripts/partition-rollover.sh
```

### Cron (recommended)

Jalankan setiap 1 Januari pukul 00:00:

```cron
0 0 1 1 * cd /path/to/tokobangunan && bash scripts/partition-rollover.sh >> /var/log/penjualan-partition.log 2>&1
```

Atau lebih defensif, tiap minggu untuk mengantisipasi server down saat 1 Jan:

```cron
0 1 * * 1 cd /path/to/tokobangunan && bash scripts/partition-rollover.sh >> /var/log/penjualan-partition.log 2>&1
```

### Override DB target

Set env var sebelum eksekusi:

- `TOKOBANGUNAN_DB_CONTAINER` (default `tokobangunan-db`)
- `TOKOBANGUNAN_DB_USER` (default `dev`)
- `TOKOBANGUNAN_DB_NAME` (default `tokobangunan`)

## Verifikasi partisi

```sql
SELECT inhrelid::regclass FROM pg_inherits
WHERE inhparent = 'penjualan'::regclass ORDER BY 1;
```

## Rollback partisi tahunan

Drop hanya jika partisi kosong:

```sql
SELECT count(*) FROM penjualan_2030;  -- pastikan 0
DROP TABLE penjualan_2030;
```
