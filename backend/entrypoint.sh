#!/bin/bash
# Exit immediately if a command exits with a non-zero status
set -e

echo "========================================="
echo "🚀 TEREN Hotels Backend Startup"
echo "========================================="

echo "1. Running database migrations..."
./run-migrations

echo "2. Running database seeds..."
./seed

echo "3. Starting Go API server..."
exec ./api
