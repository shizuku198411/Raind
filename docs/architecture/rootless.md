# Rootless Architecture

This document describes how Raind implements rootless containers.

For user-facing commands, see [Rootless containers](../guides/rootless-containers.md). For a compact mode reference, see [Rootless modes](../reference/rootless-modes.md).

## Control flow

```text
raind CLI
  |
  | container create/run request
  | rootless, rootlessMode, rootlessRootUID, rootlessRootGID
  v
Condenser
  |
  | normalize rootless options
  | prepare image/rootfs metadata
  | prepare rootless-shifted cache progress
  | generate Droplet spec annotations
  v
Droplet
  |
  | read io.raind.rootless annotation
  | build SysProcAttr UID/GID maps
  | start init in a user namespace
  | prepare rootless writable paths
  | mount/pivot_root/exec container process
  v
container process
```

For Pod manifests, `spec.hostUsers: false` enables rootless execution for Pod-managed containers. Rootless Pod app containers share the infra container's network, IPC, and UTS namespaces so Service and Ingress resources can target the Pod IP, while each app container keeps its own user namespace mapping. Raind configures the shared Pod network namespace so rootless workloads can bind normal service ports such as 80.

## Spec annotation

Rootless configuration is encoded in the Droplet spec annotation `io.raind.rootless`.

Conceptually:

```json
{
  "enabled": true,
  "mode": "login-root",
  "hostRootUID": 1000,
  "hostRootGID": 1000
}
```

The annotation is only emitted when rootless is enabled.

## Modes

### `shifted-root`

Default rootless mode:

```text
container 0..65535 -> host 100000..165535
```

With default settings:

```text
uid_map: 0 100000 65536
gid_map: 0 100000 65536
```

### `login-root`

Host-friendly development mode:

```text
container 0        -> host login UID/GID, size 1
container 1..65535 -> host subordinate range, size 65535
```

With login user `1000:1000` and default subordinate range:

```text
uid_map: 0 1000   1
         1 100000 65535

gid_map: 0 1000   1
         1 100000 65535
```

## Mapping policy

Droplet converts the annotation into a mapping policy:

```text
mode
uidBase / gidBase
mapSize
rootUID / rootGID
```

Defaults:

```text
RAIND_ROOTLESS_UID_BASE=100000
RAIND_ROOTLESS_GID_BASE=100000
RAIND_ROOTLESS_ID_MAP_SIZE=65536
```

For `login-root`, the CLI resolves `rootUID/rootGID` before the request reaches the daemon. This is important when the CLI is invoked through `sudo`: `SUDO_UID` and `SUDO_GID` represent the login user, while `os.Getuid()` inside a root-running daemon would be `0`.

## Process creation

Rootless containers use a user namespace. Droplet constructs `syscall.SysProcAttr` with:

- namespace clone flags
- UID/GID mappings
- `GidMappingsEnableSetgroups=true` for rootless maps, so images that call `initgroups(3)` while dropping privileges can still start workers inside the mapped GID range
- `Credential{Uid: 0, Gid: 0, NoSetGroups: true}` so the init process starts as root inside the new user namespace

The init process then performs the normal container setup path: mount preparation, rootfs setup, `pivot_root`, capabilities/seccomp/AppArmor handling, and final `exec`.

## Rootfs and writable path preparation

Rootless mode requires host-side file ownership to match the user namespace ID map.

Raind prepares rootless-shifted image layer caches. Cache keys include the rootless mode and mapping parameters so different modes can coexist safely.

Raind also adjusts runtime-writable paths such as upperdir, workdir, rootfs, logs, and initialization IPC paths to the host ID that maps to container root for the selected mode.

## Exec path

`raind container exec` uses a common nsenter builder for rootfull and rootless containers.

For rootfull containers, exec enters the target mount, UTS, IPC, network, PID, and cgroup namespaces, adopts the target process root, and uses the OCI working directory.

For rootless containers, exec additionally enters the user namespace and switches to UID/GID 0 inside that namespace:

```text
nsenter -t <pid> -U --setuid 0 --setgid 0 -m -u -i -n -p -C --root --wd=<cwd> -- <command>
```

Command lookup resolves bare executables against `/proc/<pid>/root` and the container `PATH`, so commands are resolved inside the container rootfs.

## Pod status

Rootless Pod manifests are supported through `spec.hostUsers: false`.

Current implementation notes:

- the Pod infra container and app containers are created with rootless annotations
- app containers are tracked as Pod members
- app containers pre-join the infra container's network, IPC, and UTS namespaces before creating their own user namespace
- app containers keep independent user namespace mappings instead of sharing the infra container's user namespace
- app containers set the shared network namespace's `net.ipv4.ip_unprivileged_port_start` to `0` before entering their rootless user namespace, allowing ports such as 80 without granting host root privileges

Full Kubernetes-style shared user namespace behavior for Pod members still needs additional design around infra container ownership and ID-map compatibility across app containers.
