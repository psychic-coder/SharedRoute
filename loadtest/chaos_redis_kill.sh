#!/bin/bash
set -euo pipefail

# Always run from the loadtest/ directory so k6 can find the script file.
cd "$(dirname "$0")"

echo "Starting optimized load test in background..."
k6 run k6_optimized.js --summary-export=chaos_results.json &
K6_PID=$!

echo "Waiting 20 seconds before chaos event..."
sleep 20

echo "🚨 KILLING REDIS NOW 🚨"
docker kill deploy-redis-1 2>/dev/null \
  || docker kill shardroute-redis-1 2>/dev/null \
  || docker kill ratelimitter-redis-1 2>/dev/null \
  || { echo "ERROR: Could not find Redis container to kill"; exit 1; }

echo "Waiting 10 seconds (outage window)..."
sleep 10

echo "✅ RESTARTING REDIS NOW ✅"
docker start deploy-redis-1 2>/dev/null \
  || docker start shardroute-redis-1 2>/dev/null \
  || docker start ratelimitter-redis-1 2>/dev/null \
  || { echo "ERROR: Could not restart Redis container"; exit 1; }

echo "Waiting for load test to finish..."
wait $K6_PID

echo "Load test complete. Results written to chaos_results.json"
