#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

build_one() {
  local name="$1"
  echo "[build] ${name}"
  (cd "${ROOT_DIR}/components/${name}" && ./scripts/build.sh)
}

build_one droplet
build_one condenser
build_one raind-cli