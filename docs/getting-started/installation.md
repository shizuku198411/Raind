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
sudo apt install -y ulogd2 ulogd2-json dpkg-dev
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

## Installation

Currently, Raind supports creating and installing `snap` packages locally, or system-wide installation via scripts.

### Snap Install

```bash
sudo snap install snapcraft

snapcraft pack --destructive-mode
sudo install ./raind_<version>_<arch>.snap --dangerous --classic
```

verify snap log
```bash
sudo snap logs raind.condenser -n 50

2026-06-16T13:31:20+09:00 systemd[1]: Started snap.raind.condenser.service - Service for snap application raind.condenser.
2026-06-16T13:31:34+09:00 raind.condenser[749955]: 2026/06/16 13:31:34 [*] management server listening on 127.0.0.1:7755
2026-06-16T13:31:34+09:00 raind.condenser[749955]: 2026/06/16 13:31:34 [*] ca server listening on 127.0.0.1:7757
2026-06-16T13:31:34+09:00 raind.condenser[749955]: 2026/06/16 13:31:34 [*] Container Monitoring Start
2026-06-16T13:31:34+09:00 raind.condenser[749955]: 2026/06/16 13:31:34 [*] pod controller start
2026-06-16T13:31:34+09:00 raind.condenser[749955]: 2026/06/16 13:31:34 [*] enrichement logger start
2026-06-16T13:31:34+09:00 raind.condenser[749955]: 2026/06/16 13:31:34 [*] ingress http gateway listening on :7780
2026-06-16T13:31:34+09:00 raind.condenser[749955]: 2026/06/16 13:31:34 [*] ingress https gateway listening on :7443
2026-06-16T13:31:34+09:00 raind.condenser[749955]: 2026/06/16 13:31:34 [*] hook server listening on :7756
2026-06-16T13:31:34+09:00 raind.condenser[749955]: 2026/06/16 13:31:34 [*] dns proxy listening
2026-06-16T13:31:34+09:00 raind.condenser[749955]: 2026/06/16 13:31:34 [*] service controller start
2026-06-16T13:31:34+09:00 raind.condenser[749955]: 2026/06/16 13:31:34 [*] dns proxy start udp listen=10.166.254.254:1053 upstreams=[8.8.8.8:53 1.1.1.1:53]
2026-06-16T13:31:34+09:00 raind.condenser[749955]: 2026/06/16 13:31:34 [*] dns proxy start tcp listen=10.166.254.254:1053 upstreams=[8.8.8.8:53 1.1.1.1:53]
```

### Scripts Install
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

verify systemctl log
```bash
sudo systemctl status raind-daemon.service 
● raind-daemon.service - Raind Condenser Daemon
     Loaded: loaded (/etc/systemd/system/raind-daemon.service; enabled; preset: enabled)
     Active: active (running) since Tue 2026-06-16 16:26:11 JST; 2s ago
   Main PID: 4747 (condenser)
      Tasks: 6 (limit: 9063)
     Memory: 3.8M (peak: 4.6M)
        CPU: 157ms
     CGroup: /system.slice/raind-daemon.service
             └─4747 /usr/local/bin/condenser

Jun 16 16:26:11 raind-dev-4ec1a7cf condenser[4747]: 2026/06/16 16:26:11 [*] swagger listening on :7758
Jun 16 16:26:11 raind-dev-4ec1a7cf condenser[4747]: 2026/06/16 16:26:11 [*] enrichement logger start
Jun 16 16:26:11 raind-dev-4ec1a7cf condenser[4747]: 2026/06/16 16:26:11 [*] service controller start
Jun 16 16:26:11 raind-dev-4ec1a7cf condenser[4747]: 2026/06/16 16:26:11 [*] ingress https gateway listening on :7443
Jun 16 16:26:11 raind-dev-4ec1a7cf condenser[4747]: 2026/06/16 16:26:11 [*] dns proxy listening
Jun 16 16:26:11 raind-dev-4ec1a7cf condenser[4747]: 2026/06/16 16:26:11 [*] pod controller start
Jun 16 16:26:11 raind-dev-4ec1a7cf condenser[4747]: 2026/06/16 16:26:11 [*] ingress http gateway listening on :7780
Jun 16 16:26:11 raind-dev-4ec1a7cf condenser[4747]: 2026/06/16 16:26:11 [*] dns proxy start udp listen=10.166.254.254:1053 upstreams=[8.8.8.8:53 1.1.1.1:53]
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


## Log Forwarding

Raind uses NFLOG + ulogd to collect raw packet logs and writes enriched runtime logs under `/var/log/raind`.

On condenser startup, Raind now tries to configure the required ulogd NFLOG stacks automatically. The automatic configuration is conservative:

- Raind only manages its marker blocks in `/etc/ulogd.conf`: `# BEGIN RAIND MANAGED ULOGD PLUGINS` / `# END RAIND MANAGED ULOGD PLUGINS` and `# BEGIN RAIND MANAGED NFLOG CONFIG` / `# END RAIND MANAGED NFLOG CONFIG`.
- Existing ulogd configuration outside those blocks is not modified.
- If ulogd is not installed, required plugins are missing, or ulogd cannot be restarted, condenser startup continues and prints a warning.
- The raw ulogd output remains `/var/log/ulog/raind.jsonl`.
- The enriched Raind netflow output remains `/var/log/raind/raind_netflow.jsonl`.

You can disable automatic ulogd configuration if you manage ulogd yourself:

```sh
RAIND_AUTO_CONFIG_ULOGD=false condenser
```

For systemd installs, add the environment variable to the condenser service override if needed.

Inspect the enriched log output:

```sh
sudo cat /var/log/raind/raind_netflow.jsonl | jq .
raind logs netflow --line 20
```

### Troubleshooting: manual ulogd configuration

If automatic configuration is disabled or skipped, configure ulogd manually.

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

Define the Raind NFLOG stacks near the other `stack=` declarations, before the input/output instance sections. Using Raind-prefixed instance names avoids collisions with existing ulogd instances:

```conf
stack=raind_log10:NFLOG,raind_base:BASE,raind_ifi:IFINDEX,raind_ip2str:IP2STR,raind_print:PRINTPKT,raind_json:JSON
stack=raind_log11:NFLOG,raind_base:BASE,raind_ifi:IFINDEX,raind_ip2str:IP2STR,raind_print:PRINTPKT,raind_json:JSON
stack=raind_log12:NFLOG,raind_base:BASE,raind_ifi:IFINDEX,raind_ip2str:IP2STR,raind_print:PRINTPKT,raind_json:JSON
```

Add the instances after the stack declarations:

```conf
[raind_log10]
group=10

[raind_log11]
group=11

[raind_log12]
group=12

[raind_base]
[raind_ifi]
[raind_ip2str]
[raind_print]

[raind_json]
file="/var/log/ulog/raind.jsonl"
sync=1
```

Restart ulogd:

```sh
sudo systemctl restart ulogd
sudo systemctl status ulogd --no-pager
```
