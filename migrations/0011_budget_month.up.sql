ALTER TABLE budgets
    ADD COLUMN IF NOT EXISTS month VARCHAR(7) NOT NULL
    DEFAULT to_char(now(), 'YYYY-MM');

UPDATE budgets SET month = to_char(now(), 'YYYY-MM')
WHERE month IS NULL OR month = '';

ALTER TABLE budgets
    ADD CONSTRAINT budgets_month_format
    CHECK (month ~ '^\d{4}-\d{2}$');

ALTER TABLE budgets
    DROP CONSTRAINT IF EXISTS budgets_user_id_category_id_key;
ALTER TABLE budgets
    ADD CONSTRAINT budgets_user_category_month_key
    UNIQUE (user_id, category_id, month);

CREATE INDEX IF NOT EXISTS idx_budgets_user_month ON budgets(user_id, month);