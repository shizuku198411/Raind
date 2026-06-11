#!/bin/bash

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "${ROOT_DIR}"

swag init -g cmd/condenser/main.go
sudo go run ./cmd/condenser/main.go
