#!/bin/bash
# Seed initial data into the database

set -e

echo "Seeding database..."
psql "$USER_DB_URL" -f scripts/seed_users.sql
echo "Seeding completed."
