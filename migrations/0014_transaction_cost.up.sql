ALTER TABLE transactions_v2
    ADD COLUMN transaction_cost NUMERIC(19, 2) NOT NULL DEFAULT 0
    CHECK (transaction_cost >= 0);
