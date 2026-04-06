#!/bin/sh
set -e

cleanup() {
  echo "Shutting down..."
  kill "$API_PID" "$WORKER_PID" "$WS_PID" 2>/dev/null || true
  wait "$API_PID" "$WORKER_PID" "$WS_PID" 2>/dev/null || true
  mongod --dbpath /data/db --shutdown 2>/dev/null || true
  redis-cli shutdown nosave 2>/dev/null || true
  echo "All processes stopped"
  exit 0
}

trap cleanup TERM INT

# Start MongoDB
echo "Starting MongoDB..."
mongod --dbpath /data/db --bind_ip 127.0.0.1 --quiet --logpath /var/log/miab/mongod.log --fork

# Wait for MongoDB to accept connections
echo "Waiting for MongoDB..."
for i in $(seq 1 30); do
  if mongosh --quiet --eval "db.runCommand({ping:1})" >/dev/null 2>&1; then
    echo "MongoDB ready"
    break
  fi
  if [ "$i" -eq 30 ]; then
    echo "MongoDB failed to start"
    cat /var/log/miab/mongod.log
    exit 1
  fi
  sleep 1
done

# Start Redis
echo "Starting Redis..."
redis-server \
  --bind 127.0.0.1 \
  --maxmemory 512mb \
  --maxmemory-policy allkeys-lru \
  --daemonize yes \
  --loglevel warning

# Wait for Redis
for i in $(seq 1 10); do
  if redis-cli ping >/dev/null 2>&1; then
    echo "Redis ready"
    break
  fi
  if [ "$i" -eq 10 ]; then
    echo "Redis failed to start"
    exit 1
  fi
  sleep 1
done

# Start worker (background)
echo "Starting worker..."
/worker &
WORKER_PID=$!

# Start WebSocket server (background, only if SUBSCRIBER_HMAC_SECRET is set)
if [ -n "$SUBSCRIBER_HMAC_SECRET" ]; then
  echo "Starting WebSocket server..."
  /ws &
  WS_PID=$!
else
  echo "SUBSCRIBER_HMAC_SECRET not set, skipping WebSocket server"
  WS_PID=
fi

# Start API server (foreground)
echo "Starting API server..."
/api &
API_PID=$!

wait "$API_PID"
