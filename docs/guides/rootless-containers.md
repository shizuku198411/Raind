# Rootless Containers

Raind supports rootless execution for standalone containers by creating a user namespace and mapping container IDs to non-root host IDs.

Rootless support is currently focused on single containers created through `raind container create` and `raind container run`. Pod-managed containers are not yet supported in rootless mode.

## Modes

Raind currently supports two rootless mapping modes:

| Mode | Enable with | Container UID 0 maps to | Main use case |
|---|---|---|---|
| `shifted-root` | `--rootless` or `--rootless-mode shifted-root` | subordinate host UID base, default `100000` | isolate all container IDs from the login user |
| `login-root` | `--rootless-mode login-root` | the invoking login user's UID/GID | Docker-rootless-like bind mount ownership for local development |

Setting `--rootless-mode` explicitly enables rootless mode, even when `--rootless` is omitted.

## `shifted-root`

`shifted-root` is the default rootless mode.

Default UID/GID map:

```text
container 0..65535 -> host 100000..165535
```

Example:

```sh
raind container run --name shifted-demo \
  --rootless \
  alpine:latest /bin/sh -c 'id; cat /proc/self/uid_map; cat /proc/self/gid_map; sleep 60'
```

In this mode, files created by container root on a bind mount are owned by the shifted host ID, normally `100000:100000`. This is more isolated from the login user, but may be less convenient for editing bind-mounted files from the host.

## `login-root`

`login-root` maps container root to the user's login UID/GID, and maps non-root container IDs into the subordinate range.

Default UID/GID map, assuming login user `1000:1000`:

```text
container 0       -> host 1000      size 1
container 1..65535 -> host 100000..165534
```

Example:

```sh
mkdir -p /tmp/raind-login-root

raind container run --name login-root-demo \
  --rootless-mode login-root \
  -v /tmp/raind-login-root:/data \
  alpine:latest /bin/sh -c 'id; echo hello > /data/hello.txt; sleep 60'

ls -lan /tmp/raind-login-root
```

Expected host-side behavior:

```text
hello.txt is owned by the login user's UID/GID
```

This mode is useful when a development container writes into a host project directory and the host user should be able to edit or remove those files without manual `chown`.

## UID/GID source for `login-root`

The CLI sends the intended host UID/GID to Condenser and Droplet.

- normally, Raind uses the CLI process UID/GID
- when invoked through `sudo`, Raind prefers `SUDO_UID` and `SUDO_GID`

This prevents a root-running daemon from accidentally treating host root (`0:0`) as the login user.

## Custom subordinate ranges

The default subordinate ID range is controlled by environment variables used by the runtime:

```sh
RAIND_ROOTLESS_UID_BASE=100000
RAIND_ROOTLESS_GID_BASE=100000
RAIND_ROOTLESS_ID_MAP_SIZE=65536
```

These values affect the generated user namespace mappings and rootless-shifted image cache keys.

## Image layer shifting

Rootless containers need rootfs files to be visible with host IDs that match the selected ID map. Raind prepares a rootless-shifted layer cache next to image layer rootfs data.

Cache examples:

```text
rootless-shifted/uid_100000_gid_100000_size_65536_v1/rootfs
rootless-shifted/login_root_uid_1000_gid_1000_base_100000_100000_size_65536_v1/rootfs
```

Different mapping policies use different cache keys, so `shifted-root` and `login-root` do not overwrite each other.

## Exec behavior

`raind container exec` joins the target container namespaces with `nsenter`. For rootless containers, exec also enters the user namespace and switches to namespace root before running the command.

The exec path also adopts the target process root and OCI working directory so bare commands such as `nginx -v` are resolved against the container rootfs rather than the host rootfs.

## Limitations

Current limitations:

- rootless is supported for standalone containers only
- Pod-managed containers reject rootless mode
- host device access remains constrained by the host and runtime permissions
- rootless networking still depends on Raind's host-side network setup
- subordinate ID ranges are environment-configured, not yet read from `/etc/subuid` and `/etc/subgid`

## Troubleshooting

Check the user namespace map from inside a container:

```sh
raind container exec login-root-demo /bin/cat /proc/self/uid_map
raind container exec login-root-demo /bin/cat /proc/self/gid_map
```

Check bind mount ownership from the host:

```sh
ls -lan /tmp/raind-login-root
```

Check rootless cache creation in image/container directories:

```sh
sudo find /etc/raind -path '*rootless-shifted*' -maxdepth 8 -print
```
