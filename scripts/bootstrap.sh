#!/usr/bin/env bash
set -euo pipefail

need() {
  command -v "$1" >/dev/null 2>&1 || { echo "missing: $1"; exit 1; }
}

need go
need git
need install
need ulogd

echo "ok"
