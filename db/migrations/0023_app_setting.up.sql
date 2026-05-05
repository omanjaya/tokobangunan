-- Tabel app_setting: key-value JSON untuk konfigurasi aplikasi.
CREATE TABLE app_setting (
    id         BIGSERIAL PRIMARY KEY,
    key        TEXT NOT NULL UNIQUE,
    value      JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by BIGINT REFERENCES "user"(id)
);

INSERT INTO app_setting (key, value) VALUES
    ('toko_info', '{"nama":"Toko Bangunan","alamat":"","telepon":"","npwp":"","kop_kwitansi":""}'::jsonb),
    ('onboarding_done', 'false'::jsonb)
ON CONFLICT (key) DO NOTHING;
