# Container Architecture

Raind standalone containers are created by Condenser and executed by Droplet. Condenser handles high-level lifecycle, image resolution, state, IPAM, cgroups, generated runtime specs, and host-level forwarding. Droplet handles the low-level container runtime work: namespaces, mounts, pivot_root, capabilities, seccomp, AppArmor, hooks, process startup, attach/exec, and state.

## High-level flow

```text
raind CLI
  |
  | mTLS API call
  v
Condenser management API :7755
  |
  v
ContainerService.Create
  |
  +--> image pull / image config
  +--> IPAM address allocation
  +--> CSM state entry
  +--> container directory setup
  +--> /etc files
  +--> cgroup subtree setup
  +--> droplet spec -> config.json
  +--> host port forwarding, if requested
  +--> droplet create
  v
Container state: created

raind container start
  |
  v
Condenser -> droplet start
  |
  v
container init process runs
```

## Control-plane and runtime split

```text
+------------------------------+       +------------------------------+
| Condenser                    |       | Droplet                      |
|------------------------------|       |------------------------------|
| API and auth                 |       | low-level runtime commands   |
| CSM / IPAM / ILM stores      |       | config.json loader           |
| image pull and extraction    |       | namespace setup              |
| generated runtime spec       | ----> | mount + pivot_root           |
| cgroup directory creation    |       | capabilities / seccomp       |
| iptables host forwarding     |       | AppArmor on exec             |
| hooks configuration          |       | process exec                 |
+------------------------------+       +------------------------------+
```

Condenser produces the desired runtime state. Droplet applies that state to the Linux kernel.

## Runtime directories and stores

The default host-side layout is:

```text
/etc/raind/
  container/
    <container-id>/
      config.json
      rootfs / merged / diff / work
      logs/
      cert/
  image/
    layers/
  store/
    container/
      csm.json
      bsm.json
    image/
      ilm.json
    network/
      ipam.json
      npm.json
    resource/
      pod/
        psm.json
      service/
        ssm.json
      ingress/
        ism.json

/var/log/raind/
  raind_audit.jsonl
  raind_dns.jsonl
  raind_netflow.jsonl
  raind_metrics.jsonl

/sys/fs/cgroup/raind/
  <container-id>/
```

The most relevant stores for standalone containers are:

| Store | Meaning |
|---|---|
| CSM | Container state manager. Tracks ID, name, state, PID, image, command, log path, Pod ID if any. |
| IPAM | Bridge pools and container address allocations. |
| ILM | Image/layer metadata. |
| PSM | Pod templates and Pod state, only used when the container belongs to a Pod. |

## Container creation pipeline

```text
ContainerService.Create
  |
  +-- 1. generate container ID and name
  |
  +-- 2. parse image reference
  |
  +-- 3. pull image if missing
  |
  +-- 4. load image config
  |
  +-- 5. allocate address from target bridge
  |
  +-- 6. create CSM entry: state=creating
  |
  +-- 7. setup /etc/raind/container/<id>
  |
  +-- 8. setup generated /etc files for container
  |
  +-- 9. setup /sys/fs/cgroup/raind/<id>
  |
  +-- 10. generate droplet config.json
  |
  +-- 11. install host forwarding rules, if needed
  |
  +-- 12. droplet create
  |
  +-- 13. state becomes created
```

Rollback is applied on failure. For example, Condenser releases IPAM allocations, removes CSM entries, deletes container directories, removes cgroup subtrees, and cleans up forwarding rules where appropriate.

## Image and root filesystem

Raind resolves the requested image, pulls it if missing, selects the target platform, downloads layers, and prepares the root filesystem.

At spec generation time, Condenser passes Droplet:

```text
rootfs       -> /etc/raind/container/<id>/merged
image layers -> ILM rootfs path
upper_dir    -> /etc/raind/container/<id>/diff
work_dir     -> /etc/raind/container/<id>/work
```

Droplet mounts the container root filesystem using the configured image layer and writable overlay directories, then performs `pivot_root` into the new root.

```text
image layers
    |
    v
overlay mount
    |
    +-- lowerdir = image layer rootfs
    +-- upperdir = container diff
    +-- workdir  = container work
    v
merged rootfs
    |
    v
pivot_root
```

## Namespace model

For standalone containers, Condenser requests these namespaces by default:

```text
mount
network
uts
pid
ipc
cgroup
```

Rootful standalone containers do not use a user namespace by default.

```text
host
  |
  v
droplet create
  |
  v
container init process
  +-- new mount namespace
  +-- new network namespace
  +-- new UTS namespace
  +-- new PID namespace
  +-- new IPC namespace
  +-- new cgroup namespace
```

Diagram:

```text
                  host namespace set

  +--------------------------------------------------+
  | host mnt / net / uts / pid / ipc / cgroup        |
  +-----------------------+--------------------------+
                          |
                          | clone/unshare via droplet
                          v
  +--------------------------------------------------+
  | container namespace set                          |
  |                                                  |
  | mnt: private rootfs + mounts                     |
  | net: veth connected to Raind bridge              |
  | uts: container hostname                          |
  | pid: isolated process tree                       |
  | ipc: isolated SysV/POSIX IPC                     |
  | cgroup: container cgroup view                    |
  +--------------------------------------------------+
```

## Network setup

Condenser allocates an IP address from the selected bridge pool. Droplet receives:

```text
host interface
bridge interface
container interface name
container interface address
container gateway
container DNS entries
```

The container network shape is:

```text
container netns
  eth-like interface rd_<container-id>
        |
        | veth peer
        v
host bridge raind0 or rns...
        |
        v
host routing / NAT / DNS redirect
```

The bridge address is used as the container's default gateway and resolver address. DNS traffic to the bridge gateway is redirected to Raind's DNS proxy.

## `/etc` files

Condenser generates container-side runtime files before Droplet starts the process. The important ones are conceptually:

```text
/etc/hosts
/etc/hostname
/etc/resolv.conf
```

The resolver points at the bridge gateway:

```text
nameserver <bridge-gateway-ip>
```

The host redirects bridge DNS traffic to the Raind DNS proxy.

## Cgroup architecture

Raind uses cgroup v2 under:

```text
/sys/fs/cgroup/raind
```

At bootstrap, Condenser ensures the runtime cgroup directory exists and enables available controllers such as:

```text
cpu
memory
pids
io
```

For each container, Condenser creates:

```text
/sys/fs/cgroup/raind/<container-id>
```

Droplet attaches the container init process to the cgroup and applies resource-related runtime settings. Metrics collection later reads cgroup files for CPU, memory, IO, and OOM information.

```text
/sys/fs/cgroup/raind
  |
  +-- <container-id>
        |
        +-- cgroup.procs
        +-- cpu.stat
        +-- memory.current
        +-- memory.max
        +-- io.stat
        +-- pids.current
```

## Mount architecture

Droplet prepares the root environment in the init path.

Typical order:

```text
1. join existing namespaces if configured
2. make mount propagation private
3. setup overlay root filesystem
4. mount configured filesystems
5. mount standard device files
6. create required runtime directories
7. pivot_root into the container rootfs
8. unmount old root
```

The generated mount set includes standard mounts such as:

```text
/proc
/dev
/sys
```

User mounts are passed from CLI or manifest fields as bind mounts. Directory hostPath mounts from resource manifests become bind mounts in the generated Droplet spec.

Example:

```text
/home/workshop/data:/data:ro
```

becomes a read-only bind mount:

```text
source      = /home/workshop/data
destination = /data
options     = bind,ro
```

Droplet validates user mounts before applying them. Device paths require explicit device mount handling and are guarded separately.

## Capabilities

Raind supports capability additions and drops from CLI/resource inputs.

```yaml
securityContext:
  capabilities:
    add:
      - NET_ADMIN
    drop:
      - NET_RAW
```

The manifest layer converts these to Droplet capability options. Droplet normalizes names like `NET_ADMIN` and `CAP_NET_ADMIN`, then applies the configured capability set in the container init process.

Conceptually:

```text
image process default capabilities
        |
        +-- cap-add
        +-- cap-drop
        v
final process capability sets
```

Capabilities matter for operations such as raw sockets, network administration, mounting, and other privileged kernel operations.

## Seccomp and AppArmor

Droplet includes seccomp and AppArmor integration.

### Seccomp

Droplet can install a seccomp deny filter during init. Seccomp is applied before the final process is executed.

```text
container init
  |
  +-- setup namespaces / mounts
  +-- set capabilities
  +-- install seccomp filter
  v
exec workload process
```

### AppArmor

Raind bootstraps a default AppArmor profile when the host supports AppArmor. Droplet applies AppArmor on exec. If AppArmor is unavailable in constrained environments, Raind treats some AppArmor setup failures as non-fatal so containers can still run in environments such as nested CI/workshop setups.

```text
droplet init
  |
  +-- ApplyAAProfileOnExec(profile)
  v
exec workload under profile
```

## Hook flow

Condenser configures Droplet hooks in the generated spec. The hook agent calls back into Condenser over mTLS.

```text
droplet lifecycle event
  |
  v
condenser-hook-agent
  |
  | HTTPS + client cert
  v
Condenser hook server :7756
```

Hook phases include lifecycle points such as:

```text
createRuntime
createContainer
poststart
stopContainer
poststop
```

One important use of hooks is to let Condenser synchronize runtime state and issue per-container hook/client certificates.

## Start and process execution

`droplet create` prepares the runtime bundle and init state. `droplet start` starts the process.

```text
raind container start
  |
  v
Condenser Start
  |
  v
droplet start <container-id>
  |
  v
container init
  |
  +-- namespace setup
  +-- mount setup
  +-- pivot_root
  +-- capability/seccomp/AppArmor
  v
exec workload command
```

The command comes from user input when provided. Otherwise, Raind combines the image config's entrypoint and command.

## Attach, logs, and exec

Containers can be started with or without TTY.

Log paths differ by mode:

```text
TTY container:
  /etc/raind/container/<id>/logs/console.log

non-TTY container:
  /etc/raind/container/<id>/logs/init.log
```

Exec and attach are delegated to Droplet. Condenser resolves container state and invokes the runtime handler.

## Port forwarding

Standalone containers may publish host ports.

```text
host:<hostPort>
      |
      v
iptables DNAT
      |
      v
containerIP:<containerPort>
```

Raind adds PREROUTING and OUTPUT DNAT rules and FORWARD rules. It also handles bridge hairpin traffic so containers can access host-local published ports.

Pod member containers do not install standalone host forwarding rules. In Pod workflows, exposure should usually go through Service and Ingress resources.

## Standalone container vs Pod member container

| Aspect | Standalone container | Pod member container |
|---|---|---|
| IP address | Own container IP | Shares Pod infra IP if joining Pod namespaces |
| Network namespace | New network namespace | Joins infra container network namespace |
| IPC/UTS | Own namespaces | Joins infra IPC/UTS namespaces |
| Port forwarding | Can install host forwarding | Not installed directly for app members |
| Hostname | Container ID | Short Pod ID / shared Pod UTS |
| Service backend | Usually not selected directly | Pod infra IP is selected by Service controller |

## Security notes

- Management API is mTLS protected and bound to localhost.
- Containers are blocked from direct management API access by host INPUT rules.
- Hook access is allowed because containers need runtime lifecycle callbacks.
- Capabilities should be added only when required.
- HostPath mounts should be treated as privileged access to host data.
- Device mounts are guarded and should be explicit.
- AppArmor/seccomp hardening depends on host support.

## Debugging commands

```sh
raind container ls
raind container spec <container>
raind container logs <container>
raind container stats <container>

sudo ip netns list
ip addr show
sudo iptables -t nat -nvL
sudo cat /sys/fs/cgroup/raind/<container-id>/cgroup.procs
sudo cat /sys/fs/cgroup/raind/<container-id>/memory.current
```


## Related documents

- [Runtime stack](runtime-stack.md)
- [Exec architecture](exec.md)
- [Rootless architecture](rootless.md)
