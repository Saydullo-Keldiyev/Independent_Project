#!/bin/bash
# Run database migrations for all services

set -e

echo "Running migrations for user-service..."
migrate -path services/user-service/migrations -database "$USER_DB_URL" up

echo "Running migrations for auction-service..."
migrate -path services/auction-service/migrations -database "$AUCTION_DB_URL" up

echo "Running migrations for bid-service..."
migrate -path services/bid-service/migrations -database "$BID_DB_URL" up

echo "All migrations completed."
