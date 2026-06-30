#!/bin/bash
# Exit immediately if a command exits with a non-zero status
set -e

echo "========================================="
echo "🚀 TEREN Hotels Backend Startup"
echo "========================================="

echo "1. Running database migrations..."
./run-migrations

# Seed step is gated by APP_ENV + ENABLE_SEED.
#
# Why this exists: the seed suite was being re-executed on every Railway
# redeploy, producing duplicate guests and overlapping bookings (see
# Docs/fixes/2026-06-30_seed_idempotency_and_production_cleanup.md).
#
# Rules:
#   - APP_ENV=production → skip seeds (production data must not be touched).
#   - ENABLE_SEED=true    → opt-in for staging or one-shot manual re-seeds.
#   - any other case      → run seeds (default = development).
if [ "${APP_ENV:-development}" = "production" ] && [ "${ENABLE_SEED:-false}" != "true" ]; then
    echo "2. Skipping database seeds (APP_ENV=production, ENABLE_SEED!=true)."
else
    echo "2. Running database seeds..."
    ./seed
fi

echo "3. Starting Go API server..."
exec ./api
