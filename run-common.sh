#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="$ROOT_DIR/.env"

if [[ ! -f "$ENV_FILE" ]]; then
  printf 'Missing %s. Copy .env.example to .env and set the private values.\n' "$ENV_FILE" >&2
  exit 1
fi

run_service() {
  local service="$1"
  (cd "$ROOT_DIR" && docker compose up --build "$service")
}
