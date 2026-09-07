-- Initialize the denormalized balance row for accounts created before
-- account_balances was introduced.
INSERT INTO account_balances (account_id, balance, version)
SELECT id, balance, 1
FROM accounts
ON CONFLICT (account_id) DO NOTHING;
