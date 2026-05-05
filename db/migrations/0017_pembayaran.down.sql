DROP TRIGGER IF EXISTS pembayaran_after_change ON pembayaran;
DROP FUNCTION IF EXISTS trg_pembayaran_after_change();
DROP FUNCTION IF EXISTS pembayaran_recompute_status(BIGINT, DATE);
DROP TABLE IF EXISTS pembayaran;
