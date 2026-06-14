-- Payment Service — Initial Schema
-- ACID-compliant financial tables

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Wallets — one per user, tracks available and held balances
CREATE TABLE IF NOT EXISTS wallets (
    id                UUID           PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id           UUID           UNIQUE NOT NULL,
    available_balance NUMERIC(15,2)  NOT NULL DEFAULT 0 CHECK (available_balance >= 0),
    held_balance      NUMERIC(15,2)  NOT NULL DEFAULT 0 CHECK (held_balance >= 0),
    currency          VARCHAR(10)    NOT NULL DEFAULT 'USD',
    created_at        TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

-- Transaction ledger — immutable record of all financial operations
CREATE TABLE IF NOT EXISTS transactions (
    id              UUID           PRIMARY KEY DEFAULT uuid_generate_v4(),
    wallet_id       UUID           NOT NULL REFERENCES wallets(id),
    type            VARCHAR(30)    NOT NULL,
    amount          NUMERIC(15,2)  NOT NULL CHECK (amount > 0),
    reference_id    UUID,
    idempotency_key VARCHAR(255)   UNIQUE NOT NULL,
    status          VARCHAR(20)    NOT NULL DEFAULT 'pending',
    metadata        JSONB          DEFAULT '{}',
    created_at      TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

-- Bid holds — money reserved during active auctions
CREATE TABLE IF NOT EXISTS bid_holds (
    id         UUID           PRIMARY KEY DEFAULT uuid_generate_v4(),
    auction_id UUID           NOT NULL,
    bidder_id  UUID           NOT NULL,
    wallet_id  UUID           NOT NULL REFERENCES wallets(id),
    amount     NUMERIC(15,2)  NOT NULL CHECK (amount > 0),
    status     VARCHAR(20)    NOT NULL DEFAULT 'active'
                   CHECK (status IN ('active', 'released', 'charged', 'expired')),
    expires_at TIMESTAMPTZ    NOT NULL,
    created_at TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

-- Outbox events — transactional outbox pattern for Kafka consistency
CREATE TABLE IF NOT EXISTS outbox_events (
    id             UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    aggregate_type VARCHAR(50)  NOT NULL,
    aggregate_id   UUID         NOT NULL,
    event_type     VARCHAR(100) NOT NULL,
    payload        JSONB        NOT NULL,
    processed      BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_wallets_user_id        ON wallets(user_id);
CREATE INDEX IF NOT EXISTS idx_transactions_wallet    ON transactions(wallet_id);
CREATE INDEX IF NOT EXISTS idx_transactions_idem_key  ON transactions(idempotency_key);
CREATE INDEX IF NOT EXISTS idx_transactions_created   ON transactions(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_holds_auction          ON bid_holds(auction_id, status);
CREATE INDEX IF NOT EXISTS idx_holds_bidder           ON bid_holds(bidder_id, status);
CREATE INDEX IF NOT EXISTS idx_holds_expires          ON bid_holds(expires_at) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_outbox_unprocessed     ON outbox_events(created_at) WHERE processed = FALSE;
