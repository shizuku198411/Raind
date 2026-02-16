#!/bin/bash

CID=111111
ROOTFS=/etc/raind/container/$CID/merged
CWD=/
#CMD='/usr/bin/python3 -m http.server 8777'
CMD='/bin/sh -c "echo hello world!; sleep 60"'
#CMD='/bin/sh'
HOSTNAME=$CID

HOST_IF_NAME=wlan0
BR_IF_NAME=raind0
IF_NAME=eth0
IF_ADDR=10.166.0.100/24
IF_GW=10.166.0.254
DNS=8.8.8.8

IMAGE_LAYER=/etc/raind/image/layers/alpine/latest/rootfs
UPPER_DIR=/etc/raind/container/$CID/diff
WORK_DIR=/etc/raind/container/$CID/work

OUTDIR=/etc/raind/container/$CID


./bin/droplet spec \
  --rootfs "$ROOTFS" \
  --cwd "$CWD" \
  --command "$CMD" \
  --ns "mount" --ns "network" --ns "uts" --ns "pid" --ns "ipc" --ns "user" --ns "cgroup" \
  --hostname "$HOSTNAME" \
  --host_if_name "$HOST_IF_NAME" --bridge_if_name "$BR_IF_NAME" --if_name "$IF_NAME" --if_addr "$IF_ADDR" --if_gateway "$IF_GW" --dns "$DNS" \
  --image_layer "$IMAGE_LAYER" --upper_dir "$UPPER_DIR" --work_dir "$WORK_DIR" \
  --hook-create-runtime "/bin/sh,-c,cat > /tmp/create-runtime_state.json" \
  --hook-create-container "/bin/sh,-c,cat > /tmp/create-container_state.json" \
  --hook-start-container "/bin/sh,-c,cat > /tmp/start-container_state.json" \
  --hook-poststart "/bin/sh,-c,cat > /tmp/poststart_state.json" \
  --hook-poststop "/bin/sh,-c,cat > /tmp/poststop_state.json" \
  --output "$OUTDIR"