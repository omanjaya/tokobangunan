-- Aktifkan ekstensi PostgreSQL yang dibutuhkan: pg_trgm untuk fuzzy search produk/mitra,
-- uuid-ossp untuk generate UUID (client_uuid idempotency dan session id).
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
