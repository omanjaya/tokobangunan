-- mitra_access_token: magic link token untuk public portal mitra (read-only).
CREATE TABLE mitra_access_token (
    id          BIGSERIAL PRIMARY KEY,
    mitra_id    BIGINT NOT NULL REFERENCES mitra(id),
    token       TEXT NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked     BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE INDEX idx_mitra_token_lookup ON mitra_access_token(token) WHERE NOT revoked;
CREATE INDEX idx_mitra_token_mitra ON mitra_access_token(mitra_id);
