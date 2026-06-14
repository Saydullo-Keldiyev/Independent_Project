CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS categories (
    id         UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    name       VARCHAR(100) UNIQUE NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS auctions (
    id             UUID           PRIMARY KEY DEFAULT uuid_generate_v4(),
    seller_id      UUID           NOT NULL,
    title          VARCHAR(255)   NOT NULL,
    description    TEXT,
    category_id    UUID           REFERENCES categories(id),
    starting_price NUMERIC(15,2)  NOT NULL CHECK (starting_price >= 0),
    reserve_price  NUMERIC(15,2)  NOT NULL DEFAULT 0,
    current_price  NUMERIC(15,2)  NOT NULL DEFAULT 0,
    state          VARCHAR(20)    NOT NULL DEFAULT 'draft'
                       CHECK (state IN ('draft','scheduled','active','ending','ended','archived')),
    start_time     TIMESTAMPTZ    NOT NULL,
    end_time       TIMESTAMPTZ    NOT NULL,
    winner_id      UUID,
    total_bids     INT            NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    deleted_at     TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS auction_images (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    auction_id UUID NOT NULL REFERENCES auctions(id) ON DELETE CASCADE,
    image_url  TEXT NOT NULL,
    sort_order INT  NOT NULL DEFAULT 0
);

-- Bids table (shared with bid-service — same DB or cross-service query)
CREATE TABLE IF NOT EXISTS bids (
    id         UUID           PRIMARY KEY DEFAULT uuid_generate_v4(),
    auction_id UUID           NOT NULL REFERENCES auctions(id) ON DELETE CASCADE,
    user_id    UUID           NOT NULL,
    amount     NUMERIC(15,2)  NOT NULL CHECK (amount > 0),
    created_at TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_auctions_state      ON auctions(state);
CREATE INDEX IF NOT EXISTS idx_auctions_end_time   ON auctions(end_time) WHERE state = 'active';
CREATE INDEX IF NOT EXISTS idx_auctions_start_time ON auctions(start_time) WHERE state = 'scheduled';
CREATE INDEX IF NOT EXISTS idx_auctions_seller     ON auctions(seller_id);
CREATE INDEX IF NOT EXISTS idx_auctions_category   ON auctions(category_id);
CREATE INDEX IF NOT EXISTS idx_bids_auction        ON bids(auction_id);
CREATE INDEX IF NOT EXISTS idx_bids_amount         ON bids(auction_id, amount DESC);

-- Seed categories
INSERT INTO categories (name) VALUES
    ('Electronics'), ('Vehicles'), ('Art'), ('Collectibles'),
    ('Fashion'), ('Home & Garden'), ('Sports'), ('Other')
ON CONFLICT (name) DO NOTHING;
