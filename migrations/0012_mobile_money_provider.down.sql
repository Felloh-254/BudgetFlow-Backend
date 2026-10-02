ALTER TABLE accounts
    DROP CONSTRAINT IF EXISTS accounts_provider_check;

ALTER TABLE accounts
    DROP COLUMN IF EXISTS provider;
