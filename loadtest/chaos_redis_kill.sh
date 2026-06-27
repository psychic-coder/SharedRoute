#!/bin/bash

echo "Starting load test in background..."
k6 run k6_optimized.js &
K6_PID=$!

echo "Waiting 20 seconds before Chaos event..."
sleep 20

echo "🚨 KILLING REDIS NOW 🚨"
docker kill deploy-redis-1 || docker kill ratelimitter-redis-1 || fly machine stop -a shardroute-redis

echo "Waiting 10 seconds (outage window)..."
sleep 10

echo "✅ RESTARTING REDIS NOW ✅"
docker start deploy-redis-1 || docker start ratelimitter-redis-1 || fly machine start -a shardroute-redis

echo "Waiting for load test to finish..."
wait $K6_PID

echo "Load test complete."
