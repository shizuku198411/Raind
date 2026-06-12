# raind container runtime

<p>
  <img src="./assets/raind_icon.png" alt="Project Icon" width="150">
</p>

raind is an experimental container runtime stack written in Go. It is split into small components so the user-facing CLI can stay unprivileged while the runtime daemons keep the root-only operations isolated.

## Components

- `raind`: CLI for images, containers, networks, resources, policies, logs, and bottles.
- `condenser`: root daemon that exposes the management API and coordinates runtime operations.
- `droplet`: low-level runtime component for container lifecycle operations.
- `condenser-hook-agent`: hook-side helper used by condenser and droplet workflows.
- `raind-ui-gateway`: gateway process for UI access.

## Highlights

- Docker-style image pull flow with manifest and layer progress reporting.
- mTLS between CLI and daemon components.
- Non-root `raind` CLI access through the `raind` Unix group.
- Root-only daemon operations are handled by `condenser`, `droplet`, and the UI gateway service.
- Runtime security policy management for east-west and north-south traffic control.
- Workshop-based test and manual verification environment.
- Container, image, network, pod, ReplicaSet, service, policy, bottle, and netflow log command groups.

## Pre-Setup

### Packages

raind uses the following packages on the host:

- `go`
- `ulogd2`
- `ulogd2-json`

Install them for your environment. On Ubuntu, for example:

```sh
sudo apt update
sudo apt install -y ulogd2 ulogd2-json
```

Go can be installed with your preferred toolchain manager, distribution package, or Workshop SDK.

### Enable Packet Forwarding

Enable IPv4 packet forwarding so containers can communicate with external networks:

```sh
cat /proc/sys/net/ipv4/ip_forward
# 0 = disabled, 1 = enabled

sudo sysctl -w net.ipv4.ip_forward=1
```

To enable it permanently, add or uncomment this line in `/etc/sysctl.conf`:

```conf
net.ipv4.ip_forward = 1
```

Apply the setting:

```sh
sudo sysctl -p
```

### Configure Log Forwarding

raind reads raw NFLOG records from ulogd and writes enriched runtime logs under `/var/log/raind`.

Create the ulog output directory:

```sh
sudo mkdir -p /var/log/ulog
```

Edit `/etc/ulogd.conf`. The plugin path depends on the host architecture, so check the multiarch directory first:

```sh
dpkg-architecture -qDEB_HOST_MULTIARCH
```

Enable these plugins in the `PLUGIN OPTIONS` section, replacing `aarch64-linux-gnu` with your host multiarch directory when needed:

```conf
plugin="/usr/lib/aarch64-linux-gnu/ulogd/ulogd_inppkt_NFLOG.so"
plugin="/usr/lib/aarch64-linux-gnu/ulogd/ulogd_filter_IFINDEX.so"
plugin="/usr/lib/aarch64-linux-gnu/ulogd/ulogd_filter_IP2STR.so"
plugin="/usr/lib/aarch64-linux-gnu/ulogd/ulogd_filter_PRINTPKT.so"
plugin="/usr/lib/aarch64-linux-gnu/ulogd/ulogd_raw2packet_BASE.so"
plugin="/usr/lib/aarch64-linux-gnu/ulogd/ulogd_output_JSON.so"
```

Define the raind NFLOG stacks:

```conf
stack=log10:NFLOG,base:BASE,ifi:IFINDEX,ip2str:IP2STR,print:PRINTPKT,json:JSON
stack=log11:NFLOG,base:BASE,ifi:IFINDEX,ip2str:IP2STR,print:PRINTPKT,json:JSON
stack=log12:NFLOG,base:BASE,ifi:IFINDEX,ip2str:IP2STR,print:PRINTPKT,json:JSON
```

Add the instances:

```conf
[log10]
group=10

[log11]
group=11

[log12]
group=12

[base]
[ifi]
[ip2str]
[print]

[json]
file="/var/log/ulog/raind.jsonl"
sync=1
```

Restart ulogd:

```sh
sudo systemctl restart ulogd
sudo systemctl status ulogd --no-pager
```

## Build

Download Go modules and build on the host:

```sh
./scripts/build.sh bootstrap
./scripts/build.sh build
```

Or build inside Workshop:

```sh
workshop run raind-dev -- bootstrap
workshop run raind-dev -- build
```

Built binaries are written to `bin/`:

- `bin/raind`
- `bin/condenser`
- `bin/condenser-hook-agent`
- `bin/droplet`
- `bin/raind-ui-gateway`

## Install Locally

Install the built binaries to `/usr/local/bin`:

```sh
sudo ./scripts/build.sh install
```

Create and start the condenser daemon service:

```sh
sudo ./scripts/build.sh enable-service
```

Optionally create and start the UI gateway service:

```sh
sudo ./scripts/build.sh enable-ui-gateway-service
```

Add your user to the `raind` group so the CLI can read the client certificate without running as root:

```sh
sudo usermod -aG raind "$USER"
```

Log out and back in, or start a new group session:

```sh
newgrp raind
```

You can also build, install, and enable the main condenser service in one command:

```sh
sudo ./scripts/build.sh all
```

Enable the UI gateway separately if needed:

```sh
sudo ./scripts/build.sh enable-ui-gateway-service
```

## First Checks

```sh
raind --version
raind image ls
raind container ls
raind network ls
```

Run a container and verify port forwarding:

```sh
raind container run -p 9988:80 nginx:latest
raind container ls
```

Generate external traffic and check enriched netflow logs:

```sh
raind container run -t --rm alpine:latest
# inside the container:
ping 1.1.1.1
exit
```

Then inspect the enriched log output:

```sh
sudo cat /var/log/raind/raind_netflow.jsonl | jq .
raind logs netflow --line 20
```

The CLI should be run as a non-root user in the `raind` group. If certificate paths need to be overridden, use:

- `RAIND_CA_CERT`
- `RAIND_CLIENT_CERT`
- `RAIND_CLIENT_KEY`

## Documentation

- [Testing with Workshop](docs/testing.md)
- [Command Reference](docs/commands.md)
- [Usage Examples](docs/examples.md)
