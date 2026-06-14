-- Migration: 001_init.sql
-- Creates the initial schema for bid-service

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Auctions table (read replica for bid validation)
CREATE TABLE IF NOT EXISTS auctions (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title         VARCHAR(255)   NOT NULL,
    start_price   NUMERIC(15, 2) NOT NULL DEFAULT 0,
    current_price NUMERIC(15, 2) NOT NULL DEFAULT 0,
    status        VARCHAR(20)    NOT NULL DEFAULT 'pending'
                      CHECK (status IN ('pending', 'active', 'ended')),
    seller_id     UUID           NOT NULL,
    winner_id     UUID,
    start_at      TIMESTAMPTZ    NOT NULL,
    end_at        TIMESTAMPTZ    NOT NULL,
    created_at    TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

-- Bids table
CREATE TABLE IF NOT EXISTS bids (
    id         UUID           PRIMARY KEY DEFAULT uuid_generate_v4(),
    auction_id UUID           NOT NULL REFERENCES auctions(id) ON DELETE CASCADE,
    user_id    UUID           NOT NULL,
    amount     NUMERIC(15, 2) NOT NULL CHECK (amount > 0),
    created_at TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_bids_auction_id   ON bids(auction_id);
CREATE INDEX IF NOT EXISTS idx_bids_user_id      ON bids(user_id);
CREATE INDEX IF NOT EXISTS idx_bids_amount_desc  ON bids(auction_id, amount DESC);
CREATE INDEX IF NOT EXISTS idx_auctions_status   ON auctions(status);
