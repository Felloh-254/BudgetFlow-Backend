-- Rollback ledger entries migration

DROP TABLE IF EXISTS account_balances;
DROP TABLE IF EXISTS transaction_categories;
DROP TABLE IF EXISTS ledger_entries;
DROP TABLE IF EXISTS transactions_v2;
