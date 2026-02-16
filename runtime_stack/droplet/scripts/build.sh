#!/bin/bash

set -euo pipefail

BINDIR=./bin
MAINDIR=./cmd/droplet
BINNAME=droplet
VERSION_FILE="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)/VERSION"

if [[ -n "${BUILD_VERSION:-}" ]]; then
  VERSION_VALUE="${BUILD_VERSION}"
elif [[ -f "${VERSION_FILE}" ]]; then
  VERSION_VALUE="$(tr -d '[:space:]' < "${VERSION_FILE}")"
else
  VERSION_VALUE="dev"
fi

BUILD_VERSION="${VERSION_VALUE}"
BUILD_COMMIT="${BUILD_COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo unknown)}"
BUILD_DATE="${BUILD_DATE:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}"
LDFLAGS="-X droplet/internal/buildinfo.Version=${BUILD_VERSION} -X droplet/internal/buildinfo.Commit=${BUILD_COMMIT} -X droplet/internal/buildinfo.BuiltAt=${BUILD_DATE}"

go build -ldflags "${LDFLAGS}" -o $BINDIR/$BINNAME $MAINDIR
