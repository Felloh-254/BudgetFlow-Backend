ALTER TABLE account_balances 
    DROP CONSTRAINT IF EXISTS account_balances_last_updated_txn_fkey,
    ADD CONSTRAINT account_balances_last_updated_txn_fkey 
    FOREIGN KEY (last_updated_txn) REFERENCES transactions_v2(id);
