# Raind - Bottle
Bottle is an orchestration feature that manages and operates multiple containers as a single group.

## Definition File
To create a Bottle, prepare a definition file `<any-filename>.yaml`.

```yaml
bottle:
  name: wordpress   # bottle name

services:
  client:           # service#1
    image: alpine   # image
    tty: true       # TTY attach
    depends_on:     # dependencies
      - wp
  wp:               # service#2
    image: wordpress
    env:            # environment variables
      - WORDPRESS_DB_HOST=db:3306
      - WORDPRESS_DB_USER=wordpress
      - WORDPRESS_DB_PASSWORD=wordpress
      - WORDPRESS_DB_NAME=wordpress
    ports:         # port forward
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
    mount:         # mount
      - "/mnt/db:/var/lib/mysql"

policies:
  - type: east-west                 # policy type
    source: wp                      # source service
    destination: db                 # destination service
    protocol: tcp                   # protocol
    dest_port: 3306                 # destination port
    comment: "wp -> db 3306/tcp"    # comment

  - type: east-west
    source: client
    destination: wp
    protocol: tcp
    dest_port: 80
    comment: "client -> wp 80/tcp"
```

In Raind, **container-to-container traffic is denied by default**.  
Therefore, the required communications must be explicitly allowed under `policies:`.

## Create Bottle
Create a Bottle from the definition file.

```
$ raind bottle create -f /path/to/bottle.yaml
bottle: wordpress created
```

## Show Bottle
List Bottles.

```
$ raind bottle ls
BOTTLE ID     BOTTLE NAME  SERVICES  STATUS
01kgv7wn56v6  wordpress    3         created
```

To view details, use the `show` subcommand.

```
$ raind bottle show wordpress
BOTTLE ID    01kgv7wn56v6
BOTTLE NAME  wordpress
CREATED AT   2026-02-07T14:06:14.976263167+09:00
START ORDER  db, wp, client

SERVICES
CONTAINER ID  IMAGE             COMMAND                  CREATED        STATUS   PORTS                  NAME
01kgv7wwd2rz  alpine:latest     "/bin/sh"                1 minutes ago  created                         wordpress-client
01kgv7wng48d  mysql:latest      "docker-entrypoint.sh..."  1 minutes ago  created                         wordpress-db
01kgv7wr11sr  wordpress:latest  "docker-entrypoint.sh..."  1 minutes ago  created  0.0.0.0:11240->80/tcp  wordpress-wp

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

## Start Bottle
Bottles are not started immediately after creation, so use `start`.

```
$ raind bottle start wordpress
bottle: wordpress started

$ raind bottle ls
BOTTLE ID     BOTTLE NAME  SERVICES  STATUS
01kgv7wn56v6  wordpress    3         running
```

## Stop and Remove Bottle
Stop with `stop`, remove with `rm`.

```
$ raind bottle stop wordpress
bottle: wordpress stopped

$ raind bottle rm wordpress
bottle: wordpress deleted
```
