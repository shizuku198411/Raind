# Quickstart

This quickstart assumes Raind is installed, Condenser is running, and the current user can access the Raind management API.

For installation steps, see [Installation](installation.md).

## Check the runtime

```sh
raind --version
raind image ls
raind container ls
raind network ls
```

## Pull an image

```sh
raind image pull nginx:latest
```

## Run a container

```sh
raind container run --name web -p 8080:80 nginx:latest
```

Check state and logs:

```sh
raind container ls
raind container logs --line 80 web
```

Execute a command in the container:

```sh
raind container exec web nginx -v
raind container exec web /bin/cat /etc/os-release
```

Stop and remove it:

```sh
raind container stop web
raind container rm web
```

## Run a short-lived command

```sh
raind container run --rm busybox:latest /bin/sh -c 'echo hello from raind'
```

## Run a rootless container

The default rootless mode maps container root to an unprivileged subordinate host ID range:

```sh
raind container run --name rootless-demo --rootless alpine:latest /bin/sh -c 'id; sleep 30'
```

For host-friendly bind mount ownership, use `login-root`:

```sh
mkdir -p /tmp/raind-login-root
raind container run --name login-root-demo \
  --rootless-mode login-root \
  -v /tmp/raind-login-root:/data \
  alpine:latest /bin/sh -c 'id; echo hello > /data/hello.txt; sleep 30'

ls -lan /tmp/raind-login-root
```

See [Rootless containers](../guides/rootless-containers.md) for the mapping model and limitations.
