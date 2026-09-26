-- 0004_ledger_entries.up.sql created idx_transactions_v2_idempotency as a GLOBAL
-- unique index on idempotency_key alone. That means no two users, ever, could use
-- the same idempotency key value - too broad, and a possible source of spurious
-- rejections if two different users' clients happen to generate the same key.
-- Idempotency keys should be scoped to the caller (user), not the whole app.
--
-- This also makes the application's `ON CONFLICT (user_id, idempotency_key)`
-- clause in TransactionService valid - Postgres requires a matching unique
-- index/constraint for that conflict target to resolve.

BEGIN;

-- Inspect before running in production if you suspect duplicates already exist:
-- SELECT idempotency_key, COUNT(*) FROM transactions_v2
-- WHERE idempotency_key IS NOT NULL GROUP BY idempotency_key HAVING COUNT(*) > 1;

DROP INDEX IF EXISTS idx_transactions_v2_idempotency;

CREATE UNIQUE INDEX IF NOT EXISTS idx_transactions_v2_user_idempotency_key
ON transactions_v2 (user_id, idempotency_key)
WHERE idempotency_key IS NOT NULL;

COMMIT;
