#!/bin/bash
set -euo pipefail

#
# THIS SCRIPT REQUIRES RUNNING AS SUDO
#   sudo ./scripts/setup.sh
#

# == create bridge interface ==
BRIDGE_NAME="raind0"
BRIDGE_ADDR="10.166.0.254/24"

echo "[*] initiate bridge interface. name=${BRIDGE_NAME} addr=${BRIDGE_ADDR}"

# check if the bridge already exists
if ip link show "${BRIDGE_NAME}" > /dev/null 2>&1; then
    echo "[*] bridge ${BRIDGE_NAME} already exists"
else
    echo "[*] create bridge ${BRIDGE_NAME}"
    ip link add "${BRIDGE_NAME}" type bridge
fi

echo "[*] assign addr ${BRIDGE_ADDR}"
ip addr add "${BRIDGE_ADDR}" dev "${BRIDGE_NAME}"

echo "[*] set link ${BRIDGE_NAME} up"
ip link set "${BRIDGE_NAME}" up

## == masquerade ==
RAIND_SUBNET="10.166.0.0/16"
HOST_IF="wlan0"

echo "[*] masquerade container traffic with host address"
iptables -t nat -A POSTROUTING -s "${RAIND_SUBNET}" -o "${HOST_IF}" -j MASQUERADE

echo "[*] setup completed"