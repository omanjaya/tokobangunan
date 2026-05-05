-- Allow cancel after status 'dikirim' (revert stok ke gudang_asal).
-- Replace fungsi apply_mutasi_stok untuk handle case OLD='dikirim' AND NEW='dibatalkan'.
CREATE OR REPLACE FUNCTION apply_mutasi_stok()
RETURNS TRIGGER AS $$
DECLARE
    item RECORD;
BEGIN
    IF NEW.status = OLD.status THEN
        RETURN NEW;
    END IF;

    -- draft -> dikirim : kurangi stok di gudang_asal.
    IF OLD.status = 'draft' AND NEW.status = 'dikirim' THEN
        FOR item IN
            SELECT produk_id, qty_konversi FROM mutasi_item WHERE mutasi_id = NEW.id
        LOOP
            INSERT INTO stok (gudang_id, produk_id, qty)
            VALUES (NEW.gudang_asal_id, item.produk_id, -item.qty_konversi)
            ON CONFLICT (gudang_id, produk_id)
            DO UPDATE SET qty = stok.qty - item.qty_konversi,
                          updated_at = now();
        END LOOP;
        RETURN NEW;
    END IF;

    -- dikirim -> diterima : tambah stok di gudang_tujuan.
    IF OLD.status = 'dikirim' AND NEW.status = 'diterima' THEN
        FOR item IN
            SELECT produk_id, qty_konversi FROM mutasi_item WHERE mutasi_id = NEW.id
        LOOP
            INSERT INTO stok (gudang_id, produk_id, qty)
            VALUES (NEW.gudang_tujuan_id, item.produk_id, item.qty_konversi)
            ON CONFLICT (gudang_id, produk_id)
            DO UPDATE SET qty = stok.qty + item.qty_konversi,
                          updated_at = now();
        END LOOP;
        RETURN NEW;
    END IF;

    -- dikirim -> dibatalkan : revert stok ke gudang_asal (tambah kembali).
    IF OLD.status = 'dikirim' AND NEW.status = 'dibatalkan' THEN
        FOR item IN
            SELECT produk_id, qty_konversi FROM mutasi_item WHERE mutasi_id = NEW.id
        LOOP
            INSERT INTO stok (gudang_id, produk_id, qty)
            VALUES (NEW.gudang_asal_id, item.produk_id, item.qty_konversi)
            ON CONFLICT (gudang_id, produk_id)
            DO UPDATE SET qty = stok.qty + item.qty_konversi,
                          updated_at = now();
        END LOOP;
        RETURN NEW;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
