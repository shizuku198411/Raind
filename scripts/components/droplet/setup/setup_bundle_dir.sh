#!/bin/bash
set -euo pipefail

#
# THIS SCRIPT REQUIRES RUNNING AS SUDO
#   sudo ./scripts/setup_bundle_dir.sh
#

CONTAINER_ID=111111

RUNTIME_ROOT="/etc/raind/container"
CONTAINER_DIR="${RUNTIME_ROOT}/${CONTAINER_ID}"
UPPER_DIR="${CONTAINER_DIR}/diff"
WORK_DIR="${CONTAINER_DIR}/work"
MERGED_DIR="${CONTAINER_DIR}/merged"
ETC_DIR="${CONTAINER_DIR}/etc"

echo "[*] initiate bundle directory: container root path=${CONTAINER_DIR}"
mkdir -p "${UPPER_DIR}"
mkdir -p "${WORK_DIR}"
mkdir -p "${MERGED_DIR}"
mkdir -p "${ETC_DIR}"

echo "[*] create resolv.conf, hosts and hostname"
echo "nameserver 8.8.8.8" > "${ETC_DIR}/resolv.conf"
echo "127.0.0.1 localhost" > "${ETC_DIR}/hosts"
echo "${CONTAINER_ID}" > "${ETC_DIR}/hostname"

echo "[*] setup completed"