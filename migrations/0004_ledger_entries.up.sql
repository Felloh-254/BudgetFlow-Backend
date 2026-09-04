-- ============================================================
-- Ledger entries: Double-entry accounting foundation
-- ============================================================

-- Refactor transactions to be event-oriented (not account-specific)
-- Old transactions had: user_id, account_id, budget_id, category_id, type, amount, date, note
-- New transactions have: user_id, type (income|expense|transfer), title, date, note

-- First, create the new transactions_v2 to hold event data
CREATE TABLE transactions_v2 (
    id                SERIAL PRIMARY KEY,
    user_id           INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type              TEXT NOT NULL CHECK (type IN ('income', 'expense', 'transfer')),
    title             TEXT NOT NULL,
    date              DATE NOT NULL,
    note              TEXT NOT NULL DEFAULT '',
    idempotency_key   TEXT UNIQUE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_transactions_v2_user_id ON transactions_v2(user_id);
CREATE INDEX idx_transactions_v2_date ON transactions_v2(date);
CREATE INDEX idx_transactions_v2_type ON transactions_v2(type);
CREATE UNIQUE INDEX idx_transactions_v2_idempotency ON transactions_v2(idempotency_key) WHERE idempotency_key IS NOT NULL;

-- Ledger entries: The actual account movements
CREATE TABLE ledger_entries (
    id                SERIAL PRIMARY KEY,
    transaction_id    INTEGER NOT NULL REFERENCES transactions_v2(id) ON DELETE CASCADE,
    account_id        INTEGER NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    amount            NUMERIC(19, 2) NOT NULL,
    entry_type        TEXT NOT NULL CHECK (entry_type IN ('debit', 'credit')),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_ledger_entries_transaction_id ON ledger_entries(transaction_id);
CREATE INDEX idx_ledger_entries_account_id ON ledger_entries(account_id);
CREATE INDEX idx_ledger_entries_created_at ON ledger_entries(created_at);

-- Transaction-to-category mapping (multiple categories per transaction supported)
CREATE TABLE transaction_categories (
    id                SERIAL PRIMARY KEY,
    transaction_id    INTEGER NOT NULL REFERENCES transactions_v2(id) ON DELETE CASCADE,
    category_id       INTEGER NOT NULL REFERENCES categories(id),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (transaction_id, category_id)
);

CREATE INDEX idx_transaction_categories_transaction_id ON transaction_categories(transaction_id);
CREATE INDEX idx_transaction_categories_category_id ON transaction_categories(category_id);

-- Denormalized account balance (computed from ledger entries)
-- This is a cache that must be kept in sync
CREATE TABLE account_balances (
    id                SERIAL PRIMARY KEY,
    account_id        INTEGER NOT NULL UNIQUE REFERENCES accounts(id) ON DELETE CASCADE,
    balance           NUMERIC(19, 2) NOT NULL DEFAULT 0,
    last_updated_txn  INTEGER REFERENCES transactions_v2(id),
    version           INTEGER NOT NULL DEFAULT 1,
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_account_balances_account_id ON account_balances(account_id);
