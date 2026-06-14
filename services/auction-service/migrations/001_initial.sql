-- Auction Service — canonical schema (shared PostgreSQL with bid-service)
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS categories (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(100) UNIQUE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Fresh install
CREATE TABLE IF NOT EXISTS auctions (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    seller_id      UUID NOT NULL,
    title          VARCHAR(255) NOT NULL,
    description    TEXT,
    category_id    UUID REFERENCES categories(id),
    starting_price NUMERIC(15,2) NOT NULL DEFAULT 0,
    reserve_price  NUMERIC(15,2),
    current_price  NUMERIC(15,2) NOT NULL DEFAULT 0,
    state          VARCHAR(20) NOT NULL DEFAULT 'draft',
    status         VARCHAR(20) NOT NULL DEFAULT 'pending',
    start_time     TIMESTAMPTZ,
    end_time       TIMESTAMPTZ,
    winner_id      UUID,
    total_bids     INT NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at     TIMESTAMPTZ,
    start_price    NUMERIC(15,2) NOT NULL DEFAULT 0,
    start_at       TIMESTAMPTZ,
    end_at         TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS auction_images (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    auction_id UUID NOT NULL REFERENCES auctions(id) ON DELETE CASCADE,
    image_url  TEXT NOT NULL,
    sort_order INT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_auction_state ON auctions(state);
CREATE INDEX IF NOT EXISTS idx_auction_end_time ON auctions(end_time);
CREATE INDEX IF NOT EXISTS idx_auction_seller ON auctions(seller_id);
CREATE INDEX IF NOT EXISTS idx_auction_category ON auctions(category_id);

-- Legacy bid-service columns (add if missing)
ALTER TABLE auctions ADD COLUMN IF NOT EXISTS state VARCHAR(20);
ALTER TABLE auctions ADD COLUMN IF NOT EXISTS description TEXT;
ALTER TABLE auctions ADD COLUMN IF NOT EXISTS category_id UUID;
ALTER TABLE auctions ADD COLUMN IF NOT EXISTS reserve_price NUMERIC(15,2);
ALTER TABLE auctions ADD COLUMN IF NOT EXISTS total_bids INT NOT NULL DEFAULT 0;
ALTER TABLE auctions ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
ALTER TABLE auctions ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE auctions ADD COLUMN IF NOT EXISTS starting_price NUMERIC(15,2);

UPDATE auctions SET state = 'active', status = 'active' WHERE state IS NULL AND status = 'active';
UPDATE auctions SET state = 'scheduled', status = 'pending' WHERE state IS NULL AND status = 'pending';
UPDATE auctions SET state = 'ended', status = 'ended' WHERE state IS NULL;
UPDATE auctions SET starting_price = COALESCE(starting_price, start_price, current_price, 0);
UPDATE auctions SET start_time = COALESCE(start_time, start_at);
UPDATE auctions SET end_time = COALESCE(end_time, end_at);
