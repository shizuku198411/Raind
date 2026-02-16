# Raind - Zero Trust Oriented Container Runtime
<p>
  <img src="assets/raind_icon.png" alt="Project Icon" width="190">
</p>

![version](https://img.shields.io/badge/version-v0.1.3-blue) ![PoC](https://img.shields.io/badge/PoC-00ac97)

Raind is a container runtime built as a PoC (proof of concept) to evaluate whether **Zero Trust can be implemented at the container runtime layer**.  
This repository is a meta-repo that places the three Raind components under `runtime_stack/` as git submodules.

**Components**
- `Raind-CLI`: user-facing CLI (UI layer / REST & WebSocket client)
- `Condenser`: high-level container runtime (control plane / API & state management)
- `Droplet`: low-level container runtime (execution plane / OCI-compliant)

Each component is placed as a submodule at the following locations:
- `runtime_stack/raind-cli` (Raind-CLI)
- `runtime_stack/condenser` (Condenser)
- `runtime_stack/droplet` (Droplet)

For detailed design and responsibility boundaries, see the [Design Document](docs/en/design.md).

## Key Features
- Zero Trust-oriented container control
- East-West traffic is default deny by policy
- North-South traffic switches between `observe`/`enforce` modes
- Idempotent policy application by fully rebuilding iptables management chains
- Log enrichment with NFLOG + ulogd2 + Condenser
- Container operations and policy management from the CLI
- Container/image/orchestration management by Condenser
- OCI-compliant container execution by Droplet

Raind separates "Operate (CLI)", "Control (Condenser)", and "Execute (Droplet)"  
into a runtime stack with clear API boundaries.

### Zero Trust-Oriented Container Control
In Raind, East-West traffic (container-to-container) is denied by default.  
Traffic is allowed only when **explicitly** permitted by policy at container start, which blocks lateral movement and recon during compromise. Combined with the log enrichment described below, this helps detect suspicious containers.  
When creating a Bottle (docker-compose-like), you can also define policies explicitly.

```yaml
# bottle.yaml
bottle:
  name: wordpress
services:
  wp:
    image: wordpress
    env:
      - WORDPRESS_DB_HOST=db:3306
      - WORDPRESS_DB_USER=wordpress
      - WORDPRESS_DB_PASSWORD=wordpress
      - WORDPRESS_DB_NAME=wordpress
    ports:
      - "8080:80"
    depends_on:
      - db
  db:
    image: mysql
    env:
      - MYSQL_ROOT_PASSWORD=wordpress
      - MYSQL_DATABASE=wordpress
      - MYSQL_USER=wordpress
      - MYSQL_PASSWORD=wordpress
# allowed traffic
policies:
  - type: east-west
    source: wp
    destination: db
    protocol: tcp
    dest_port: 3306
    comment: "wp->db 3306/tcp: Allow Database Traffic"
```

### Log Enrichment with NFLOG + ulogd2 + Condenser
Raind provides built-in visibility into container traffic:

- Traffic logs
- DNS logs
- Metrics

All logs map source/destination to container names for easier inspection.

#### SIEM Integration
Example visualization when using [Wazuh](https://wazuh.com/):

- Traffic logs
![netflow_timeline](assets/siem/netflow_timeline.png)
![netflow](assets/siem/netflow.png)

- DNS logs
![dns](assets/siem/dns.png)

- Metrics
![metrics_timeline](assets/siem/metrics_timeline.png)
![metrics](assets/siem/metrics.png)

With SIEM integration, you can detect abnormal container behavior such as traffic spikes, resource saturation, or OOM.

## Architecture Overview
- `Raind-CLI` operates `Condenser` via REST/WebSocket
- `Condenser` manages container state, networks, and policies as the SSOT
- `Droplet` builds namespaces/cgroups/capabilities based on OCI config and executes containers

Design diagrams and details are summarized in the [Design Document](docs/en/design.md).

## Requirements
- Linux (kernel with namespace/cgroup support)
- Go 1.25+ (CLI can run with 1.24+)
- `sudo` privileges
- `iptables` available
- `ulogd2` (for NFLOG collection)

When verifying Droplet alone, Docker may be required for pulling images, etc.  
Network log collection and enrichment assumes `ulogd2` is available.

## Setup (Meta-Repo)
1. Initialize submodules
```
git submodule update --init --recursive
```
2. Dependency check
```
make bootstrap
```
3. Build
```
make build
```
4. Install (optional)
```
sudo make install
```
5. Install + systemd setup at once (optional)
```
sudo make all
```

Additional package installation and ulogd2 setup are also required.  
See [Raind Install](docs/en/install.md) for details.

## Example Run
Start Condenser first, then use the CLI.
```
sudo ./runtime_stack/condenser/bin/condenser
sudo ./runtime_stack/raind-cli/bin/raind container ls
```

To run as a systemd service:
```
sudo make enable-service
```

## CLI Usage
For detailed commands, see the [Command List](docs/en/command_list.md).  
Examples:
```
sudo raind container run -t alpine:latest
sudo raind policy ls
sudo raind logs netflow --json
```

## What You Can Do (Summary)
Raind features are primarily provided by Condenser and operated via Raind-CLI.
- Create/start/stop/remove/attach/exec containers and get logs
- Image pull/build/list/remove
- Network create/remove/list
- Policy add/remove/commit/revert/change mode
- Bottle (Compose-like) multi-container orchestration
- Pod/ReplicaSet/Service (Kubernetes-like resources)

## Documents
- [Design Document](docs/en/design.md): Zero Trust design, iptables design, log design
- [Command List](docs/en/command_list.md): CLI command list
- [Bottle Usage](docs/en/bottle.md): Bottle orchestration usage
- [Install](docs/en/install.md): Packages, ulogd2 config, initial setup
- [Pod](docs/en/pod.md): Pod usage
- [ReplicaSet](docs/en/replicaset.md): ReplicaSet usage
- [Service](docs/en/service.md): Service usage

## Status
Raind is under active development as a PoC.  
APIs and behavior may change.
