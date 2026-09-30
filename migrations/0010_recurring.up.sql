CREATE TABLE recurring_rules (
    id                SERIAL PRIMARY KEY,
    user_id           INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type              TEXT NOT NULL CHECK (type IN ('income', 'expense', 'transfer')),
    title             TEXT NOT NULL,
    amount            NUMERIC(19,2) NOT NULL CHECK (amount > 0),
    note              TEXT NOT NULL DEFAULT '',
    category_id       INTEGER REFERENCES categories(id) ON DELETE SET NULL,
    account_id        INTEGER REFERENCES accounts(id) ON DELETE CASCADE,
    from_account_id   INTEGER REFERENCES accounts(id) ON DELETE CASCADE,
    to_account_id     INTEGER REFERENCES accounts(id) ON DELETE CASCADE,
    frequency         TEXT NOT NULL CHECK (frequency IN ('daily','weekly','monthly')),
    interval_count    INTEGER NOT NULL DEFAULT 1 CHECK (interval_count > 0),
    start_date        DATE NOT NULL,
    end_date          DATE,
    next_run_at       DATE NOT NULL,
    last_run_at       DATE,
    active            BOOLEAN NOT NULL DEFAULT true,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_recurring_user_id ON recurring_rules(user_id);
CREATE INDEX idx_recurring_next_run ON recurring_rules(next_run_at) WHERE active = true;

CREATE TABLE recurring_rule_runs (
    id             SERIAL PRIMARY KEY,
    rule_id        INTEGER NOT NULL REFERENCES recurring_rules(id) ON DELETE CASCADE,
    ran_for_date   DATE NOT NULL,
    transaction_id INTEGER REFERENCES transactions_v2(id) ON DELETE SET NULL,
    status         TEXT NOT NULL CHECK (status IN ('success','failed')),
    error          TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_recurring_runs_rule ON recurring_rule_runs(rule_id);

CREATE UNIQUE INDEX idx_recurring_rule_run_unique
    ON recurring_rule_runs (rule_id, ran_for_date);