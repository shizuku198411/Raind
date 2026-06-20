# Raind

<p align="center">
  <img src="./assets/raind_icon.png" alt="Raind" width="140">
</p>

<p align="center">
  <strong>A local deployment validation runtime for containers and Kubernetes-style workloads.</strong>
</p>

<p align="center">
  <a href="./LICENSE"><img alt="License: MIT" src="https://img.shields.io/badge/license-MIT-blue.svg"></a>
  <img alt="Status: experimental" src="https://img.shields.io/badge/status-experimental-orange.svg">
  <img alt="Go" src="https://img.shields.io/badge/go-1.25%2B-00ADD8.svg">
  <img alt="Platform" src="https://img.shields.io/badge/platform-linux-lightgrey.svg">
  <img alt="Runtime" src="https://img.shields.io/badge/runtime-OCI--style-5C6BC0.svg">
  <img alt="Kubernetes-style" src="https://img.shields.io/badge/resources-Kubernetes--style-326CE5.svg">
</p>

> [!WARNING]
> Raind is under active development. It is not a Kubernetes distribution and does not aim to be fully Kubernetes-compatible. It supports a growing subset of Kubernetes-style resources for local deployment testing.

Raind is an experimental runtime stack that runs **single containers**, **local multi-container groups**, and **Kubernetes-style resources** through one runtime path.

It is designed for local pre-deployment checks where you want to validate how an application behaves with containers, Pods, Services, Ingress, PVCs, Secrets, and NetworkPolicy-like traffic control, without starting a full Kubernetes cluster.

## Demo

<!-- TODO: Place GIF here.
Recommended file: ./assets/demo/raind-quickstart.gif

Suggested content:
1. Start Raind.
2. Run a single nginx container.
3. Apply a Kubernetes-style Deployment + Service.
4. Show `raind resource get deploy`, `raind resource get service`.
5. Access the service locally.
-->

![Raind quickstart demo](./assets/demo/raind-quickstart.gif)

## Why Raind?

Local container workflows often jump between different tools and mental models:

- one tool for single containers,
- another model for Pod-style workloads,
- another layer for Services and Ingress,
- another place to inspect traffic and policy.

Raind tries to keep those concerns in one runtime.

With Raind, you can:

- run Docker-like single containers,
- apply Kubernetes-style manifests,
- run Pod-like workloads through an infra-container model,
- reconcile ReplicaSet and Deployment resources,
- route traffic with Service and Ingress resources,
- mount PVC-backed local volumes,
- inject ConfigMap and Secret values,
- generate runtime security policy from NetworkPolicy resources,
- inspect observed network flows from the runtime itself.

Raind is useful when you want a lightweight local runtime for checking deployment behavior before moving to Docker, Compose, Kubernetes, or a real cluster.

## What Raind is not

Raind is intentionally not positioned as a full replacement for Docker, containerd, kind, minikube, or Kubernetes.

| Tool | Primary role |
|---|---|
| Docker / Podman | Run and manage containers |
| kind / minikube | Run a real local Kubernetes cluster |
| Kubernetes | Production-grade orchestration platform |
| Raind | Local runtime for container and Kubernetes-style deployment validation |

Raind focuses on the space between single-container testing and full-cluster testing.

## Features

### Runtime foundation

- Low-level OCI-style runtime layer
- Container lifecycle: create, start, exec, stop, kill, remove
- Namespace, mount, cgroup, capability, seccomp, and AppArmor-related runtime paths
- Rootful and rootless-oriented runtime work
- Runtime state managed through the Raind stack

### Container workflows

```sh
raind container run --name web -p 8080:80 nginx:latest
raind container exec web /bin/sh
raind container logs web
raind container stop web
raind container rm web
```

### Kubernetes-style resources

Raind supports a growing subset of Kubernetes-style resources.  

Example manifest:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: web
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web-deploy
  namespace: web
spec:
  replicas: 2
  selector:
    matchLabels:
      app: web-server
  template:
    metadata:
      labels:
        app: web-server
    spec:
      containers:
        - name: nginx-web
          image: nginx:latest
          ports:
            - containerPort: 80
```

Typical workflow:

```sh
raind resource apply -f app.yaml

raind resource get ns
raind resource get pod
raind resource get deploy
raind resource get service
raind resource get ingress
```

### Network visibility and policy

Raind connects Kubernetes-style NetworkPolicy resources to runtime-managed security policy.

```sh
raind resource apply -f networkpolicy.yaml

raind security policy ls
raind logs netflow
```

Example output:

```text
POLICY TYPE : East-West
CURRENT MODE: deny_by_default

FLAG  SRC CONTAINER       DST CONTAINER   PROTOCOL  DST PORT  ACTION
[*]   nextcloud-01kvfymx  mysql-01kvfyms  tcp       3306      ALLOW
  >> DENY ALL EAST-WEST TRAFFIC <<
```

```text
ALLOW   FROM: nextcloud-01kvfymx => TO: mysql-01kvfyms {TCP/3306}
```

## Quickstart

See the full quickstart guide:

- [Installation](./docs/getting-started/installation.md)
- [Quickstart](./docs/getting-started/quickstart.md)
- [Testing](./docs/getting-started/testing.md)

Minimal example:

```sh
# Run a single container
raind container run --name web -p 8080:80 nginx:latest
raind container ls

# Apply Kubernetes-style resources
raind resource apply -f examples/quickstart/web.yaml
raind resource get deploy
raind resource get service
```

## Architecture

Raind is split into three main layers.

### `droplet`

`droplet` is the low-level OCI-style container runtime layer.

It handles container lifecycle and low-level Linux runtime operations such as namespaces, mounts, capabilities, cgroups, hooks, and runtime state.

### `condenser`

`condenser` is the high-level runtime controller layer.

It manages images, containers, resources, networking, security policy, logs, and state, then delegates low-level execution to `droplet`.

### `raind`

`raind` is the user-facing CLI.

It exposes container, image, bottle, resource, policy, network, and log workflows through a single command surface.

## Documentation

- [Documentation index](./docs/)
- [Architecture](./docs/architecture/)
- [Resource reference](./docs/resources/)
- [CLI reference](./docs/reference/cli.md)
- [Manifest schema](./docs/reference/manifest-schema.md)
- [Security](./SECURITY.md)
- [Contributing](./CONTRIBUTING.md)

## Project status

Raind is experimental and evolving quickly.

Current focus areas include:

- expanding Kubernetes-style resource support,
- improving manifest validation and unsupported-field warnings,
- strengthening reconciliation behavior,
- improving Service and Ingress behavior,
- improving NetworkPolicy reconciliation,
- improving runtime observability and troubleshooting output,
- hardening state management and security-sensitive paths.

## Contributing

Contributions, bug reports, compatibility reports, and runtime investigations are welcome.

Useful starting points:

- [CONTRIBUTING.md](./CONTRIBUTING.md)
- [SUPPORT.md](./SUPPORT.md)
- [CODE_OF_CONDUCT.md](./CODE_OF_CONDUCT.md)

Good issues include:

- adding small manifest examples,
- improving resource documentation,
- improving unsupported-field warnings,
- adding tests for controllers and resource behavior,
- reporting manifests that work on Kubernetes but not yet on Raind.

## License

Raind is licensed under the [MIT License](./LICENSE).
