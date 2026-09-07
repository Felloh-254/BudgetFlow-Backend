DROP INDEX IF EXISTS idx_transactions_account_id;
ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_account_id_fkey;
ALTER TABLE transactions DROP COLUMN IF EXISTS account_id;