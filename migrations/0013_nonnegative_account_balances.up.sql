DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'account_balances'::regclass
          AND conname = 'account_balances_nonnegative_check'
    ) THEN
        ALTER TABLE account_balances
            ADD CONSTRAINT account_balances_nonnegative_check
            CHECK (balance >= 0)
            NOT VALID;
    END IF;
END
$$;
