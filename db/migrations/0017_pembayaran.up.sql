-- Pembayaran customer (mitra) ke penjualan kredit/sebagian.
-- Trigger AFTER INSERT/UPDATE/DELETE recompute penjualan.status_bayar
-- berdasarkan SUM(jumlah) vs penjualan.total.

CREATE TABLE pembayaran (
    id                 BIGSERIAL PRIMARY KEY,
    penjualan_id       BIGINT NULL,
    penjualan_tanggal  DATE NULL,
    mitra_id           BIGINT NOT NULL REFERENCES mitra(id),
    tanggal            DATE NOT NULL,
    jumlah             BIGINT NOT NULL,
    metode             TEXT NOT NULL,
    referensi          TEXT,
    user_id            BIGINT NOT NULL REFERENCES "user"(id),
    catatan            TEXT,
    client_uuid        UUID NOT NULL UNIQUE,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (penjualan_id, penjualan_tanggal) REFERENCES penjualan(id, tanggal)
);

CREATE INDEX idx_pembayaran_mitra_tanggal ON pembayaran(mitra_id, tanggal);
CREATE INDEX idx_pembayaran_penjualan     ON pembayaran(penjualan_id, penjualan_tanggal);

-- Function: recompute status_bayar untuk satu penjualan.
CREATE OR REPLACE FUNCTION pembayaran_recompute_status(
    p_penjualan_id BIGINT,
    p_penjualan_tanggal DATE
) RETURNS VOID AS $$
DECLARE
    v_total       BIGINT;
    v_dibayar     BIGINT;
    v_status_lama TEXT;
    v_status_baru TEXT;
BEGIN
    IF p_penjualan_id IS NULL OR p_penjualan_tanggal IS NULL THEN
        RETURN;
    END IF;

    SELECT total, status_bayar INTO v_total, v_status_lama
    FROM penjualan
    WHERE id = p_penjualan_id AND tanggal = p_penjualan_tanggal;

    IF NOT FOUND THEN
        RETURN;
    END IF;

    SELECT COALESCE(SUM(jumlah), 0) INTO v_dibayar
    FROM pembayaran
    WHERE penjualan_id = p_penjualan_id AND penjualan_tanggal = p_penjualan_tanggal;

    IF v_dibayar >= v_total THEN
        v_status_baru := 'lunas';
    ELSIF v_dibayar > 0 THEN
        v_status_baru := 'sebagian';
    ELSE
        v_status_baru := 'kredit';
    END IF;

    IF v_status_baru <> v_status_lama THEN
        UPDATE penjualan
           SET status_bayar = v_status_baru,
               updated_at   = now()
         WHERE id = p_penjualan_id AND tanggal = p_penjualan_tanggal;
    END IF;
END;
$$ LANGUAGE plpgsql;

-- Trigger function: panggil recompute untuk OLD dan NEW (kalau penjualan_id berubah).
CREATE OR REPLACE FUNCTION trg_pembayaran_after_change() RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        PERFORM pembayaran_recompute_status(NEW.penjualan_id, NEW.penjualan_tanggal);
        RETURN NEW;
    ELSIF TG_OP = 'UPDATE' THEN
        PERFORM pembayaran_recompute_status(OLD.penjualan_id, OLD.penjualan_tanggal);
        IF NEW.penjualan_id IS DISTINCT FROM OLD.penjualan_id
           OR NEW.penjualan_tanggal IS DISTINCT FROM OLD.penjualan_tanggal THEN
            PERFORM pembayaran_recompute_status(NEW.penjualan_id, NEW.penjualan_tanggal);
        END IF;
        RETURN NEW;
    ELSIF TG_OP = 'DELETE' THEN
        PERFORM pembayaran_recompute_status(OLD.penjualan_id, OLD.penjualan_tanggal);
        RETURN OLD;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER pembayaran_after_change
AFTER INSERT OR UPDATE OR DELETE ON pembayaran
FOR EACH ROW EXECUTE FUNCTION trg_pembayaran_after_change();
