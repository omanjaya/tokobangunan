DROP TRIGGER IF EXISTS pembayaran_breakdown_validate ON pembayaran;
DROP FUNCTION IF EXISTS trg_pembayaran_breakdown_validate();
ALTER TABLE pembayaran DROP COLUMN IF EXISTS metode_breakdown;
