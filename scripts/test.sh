#!/usr/bin/env bash
#
# Run the Go test suite against a real MongoDB.
#
# Roughly half the persistence suite is integration tests that need a database.
# Without one they skip, and `go test ./...` still exits 0 — so the tests can rot
# for months behind a green check. This script makes the database non-optional:
# it reuses one if you already have it up, starts a throwaway one if you do not,
# and sets MONGO_TEST_REQUIRED so a database that goes missing mid-run fails
# loudly instead of skipping.
#
# Usage:
#   scripts/test.sh                      # whole suite
#   scripts/test.sh ./internal/repository/ -run TestSubscriberRepo
#
# Any arguments are passed through to `go test`; with none, it runs ./... .
#
# Honoured environment:
#   MONGO_TEST_URI   use this database, skip all provisioning
#   MONGO_TEST_PORT  host port for the throwaway container (default 27018)
#   MONGO_IMAGE      image for the throwaway container (default mongo:7)

set -euo pipefail

cd "$(dirname "$0")/.."

IMAGE="${MONGO_IMAGE:-mongo:7}"
PORT="${MONGO_TEST_PORT:-27018}"
CONTAINER="miab-test-mongo-$$"

# Port 27017 is where docker-compose (`task dev`) publishes Mongo, so an existing
# dev stack gets reused rather than duplicated.
DEV_PORT=27017

port_open() {
  (exec 3<>"/dev/tcp/127.0.0.1/$1") 2>/dev/null
}

log() { printf '\033[0;36m==>\033[0m %s\n' "$*"; }

if [[ -n "${MONGO_TEST_URI:-}" ]]; then
  log "Using MONGO_TEST_URI=$MONGO_TEST_URI"
elif port_open "$DEV_PORT"; then
  export MONGO_TEST_URI="mongodb://localhost:$DEV_PORT"
  log "Reusing the MongoDB already listening on $DEV_PORT"
else
  if ! command -v docker >/dev/null 2>&1; then
    echo "error: no MongoDB on port $DEV_PORT and docker is not installed." >&2
    echo "       Start one with 'task dev', or point MONGO_TEST_URI at your own." >&2
    exit 1
  fi

  log "No MongoDB found — starting a throwaway $IMAGE on port $PORT"
  docker run -d --rm --name "$CONTAINER" -p "$PORT:27017" "$IMAGE" >/dev/null
  # --rm alone does not fire on Ctrl-C, so remove it explicitly on any exit.
  trap 'log "Removing $CONTAINER"; docker rm -f "$CONTAINER" >/dev/null 2>&1 || true' EXIT

  log "Waiting for it to accept connections"
  for _ in $(seq 1 30); do
    if docker exec "$CONTAINER" mongosh --quiet --eval 'db.adminCommand({ping:1})' >/dev/null 2>&1; then
      break
    fi
    sleep 1
  done
  if ! docker exec "$CONTAINER" mongosh --quiet --eval 'db.adminCommand({ping:1})' >/dev/null 2>&1; then
    echo "error: MongoDB did not become ready within 30s" >&2
    docker logs "$CONTAINER" >&2 || true
    exit 1
  fi

  export MONGO_TEST_URI="mongodb://localhost:$PORT"
fi

# Turn "database missing" from a skip into a failure: we just guaranteed one.
export MONGO_TEST_REQUIRED=1

if [[ $# -gt 0 ]]; then
  log "go test $*"
  go test "$@"
else
  log "go test ./..."
  go test ./...
fi
