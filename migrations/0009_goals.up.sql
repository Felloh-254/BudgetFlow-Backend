CREATE TABLE goals (
    id             SERIAL PRIMARY KEY,
    user_id        INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name           TEXT NOT NULL,
    target_amount  NUMERIC(14,2) NOT NULL CHECK (target_amount > 0),
    current_amount NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (current_amount >= 0),
    target_date    DATE,
    color          TEXT NOT NULL DEFAULT '#10b981',
    status         TEXT NOT NULL DEFAULT 'active'
                   CHECK (status IN ('active', 'completed', 'cancelled')),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_goals_user_id ON goals(user_id);
CREATE INDEX idx_goals_user_status ON goals(user_id, status);