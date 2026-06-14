# Raind - Bottle

A `Bottle` is a Raind-native orchestration format for managing multiple containers as a single group.

Unlike Kubernetes-style resources, Bottle manifests are Raind-specific. They are useful when you want a compact multi-container definition with dependency ordering, shared network setup, and explicit east-west policy rules.

## Supported Manifest

Bottle manifests do not use `apiVersion` / `kind`. The top-level fields are:

```yaml
bottle:
services:
policies:
```

## Supported Fields

### Bottle Metadata

| Field | Required | Description |
|---|---:|---|
| `bottle.name` | yes | Bottle name. |

### Services

`services` is a map whose keys are service names.

| Field | Required | Description |
|---|---:|---|
| `image` | yes | Container image. |
| `command` | no | Command array. |
| `env` | no | Environment variables as `KEY=value` strings. |
| `ports` | no | Port mappings such as `8080:80`. |
| `mount` | no | Mount strings such as `/host:/container[:options]`. |
| `device` / `devices` | no | Device mappings. Both names are supported and merged. |
| `capAdd` / `cap-add` | no | Capabilities to add. Both names are supported and merged. |
| `capDrop` / `cap-drop` | no | Capabilities to drop. Both names are supported and merged. |
| `network` | no | Explicit network name. |
| `tty` | no | Allocate a TTY. |
| `depends_on` | no | Service startup dependencies. |

### Policies

`policies` defines explicit communication rules between Bottle services.

| Field | Required | Description |
|---|---:|---|
| `type` | yes | Policy type, for example `east-west`. |
| `source` | yes | Source service name. |
| `destination` | yes | Destination service name. |
| `protocol` | no | Protocol such as `tcp`. |
| `dest_port` | no | Destination port. |
| `comment` | no | Human-readable comment. |

## Complete Example

```yaml
bottle:
  name: wordpress

services:
  client:
    image: alpine:latest
    command:
      - /bin/sh
    tty: true
    depends_on:
      - wp

  wp:
    image: wordpress:latest
    env:
      - WORDPRESS_DB_HOST=db:3306
      - WORDPRESS_DB_USER=wordpress
      - WORDPRESS_DB_PASSWORD=wordpress
      - WORDPRESS_DB_NAME=wordpress
    ports:
      - "11240:80"
    mount:
      - "/home/workshop/wordpress:/var/www/html"
    capAdd:
      - CAP_NET_BIND_SERVICE
    cap-drop:
      - CAP_NET_RAW
    depends_on:
      - db

  db:
    image: mysql:latest
    env:
      - MYSQL_ROOT_PASSWORD=wordpress
      - MYSQL_DATABASE=wordpress
      - MYSQL_USER=wordpress
      - MYSQL_PASSWORD=wordpress
    mount:
      - "/home/workshop/mysql:/var/lib/mysql"
    devices:
      - "/dev/null:/dev/null"

policies:
  - type: east-west
    source: wp
    destination: db
    protocol: tcp
    dest_port: 3306
    comment: "wp -> db 3306/tcp"

  - type: east-west
    source: client
    destination: wp
    protocol: tcp
    dest_port: 80
    comment: "client -> wp 80/tcp"
```

## Create

Create a Bottle from a definition file:

```sh
raind bottle create -f bottle.yaml
```

## List / Show

```sh
raind bottle ls
raind bottle show wordpress
```

## Start / Stop / Delete

```sh
raind bottle start wordpress
raind bottle stop wordpress
raind bottle delete wordpress
```

## Behavior

Raind calculates a startup order from `depends_on`. Dependencies must reference known service names, and dependency cycles are rejected.

When a Bottle needs an isolated group network, Raind can create a Bottle network and attach the generated containers to it. Services can also specify an explicit `network`.

## Policy Notes

Raind's Bottle model is designed for explicit east-west traffic policy. In typical Bottle usage, container-to-container traffic should be declared through `policies` instead of relying on implicit access.

## Notes

- `bottle.name` is required.
- `services` must contain at least one service.
- `depends_on` controls startup ordering.
- `device` and `devices` are aliases and are merged.
- `capAdd` and `cap-add` are aliases and are merged.
- `capDrop` and `cap-drop` are aliases and are merged.
- Bottle is Raind-native and is not a Kubernetes resource.
