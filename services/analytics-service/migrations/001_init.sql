-- Analytics Service — Initial Schema
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Daily aggregated platform metrics
CREATE TABLE IF NOT EXISTS daily_metrics (
    id                  UUID           PRIMARY KEY DEFAULT uuid_generate_v4(),
    metric_date         DATE           UNIQUE NOT NULL,
    total_revenue       NUMERIC(15,2)  NOT NULL DEFAULT 0,
    total_bids          BIGINT         NOT NULL DEFAULT 0,
    total_auctions      BIGINT         NOT NULL DEFAULT 0,
    active_users        BIGINT         NOT NULL DEFAULT 0,
    new_users           BIGINT         NOT NULL DEFAULT 0,
    completed_auctions  BIGINT         NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

-- Per-auction metrics
CREATE TABLE IF NOT EXISTS auction_metrics (
    auction_id     UUID           PRIMARY KEY,
    total_bids     BIGINT         NOT NULL DEFAULT 0,
    highest_bid    NUMERIC(15,2)  NOT NULL DEFAULT 0,
    unique_bidders BIGINT         NOT NULL DEFAULT 0,
    watch_count    BIGINT         NOT NULL DEFAULT 0,
    updated_at     TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

-- Revenue tracking per settlement
CREATE TABLE IF NOT EXISTS revenue_metrics (
    id            UUID           PRIMARY KEY DEFAULT uuid_generate_v4(),
    seller_id     UUID           NOT NULL,
    auction_id    UUID           NOT NULL,
    gross_revenue NUMERIC(15,2)  NOT NULL,
    platform_fee  NUMERIC(15,2)  NOT NULL,
    net_revenue   NUMERIC(15,2)  NOT NULL,
    created_at    TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_daily_metrics_date     ON daily_metrics(metric_date DESC);
CREATE INDEX IF NOT EXISTS idx_revenue_seller         ON revenue_metrics(seller_id);
CREATE INDEX IF NOT EXISTS idx_revenue_created        ON revenue_metrics(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_auction_metrics_bids   ON auction_metrics(total_bids DESC);
