-- Migration 002: Create wallet_transactions table with full audit fields
-- This table provides a complete financial audit trail per the design document.

-- UP
CREATE TABLE IF NOT EXISTS wallet_transactions (
    id              UUID           PRIMARY KEY DEFAULT gen_random_uuid(),
    wallet_id       UUID           NOT NULL REFERENCES wallets(id),
    transaction_id  UUID           NOT NULL,
    idempotency_key VARCHAR(255)   UNIQUE NOT NULL,
    operation_type  VARCHAR(50)    NOT NULL
                        CHECK (operation_type IN ('deposit', 'hold', 'release', 'charge', 'credit')),
    amount          DECIMAL(12,2)  NOT NULL,
    balance_before  DECIMAL(12,2)  NOT NULL,
    balance_after   DECIMAL(12,2)  NOT NULL,
    reference_type  VARCHAR(50),   -- auction, bid, manual
    reference_id    UUID,
    status          VARCHAR(20)    NOT NULL DEFAULT 'completed',
    created_at      TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_wallet_txn_idempotency ON wallet_transactions(idempotency_key);
CREATE INDEX IF NOT EXISTS idx_wallet_txn_wallet ON wallet_transactions(wallet_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_wallet_txn_reference ON wallet_transactions(reference_type, reference_id);

-- DOWN
-- DROP INDEX IF EXISTS idx_wallet_txn_reference;
-- DROP INDEX IF EXISTS idx_wallet_txn_wallet;
-- DROP INDEX IF EXISTS idx_wallet_txn_idempotency;
-- DROP TABLE IF EXISTS wallet_transactions;
