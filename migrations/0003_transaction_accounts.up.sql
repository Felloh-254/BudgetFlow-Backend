ALTER TABLE transactions
    ADD COLUMN IF NOT EXISTS account_id INTEGER;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'transactions_account_id_fkey'
    ) THEN
        ALTER TABLE transactions
            ADD CONSTRAINT transactions_account_id_fkey
            FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE RESTRICT;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_transactions_account_id ON transactions(account_id);