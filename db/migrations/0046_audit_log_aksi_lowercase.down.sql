-- 0046 down: tidak ada rollback bermakna (lowercase normalization idempotent).
-- No-op statement supaya migrator tetap bisa apply down step.
SELECT 1;
