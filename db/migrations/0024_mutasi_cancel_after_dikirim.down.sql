-- Revert apply_mutasi_stok ke definisi awal (tanpa case dikirim->dibatalkan).
CREATE OR REPLACE FUNCTION apply_mutasi_stok()
RETURNS TRIGGER AS $$
DECLARE
    item RECORD;
BEGIN
    IF NEW.status = OLD.status THEN
        RETURN NEW;
    END IF;

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

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
