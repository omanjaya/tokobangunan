-- Stok per gudang per produk. PK composite (gudang_id, produk_id).
-- qty disimpan dalam satuan_kecil (sesuai produk.satuan_kecil_id).
-- Trigger pada mutasi_gudang akan auto-update stok saat status berubah.
CREATE TABLE stok (
    gudang_id   BIGINT NOT NULL REFERENCES gudang(id),
    produk_id   BIGINT NOT NULL REFERENCES produk(id),
    qty         NUMERIC(14, 4) NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (gudang_id, produk_id)
);

CREATE INDEX idx_stok_produk ON stok(produk_id);
CREATE INDEX idx_stok_kosong ON stok(gudang_id, produk_id) WHERE qty <= 0;

CREATE TRIGGER trg_stok_updated BEFORE UPDATE ON stok
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- Trigger: ketika mutasi_gudang.status berpindah ke 'dikirim', kurangi stok
-- gudang_asal. Ketika berpindah ke 'diterima', tambah stok gudang_tujuan.
-- Status hanya boleh progress maju: draft -> dikirim -> diterima.
-- (dibatalkan hanya valid dari draft sehingga tidak menyentuh stok.)
-- ---------------------------------------------------------------------------
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

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_mutasi_apply_stok
    AFTER UPDATE OF status ON mutasi_gudang
    FOR EACH ROW
    WHEN (NEW.status IS DISTINCT FROM OLD.status)
    EXECUTE FUNCTION apply_mutasi_stok();
