ALTER TABLE budgets DROP CONSTRAINT IF EXISTS budgets_user_category_month_key;
ALTER TABLE budgets DROP CONSTRAINT IF EXISTS budgets_month_format;
DROP INDEX IF EXISTS idx_budgets_user_month;
ALTER TABLE budgets DROP COLUMN IF EXISTS month;