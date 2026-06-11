#!/bin/bash

BINDIR=./bin
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
MAINDIR="${ROOT_DIR}/cmd/condenser"
BINNAME=condenser

cd "${ROOT_DIR}"
swag init -g cmd/condenser/main.go

# condenser
mkdir -p "${ROOT_DIR}/${BINDIR}"
go build -o "${ROOT_DIR}/${BINDIR}/${BINNAME}" "${MAINDIR}"
sudo cp "${ROOT_DIR}/${BINDIR}/${BINNAME}" /usr/local/bin

HOOKMAINDIR="${ROOT_DIR}/cmd/condenser-hook"
HOOKBINNAME=condenser-hook-agent

# hook
go build -o "${ROOT_DIR}/${BINDIR}/${HOOKBINNAME}" "${HOOKMAINDIR}"
sudo cp "${ROOT_DIR}/${BINDIR}/${HOOKBINNAME}" /usr/local/bin
