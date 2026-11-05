#!/usr/bin/env sh
set -eu
cd "$(dirname "$0")/.."

if ! command -v go >/dev/null 2>&1; then
  echo "Go is not installed or is not in PATH." >&2
  exit 1
fi

if [ ! -f .env ]; then
  cp .env.example .env
  echo "Created .env. Add GNANI_API_KEY to enable voice APIs."
fi

echo "Starting Vyapari Voice Copilot at http://localhost:8080"
exec go run ./cmd/server
