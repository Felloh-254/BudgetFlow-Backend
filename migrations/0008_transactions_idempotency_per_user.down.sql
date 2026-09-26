BEGIN;

DROP INDEX IF EXISTS idx_transactions_v2_user_idempotency_key;

CREATE UNIQUE INDEX IF NOT EXISTS idx_transactions_v2_idempotency
ON transactions_v2 (idempotency_key)
WHERE idempotency_key IS NOT NULL;

COMMIT;
