# Raind Install
## Pre-Setup
### Packages
Raind uses the following packages:
- `go`
- `ulogd2`
- `ulogd2-json`

Install these packages based on your environment.

### Enable Packet Forwarding
To allow containers to communicate externally, enable packet forwarding on the host.  
Follow the steps below.
```
# check current setting
cat /proc/sys/net/ipv4/ip_forward
# 0 = disabled, 1 = enabled

# enable temporarily
sudo sysctl -w net.ipv4.ip_forward=1

# enable permanently
# edit /etc/sysctl.conf and add or uncomment:
#   net.ipv4.ip_forward = 1
# then apply:
sudo sysctl -p.
```

## Build & Install
```
git clone --recurse-submodules https://github.com/shizuku198411/Raind.git
cd raind
make bootstrap
make build
sudo make install
sudo make enable-service
# or run install + service setup together
sudo make all
```

## Verification
```
# run Nginx image (listen on host port 9988)
$ raind container run -p 9988:80 nginx:latest

# list containers
$ raind container ls
CONTAINER ID  IMAGE          COMMAND                  CREATED              STATUS   PORTS                 NAME
01kg2cnf0ytv  nginx:latest   "/docker-entrypoint.s..."  less than a minutes  running  0.0.0.0:9988->80/tcp  narrow-tangent-0103

# access from a browser if needed
```

## Log Output Setup
To output logs, configure as follows.

### Edit ulogd.conf
Edit `/etc/ulogd.conf` as below.

```
######################################################################
# PLUGIN OPTIONS
######################################################################
# Enable the following 6 plugins in OPTIONS
plugin="/usr/lib/aarch64-linux-gnu/ulogd/ulogd_inppkt_NFLOG.so"
plugin="/usr/lib/aarch64-linux-gnu/ulogd/ulogd_filter_IFINDEX.so"
plugin="/usr/lib/aarch64-linux-gnu/ulogd/ulogd_filter_IP2STR.so"
plugin="/usr/lib/aarch64-linux-gnu/ulogd/ulogd_filter_PRINTPKT.so"
plugin="/usr/lib/aarch64-linux-gnu/ulogd/ulogd_raw2packet_BASE.so"
plugin="/usr/lib/aarch64-linux-gnu/ulogd/ulogd_output_JSON.so"

# Define the following 3 stacks
stack=log10:NFLOG,base:BASE,ifi:IFINDEX,ip2str:IP2STR,print:PRINTPKT,json:JSON
stack=log11:NFLOG,base:BASE,ifi:IFINDEX,ip2str:IP2STR,print:PRINTPKT,json:JSON
stack=log12:NFLOG,base:BASE,ifi:IFINDEX,ip2str:IP2STR,print:PRINTPKT,json:JSON

# Define the following instances
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

After editing, restart the `ulogd` service to apply settings.

### Verify Log Output
By default, Raind enables logs for external communication, so start a container (e.g., alpine) and generate traffic.

```
$ raind container run -t --rm alpine
container: 01kg2d0y53va started
/ # ping 1.1.1.1
PING 1.1.1.1 (1.1.1.1): 56 data bytes
64 bytes from 1.1.1.1: seq=0 ttl=57 time=6.636 ms
64 bytes from 1.1.1.1: seq=1 ttl=57 time=8.801 ms
^C
--- 1.1.1.1 ping statistics ---
2 packets transmitted, 2 packets received, 0% packet loss
round-trip min/avg/max = 6.636/7.718/8.801 ms
/ # exit
```

Verify that formatted logs are written to `/var/log/raind/netflow.jsonl`.
```
$ cat /var/log/raind/netflow.jsonl | jq .

{
  "generated_ts": "2026-01-28T22:35:07.898647+0900",
  "received_ts": "2026-01-28T22:35:09.147661127+09:00",
  "policy": {
    "source": "predefined"
  },
  "kind": "north-south",
  "verdict": "allow",
  "proto": "ICMP",
  "src": {
    "kind": "container",
    "ip": "10.166.0.5",
    "container_id": "01kg2d0y53va",
    "container_name": "round-tangent-2218",
    "veth": "rd_01kg2d0y53va"
  },
  "dst": {
    "kind": "external",
    "ip": "1.1.1.1"
  },
  "icmp": {
    "code": 0,
    "type": 8
  },
  "rule_hint": "RAIND-NS-ALLOW,id=predefined",
  "raw_hash": "f5f3c079467bd37f5360a4bda2ac8b968b0648676844c2350cd3b6844f16564f"
}
```
