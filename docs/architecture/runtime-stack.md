# Runtime Stack

Raind is split into three runtime layers:

```text
raind CLI
  -> condenser: high-level runtime API, image/resource/policy/log controller
    -> droplet: low-level OCI-style container runtime
```

## `raind` CLI

`raind` is the user-facing command line. It sends authenticated management requests to Condenser for image, container, network, resource, Bottle, policy, and log workflows.

Typical command groups:

```sh
raind image ...
raind container ...
raind network ...
raind resource ...
raind bottle ...
raind policy ...
raind logs ...
```

## Condenser

Condenser is the high-level runtime service. It owns:

- management API and mTLS endpoint
- image pull/build/extraction state
- container state store
- IPAM and bridge/network state
- Kubernetes-style resource stores and controllers
- Bottle state
- policy state
- DNS, netflow, Service, and Ingress handling
- generated Droplet runtime specs

Condenser does not directly `pivot_root` into containers. It produces the desired runtime state and delegates low-level execution to Droplet.

## Droplet

Droplet is the low-level OCI-style runtime layer. It owns:

- namespace creation and joining
- container init process launch
- mount setup and `pivot_root`
- rootfs/image layer preparation
- capabilities, seccomp, and AppArmor on-exec handling
- attach and exec handling
- cgroup process placement
- runtime state files

## Why the split matters

Raind keeps high-level workload intent in Condenser while keeping Linux container mechanics in Droplet. This makes Docker-like containers, Bottles, and Kubernetes-style resources share the same runtime foundation.

Rootless support follows this split:

- the CLI captures the user's requested rootless mode and login UID/GID when needed
- Condenser normalizes rootless options and prepares rootless-shifted image caches
- Droplet applies user namespace mappings, prepares writable paths, and executes the container process in the mapped namespace
