-- account_balances is the canonical stored balance. The legacy accounts.balance
-- column is no longer written or read by the application.
ALTER TABLE accounts DROP COLUMN IF EXISTS balance;
