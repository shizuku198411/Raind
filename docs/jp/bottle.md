# Raind - Bottle
Bottleは複数のコンテナを1つのグループとして管理/操作を行うオーケストレーションです。

## 定義ファイル
Bottle作成には、定義ファイル`<any-filename>.yaml`を作成します。

```　yaml
bottle:
  name: wordpress   # bottle名

services:
  client:           # service#1
    image: alpine   # image
    tty: true       # TTY接続
    depends_on:     # 依存関係
      - wp
  wp:               # service#2
    image: wordpress
    env:            # 環境変数
      - WORDPRESS_DB_HOST=db:3306
      - WORDPRESS_DB_USER=wordpress
      - WORDPRESS_DB_PASSWORD=wordpress
      - WORDPRESS_DB_NAME=wordpress
    ports:         # ポートフォワード
      - "11240:80"
    depends_on:
      - db
  db:              # service#3
    image: mysql
    env:
      - MYSQL_ROOT_PASSWORD=wordpress
      - MYSQL_DATABASE=wordpress
      - MYSQL_USER=wordpress
      - MYSQL_PASSWORD=wordpress
    mount:         # マウント
      - "/mnt/db:/var/lib/mysql"

policies:
  - type: east-west                 # ポリシータイプ
    source: wp                      # 送信元Service
    destination: db                 # 宛先Service
    protocol: tcp                   # プロトコル
    dest_port: 3306                 # 宛先ポート
    comment: "wp -> db 3306/tcp"    # コメント

  - type: east-west
    source: client
    destination: wp
    protocol: tcp
    dest_port: 80
    comment: "client -> wp 80/tcp"
```

Raindでは、**コンテナ間通信はデフォルトで拒否**されます。
そのため、`policies:`にて許可する必要がある通信を指定します。

## Bottle作成
作成した定義ファイルからBottleを作成します。

```
$ raind bottle create -f /path/to/bottle.yaml
bottle: wordpress created
```

## Bottle確認
作成したBottleを確認します。

```
$ raind bottle ls
BOTTLE ID     BOTTLE NAME  SERVICES  STATUS
01kgv7wn56v6  wordpress    3         created
```

Bottleの詳細を確認するには、`show`サブコマンドを利用します。

```
$ raind bottle show wordpress
BOTTLE ID    01kgv7wn56v6
BOTTLE NAME  wordpress
CREATED AT   2026-02-07T14:06:14.976263167+09:00
START ORDER  db, wp, client

SERVICES
CONTAINER ID  IMAGE             COMMAND                  CREATED        STATUS   PORTS                  NAME
01kgv7wwd2rz  alpine:latest     "/bin/sh"                1 minutes ago  created                         wordpress-client
01kgv7wng48d  mysql:latest      "docker-entrypoint.sh…"  1 minutes ago  created                         wordpress-db
01kgv7wr11sr  wordpress:latest  "docker-entrypoint.sh…"  1 minutes ago  created  0.0.0.0:11240->80/tcp  wordpress-wp

SERVICE [1]   client
CONTAINER ID  01kgv7wwd2rz
IMAGE         alpine:latest
COMMAND       /bin/sh
ENV           -
PORTS         -
MOUNT         -
NETWORK       raind01kgv7wn56
TTY           true
DEPENDS ON    wp

SERVICE [2]   db
CONTAINER ID  01kgv7wng48d
IMAGE         mysql:latest
COMMAND       docker-entrypoint.sh mysqld
ENV           MYSQL_ROOT_PASSWORD=wordpress, MYSQL_DATABASE=wordpress, MYSQL_USER=wordpress, MYSQL_PASSWORD=wordpress
PORTS         -
MOUNT         /mnt/db:/var/lib/mysql
NETWORK       raind01kgv7wn56
TTY           false
DEPENDS ON    -

SERVICE [3]   wp
CONTAINER ID  01kgv7wr11sr
IMAGE         wordpress:latest
COMMAND       docker-entrypoint.sh apache2-foreground
ENV           WORDPRESS_DB_HOST=db:3306, WORDPRESS_DB_USER=wordpress, WORDPRESS_DB_PASSWORD=wordpress, WORDPRESS_DB_NAME=wordpress
PORTS         11240:80
MOUNT         -
NETWORK       raind01kgv7wn56
TTY           false
DEPENDS ON    db

POLICIES
ID                          TYPE       SOURCE  DESTINATION  PROTOCOL  DPORT  COMMENT
01kgv7wn56v60a736vq2b64spa  east-west  wp      db           tcp       3306   wp -> db 3306/tcp
01kgv7wn5br4r3q38ysebzwa0h  east-west  client  wp           tcp       80     client -> wp 80/tcp
```

## Bottle起動
作成時点では起動していないため、`start`サブコマンドで起動します。

```
$ raind bottle start wordpress
bottle: wordpress started

$ raind bottle ls
BOTTLE ID     BOTTLE NAME  SERVICES  STATUS
01kgv7wn56v6  wordpress    3         running
```

## Bottle停止および削除
Bottleの停止は`stop`サブコマンド、削除は`rm`サブコマンドで行います。

```
$ raind bottle stop wordpress
bottle: wordpress stopped

$ raind bottle rm wordpress
bottle: wordpress deleted
```
