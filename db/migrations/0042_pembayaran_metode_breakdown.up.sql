-- Multi-metode pembayaran per row.
-- Satu pembayaran (1 row) bisa terdiri dari beberapa metode (mis. tunai 500rb +
-- transfer 1.5jt = 1 row dengan breakdown JSONB). Kolom `metode` lama tetap dipakai
-- sebagai metode utama / fallback untuk row tanpa breakdown (backward-compat).

ALTER TABLE pembayaran ADD COLUMN IF NOT EXISTS metode_breakdown JSONB;

-- Pakai trigger (CHECK constraint tidak boleh subquery) untuk enforce
-- sum(elem.jumlah) == jumlah header.
CREATE OR REPLACE FUNCTION trg_pembayaran_breakdown_validate() RETURNS TRIGGER AS $$
DECLARE
    v_sum BIGINT;
BEGIN
    IF NEW.metode_breakdown IS NULL THEN
        RETURN NEW;
    END IF;
    IF jsonb_typeof(NEW.metode_breakdown) <> 'array' THEN
        RAISE EXCEPTION 'metode_breakdown harus berupa JSON array';
    END IF;
    SELECT COALESCE(SUM((elem->>'jumlah')::BIGINT), 0)
      INTO v_sum
      FROM jsonb_array_elements(NEW.metode_breakdown) elem;
    IF v_sum <> NEW.jumlah THEN
        RAISE EXCEPTION 'metode_breakdown sum (%) tidak sama dengan jumlah header (%)',
            v_sum, NEW.jumlah;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER pembayaran_breakdown_validate
BEFORE INSERT OR UPDATE OF metode_breakdown, jumlah ON pembayaran
FOR EACH ROW EXECUTE FUNCTION trg_pembayaran_breakdown_validate();
