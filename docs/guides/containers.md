# Container Workflows

Raind container commands provide Docker-like workflows on top of the Raind runtime stack.

## Command shape

```sh
raind container create [flags] <image:tag> [command...]
raind container run    [flags] <image:tag> [command...]
raind container start  [--tty] <container-id-or-name>
raind container stop   <container-id-or-name>
raind container rm     <container-id-or-name>
raind container ls
raind container attach <container-id-or-name>
raind container exec   [--tty] <container-id-or-name> [command...]
raind container logs   [--line <n>] [--pager] <container-id-or-name>
raind container inspect <container-id-or-name> [--json]
```

`container run` creates and starts a container. It does not currently use a Docker-style `-d` flag. For long-running detached-style workflows, run a long-lived foreground process such as `nginx`, `sleep`, or your service entrypoint.

## Common flags

| Flag | Commands | Description |
|---|---|---|
| `--name <name>` | `create`, `run` | Assign a container name. |
| `--network <network>` | `create`, `run` | Attach to a Raind network. Defaults to `raind0`. |
| `-p, --publish <host:container[:protocol]>` | `create`, `run` | Publish a host port to a container port. Protocol defaults to `tcp`. |
| `-v, --volume <host-path:container-path>` | `create`, `run` | Bind mount a host path into the container. |
| `-e, --env <KEY=VALUE>` | `create`, `run` | Add environment variables. |
| `--device <SRC[:DST[:rwm]]>` | `create`, `run` | Add a host device. |
| `--cap-add <CAP>` | `create`, `run` | Add Linux capabilities. |
| `--cap-drop <CAP>` | `create`, `run` | Drop Linux capabilities. |
| `-t, --tty` | `create`, `run`, `start`, `exec` | Allocate or attach a TTY. |
| `-i, --interactive` | `create` | Mark the container interactive. |
| `--rm` | `run` | Remove the container after the process exits. |
| `--rootless` | `create`, `run` | Enable rootless user namespace mapping. |
| `--rootless-mode <mode>` | `create`, `run` | Select `shifted-root` or `login-root`. Setting this flag enables rootless mode. |

## Examples

Run nginx and publish port 8080:

```sh
raind image pull nginx:latest
raind container run --name web -p 8080:80 nginx:latest
```

Run a shell command and remove the container afterwards:

```sh
raind container run --rm alpine:latest /bin/sh -c 'echo hello from alpine'
```

Mount a host directory:

```sh
mkdir -p /tmp/raind-data
raind container run --name data -v /tmp/raind-data:/data alpine:latest /bin/sh -c 'ls -lan /data; sleep 60'
```

Execute commands:

```sh
raind container exec web nginx -v
raind container exec web /bin/cat /etc/os-release
raind container exec --tty web /bin/sh
```

Read logs:

```sh
raind container logs --line 80 web
```

Inspect the stored runtime configuration:

```sh
raind container inspect web
raind container inspect web --json
```

The inspect output is based on the container `config.json`, but capability, seccomp, and AppArmor details are summarized as the applied Raind security profile name.

Stop and remove:

```sh
raind container stop web
raind container rm web
```

## Rootless containers

Rootless is available for standalone containers:

```sh
raind container run --name r1 --rootless alpine:latest /bin/sh -c 'id; sleep 60'
raind container run --name r2 --rootless-mode login-root alpine:latest /bin/sh -c 'id; sleep 60'
```

See [Rootless containers](rootless-containers.md).
