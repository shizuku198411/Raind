#!/bin/bash

BINDIR=./bin
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
MAINDIR="${ROOT_DIR}/cmd/raind"
BINNAME=raind

mkdir -p "${ROOT_DIR}/${BINDIR}"
go build -o "${ROOT_DIR}/${BINDIR}/${BINNAME}" "${MAINDIR}"
sudo cp "${ROOT_DIR}/${BINDIR}/${BINNAME}" /usr/local/bin
