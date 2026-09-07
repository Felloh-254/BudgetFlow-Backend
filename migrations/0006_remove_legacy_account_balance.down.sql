-- Restore the legacy column for rollback compatibility.
ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS balance DOUBLE PRECISION NOT NULL DEFAULT 0
    CHECK (balance >= 0);

-- Keep the restored legacy value aligned with the canonical balance cache.
UPDATE accounts a
SET balance = ab.balance
FROM account_balances ab
WHERE ab.account_id = a.id;
