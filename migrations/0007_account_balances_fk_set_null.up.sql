ALTER TABLE account_balances 
    DROP CONSTRAINT IF EXISTS account_balances_last_updated_txn_fkey,
    ADD CONSTRAINT account_balances_last_updated_txn_fkey 
    FOREIGN KEY (last_updated_txn) REFERENCES transactions_v2(id) ON DELETE SET NULL;

ALTER TABLE transaction_categories 
    DROP CONSTRAINT IF EXISTS transaction_categories_category_id_fkey,
    ADD CONSTRAINT transaction_categories_category_id_fkey 
    FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE CASCADE;

ALTER TABLE ledger_entries 
    DROP CONSTRAINT IF EXISTS ledger_entries_account_id_fkey,
    ADD CONSTRAINT ledger_entries_account_id_fkey 
    FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE;
