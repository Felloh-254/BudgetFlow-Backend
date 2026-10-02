ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS provider TEXT;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'accounts'::regclass
          AND conname = 'accounts_provider_check'
    ) THEN
        ALTER TABLE accounts
            ADD CONSTRAINT accounts_provider_check
            CHECK (provider IS NULL OR provider IN ('mpesa', 'airtel_money'))
            NOT VALID;
    END IF;
END
$$;
