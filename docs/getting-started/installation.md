# Installation
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

## Installation

```sh
sudo ./scripts/build.sh all
```

Add your user to the `raind` group so the CLI can read the client certificate without running as root:

```sh
sudo usermod -aG raind "$USER"
```

Log out and back in, or start a new group session:

```sh
newgrp raind
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
