# Raindセットアップ
## 事前セットアップ
### パッケージ
Raindでは以下のパッケージを利用します。  
- `go`
- `ulogd2`
- `ulogd2-json`

実行環境に応じて、上記パッケージをインストールしてください。

### パケット転送の有効化
コンテナ内から外部に通信を行うために、ホスト側でパケット転送の有効化が必要になります。  
以下の手順で有効化をしてください。
```
# 現在の設定確認
cat /proc/sys/net/ipv4/ip_forward
# 結果が0=無効、1=有効

# 無効の場合、有効にする
# 一時的に有効にする場合
sudo sysctl -w net.ipv4.ip_forward=1

# 恒久的に有効にする場合
# /etc/sysctl.conf を開き、
#   net.ipv4.ip_forward = 1 
# を書き込む(コメントアウト)
# 書き込み後、内容反映
sudo sysctl -p. 
```

## ビルド&インストール
```
git clone --recurse-submodules https://github.com/shizuku198411/Raind.git
cd raind
make bootstrap
make build
sudo make install
sudo make enable-service
```

## 動作確認
```
# Nginxイメージの起動 (ホスト側9988で待ち受け)
$ raind container run -p 9988:80 nginx:latest

# コンテナ一覧確認
$ raind container ls
CONTAINER ID  IMAGE          COMMAND                  CREATED              STATUS   PORTS                 NAME
01kg2cnf0ytv  nginx:latest   "/docker-entrypoint.s…"  less than a minutes  running  0.0.0.0:9988->80/tcp  narrow-tangent-0103

# 必要に応じてブラウザからアクセス
```

## ログ出力設定
ログ出力を行う場合、以下の設定が必要です。

### ulogd.confの編集
`/etc/ulogd.conf`を以下の内容で編集します。

```
######################################################################
# PLUGIN OPTIONS
######################################################################
# OPTIONSにて、以下6つのプラグインをコメントアウトし有効化
plugin="/usr/lib/aarch64-linux-gnu/ulogd/ulogd_inppkt_NFLOG.so"
plugin="/usr/lib/aarch64-linux-gnu/ulogd/ulogd_filter_IFINDEX.so"
plugin="/usr/lib/aarch64-linux-gnu/ulogd/ulogd_filter_IP2STR.so"
plugin="/usr/lib/aarch64-linux-gnu/ulogd/ulogd_filter_PRINTPKT.so"
plugin="/usr/lib/aarch64-linux-gnu/ulogd/ulogd_raw2packet_BASE.so"
plugin="/usr/lib/aarch64-linux-gnu/ulogd/ulogd_output_JSON.so"

# Stackとして以下3つを定義
stack=log10:NFLOG,base:BASE,ifi:IFINDEX,ip2str:IP2STR,print:PRINTPKT,json:JSON
stack=log11:NFLOG,base:BASE,ifi:IFINDEX,ip2str:IP2STR,print:PRINTPKT,json:JSON
stack=log12:NFLOG,base:BASE,ifi:IFINDEX,ip2str:IP2STR,print:PRINTPKT,json:JSON

# 以下のインスタンスを定義
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
編集後、`ulogd`サービスを再起動し設定を反映します。

### ログ出力確認
Raindではデフォルトで外部通信に対するログ出力が有効になっているため、alpineイメージ等でコンテナを起動し外部通信を発生させます。

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

整形済みのログが `/var/log/raind/netflow.jsonl` に記録されていることを確認します。
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