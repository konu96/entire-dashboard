#!/bin/bash
set -e

REPO="${1:-}"
PORT="${2:-8080}"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "==> Building backend..."
cd "$SCRIPT_DIR/backend"
go build -buildvcs=false -o entire-dashboard .

echo "==> Building frontend..."
cd "$SCRIPT_DIR/frontend"
npm run build --silent 2>/dev/null

echo "==> Starting Entire Dashboard on http://localhost:$PORT"
cd "$SCRIPT_DIR/backend"
if [ -n "$REPO" ]; then
  echo "    Initial repo: $REPO"
  ./entire-dashboard --repo "$REPO" --port "$PORT"
else
  echo "    No initial repo — add repos from the UI"
  ./entire-dashboard --port "$PORT"
fi
