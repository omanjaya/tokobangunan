-- Stok opname: hitung fisik berkala vs sistem.
-- Status: draft (input fisik) → selesai (review) → approved (adjust stok).

CREATE TABLE stok_opname (
    id          BIGSERIAL PRIMARY KEY,
    nomor       TEXT NOT NULL UNIQUE,
    gudang_id   BIGINT NOT NULL REFERENCES gudang(id),
    tanggal     DATE NOT NULL,
    user_id     BIGINT NOT NULL REFERENCES "user"(id),
    status      TEXT NOT NULL DEFAULT 'draft',     -- draft | selesai | approved
    catatan     TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_stok_opname_gudang_tanggal ON stok_opname(gudang_id, tanggal);
CREATE INDEX idx_stok_opname_status ON stok_opname(status);

CREATE TRIGGER trg_stok_opname_updated BEFORE UPDATE ON stok_opname
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE stok_opname_item (
    id          BIGSERIAL PRIMARY KEY,
    opname_id   BIGINT NOT NULL REFERENCES stok_opname(id) ON DELETE CASCADE,
    produk_id   BIGINT NOT NULL REFERENCES produk(id),
    produk_nama TEXT NOT NULL,                      -- snapshot
    qty_sistem  NUMERIC(14, 4) NOT NULL,
    qty_fisik   NUMERIC(14, 4) NOT NULL DEFAULT 0,
    selisih     NUMERIC(14, 4) GENERATED ALWAYS AS (qty_fisik - qty_sistem) STORED,
    keterangan  TEXT,
    UNIQUE (opname_id, produk_id)
);

CREATE INDEX idx_stok_opname_item_opname ON stok_opname_item(opname_id);

-- Trigger: saat opname status berubah jadi 'approved', set stok = qty_fisik.
CREATE OR REPLACE FUNCTION apply_stok_opname()
RETURNS TRIGGER AS $$
DECLARE
    v_gudang_id BIGINT;
    r RECORD;
BEGIN
    IF NEW.status = 'approved' AND OLD.status <> 'approved' THEN
        v_gudang_id := NEW.gudang_id;

        FOR r IN
            SELECT produk_id, qty_fisik FROM stok_opname_item WHERE opname_id = NEW.id
        LOOP
            UPDATE stok
            SET qty = r.qty_fisik,
                updated_at = now()
            WHERE produk_id = r.produk_id
              AND gudang_id = v_gudang_id;

            IF NOT FOUND THEN
                INSERT INTO stok (gudang_id, produk_id, qty)
                VALUES (v_gudang_id, r.produk_id, r.qty_fisik);
            END IF;
        END LOOP;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_apply_stok_opname AFTER UPDATE ON stok_opname
    FOR EACH ROW EXECUTE FUNCTION apply_stok_opname();
