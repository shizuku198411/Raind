#!/bin/bash
set -euo pipefail

#
# THIS SCRIPT REQUIRES RUNNING AS SUDO
#   sudo ./scripts/setup.sh
#

# == cgroup ==
# parent cgroup name for runtime
CONTAINER_ID=111111

RUNTIME_ROOT="raind"

CG_MNT="/sys/fs/cgroup"
PARENT="${CG_MNT}/${RUNTIME_ROOT}"

echo "[*] initiate cgroup v2: parent=${PARENT}"

# 1) verify cgroup v2 is supported
if ! grep -qw "cgroup2" /proc/filesystems; then
    echo "ERROR: cgroup v2 is not supported by this kernel" >&2
    exit 1
fi

# 2) create parent directory
if [[ ! -d "${PARENT}" ]]; then
    echo "[*] mkdir ${PARENT}"
    mkdir -p "${PARENT}"
fi

# 3) enable cpu/memory controller
SUBTREE_CTL="${PARENT}/cgroup.subtree_control"
echo "[*] enable +cpu +memory on ${SUBTREE_CTL}"
echo "+cpu +memory" > "${SUBTREE_CTL}"

# 4) create container directory
mkdir -p "${PARENT}/${CONTAINER_ID}"
# ===========

echo "[*] setup completed"