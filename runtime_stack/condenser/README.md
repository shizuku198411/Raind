# Condenser
<p>
  <img src="assets/condenser_icon.png" alt="Project Icon" width="190">
</p>

[日本語版README](./README_jp.md)  

Condenser is one of the components of the Raind container runtime stack and serves as the high-level container runtime.  
It is responsible for container lifecycle management, image management, and providing a REST API for external control.  
Condenser orchestrates container operations by invoking the low-level runtime Droplet.  

## Runtime Stack Architecture
The Raind container runtime stack is composed of three layers:

- Raind CLI – A user interface for operating the entire runtime stack
- Condenser – A high-level container runtime responsible for container lifecycle and image management (this repository)
- Droplet – A low-level container runtime that handles container startup, deletion, and related operations  
  (repository: https://github.com/pyxgun/Droplet)

Condenser acts as the control plane of the Raind stack.
It translates high-level API requests into concrete container operations by generating OCI specifications, managing container state, and delegating execution to Droplet.

### Runtime Stack Install
If you'd like to install Raind, clone/build from the following repository is recommended.    
[Raind - Zero Trust Oriented Container Runtime](https://github.com/shizuku198411/Raind)


## Features
Condenser currently supports:

- Container lifecycle management
  - Create, start, stop, delete, attach, exec, and logs
  - Coordination with Droplet for low-level container execution
  - Hook-based state updates from the runtime

- Image management
  - Pulling container images from Docker Hub
  - Managing image layers and extracted root filesystems

- Pod orchestration (Kubernetes-style semantics)
  - Pod create/start/stop/remove and list/detail
  - Multiple containers share Network/UTS/IPC namespaces
  - Infra (pause) container keeps namespaces stable across restarts

- ReplicaSet orchestration (Kubernetes-style semantics)
  - ReplicaSet create/scale/remove and list/detail
  - Controller reconciles desired replicas and recreates pods as needed

- Service (L4 load balancing)
  - Selector-based service for Pods
  - iptables DNAT distribution to Pod infra IPs
  - Supports TCP/UDP ports

- Bottle orchestration (Compose-style semantics)
  - Manage a group of containers as one unit
  - Each container runs in its own namespaces
  - Actions are coordinated via the Bottle API

- REST API
  - HTTP-based interface for controlling containers, pods, bottles, and images
  - Designed to be consumed by Raind CLI or external tools

## Build
Requirements

- Linux kernel with namespace & cgroup support
- Go (version 1.25 or later)
- root privileges (or appropriate capabilities)
- `swag` (Swagger generator) available in `PATH`

```bash
git clone https://github.com/your-org/condenser.git
cd condenser
./scripts/build.sh
```

## Usage

Condenser is designed to run as a long-lived service and to be accessed via its REST API, typically from the Raind CLI.

### Start Condenser
```bash
sudo ./bin/condenser
```

By default, Condenser starts an HTTP server and waits for API requests to manage containers and images.

## Typical Workflow
A typical container startup sequence in the Raind stack looks like this:

- A client (Raind CLI or external tool) sends a request to Condenser via the REST API
- Condenser pulls the required image from Docker Hub (if not already present)
- Condenser generates an OCI-compliant config.json and prepares the container bundle
- Condenser invokes Droplet to create and start the container
- Condenser tracks container state and exposes it via the API

## Pod/ReplicaSet/Service

### Pod
- A Pod is a logical group of containers that share Network/UTS/IPC namespaces.
- An infra container is created to maintain stable namespaces for all members.

### ReplicaSet
- A ReplicaSet ensures a desired number of Pods for a given template and selector.
- Status fields:
  - `desired`: target replicas
  - `current`: number of matched Pods
  - `ready`: Pods where `runningContainers == desiredContainers`

### Service
- L4 load balancer for Pods using label selectors.
- Implemented via iptables DNAT with per-service chains (`RAIND-SVC-*`).

### Apply / Delete (YAML)
Condenser supports kubectl-style YAML manifests. A single YAML file can include multiple resources.

Apply:
```bash
curl -X POST https://localhost:7755/v1/resource/apply \
  -H "Content-Type: text/plain" \
  --data-binary @/path/to/manifest.yaml
```

Delete:
```bash
curl -X POST https://localhost:7755/v1/resource/delete \
  -H "Content-Type: text/plain" \
  --data-binary @/path/to/manifest.yaml
```

Supported kinds:
- `Pod`
- `ReplicaSet`
- `Service`

Example manifest:
```yaml
apiVersion: apps/v1
kind: ReplicaSet
metadata:
  name: test-rs
  namespace: default
spec:
  replicas: 2
  selector:
    matchLabels:
      app: demo
  template:
    metadata:
      name: test-pod
      labels:
        app: demo
    spec:
      containers:
      - name: nginx
        image: nginx:latest
      - name: ubuntu
        image: ubuntu:latest
        tty: true
---
apiVersion: v1
kind: Service
metadata:
  name: demo-svc
  namespace: default
spec:
  selector:
    app: demo
  ports:
  - port: 11240
    targetPort: 80
    protocol: TCP
```

## Pod vs Bottle

- Use **Pods** for tightly-coupled components that must share Network/UTS/IPC (sidecars, helpers, same IP/hostname).
- Use **Bottles** for loosely-coupled services that should remain isolated but managed as a group.

## Status
Condenser and the Raind container runtime stack are currently under active development.  
APIs, storage formats, and behavior may change without notice.
