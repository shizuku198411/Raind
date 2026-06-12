# raind container runtime

<p>
  <img src="./assets/raind_icon.png" alt="Project Icon" width="150">
</p>

raind is an experimental container runtime stack written in Go. It is split into small components so the user-facing CLI can stay unprivileged while the runtime daemons keep the root-only operations isolated.

## Components

- `raind`: CLI for images, containers, networks, resources, policies, logs, and bottles.
- `condenser`: root daemon that exposes the management API and coordinates runtime operations.
- `droplet`: low-level runtime component for container lifecycle operations.
- `condenser-hook-agent`: hook-side helper used by condenser and droplet workflows.
- `raind-ui-gateway`: gateway process for UI access.

## Highlights

- Docker-style image pull flow with manifest and layer progress reporting.
- mTLS between CLI and daemon components.
- Non-root `raind` CLI access through the `raind` Unix group.
- Root-only daemon operations are handled by `condenser`, `droplet`, and the UI gateway service.
- Runtime security policy management for east-west and north-south traffic control.
- Workshop-based test and manual verification environment.
- Container, image, network, pod, ReplicaSet, service, policy, bottle, and netflow log command groups.

## Build

Download Go modules:

```sh
./scripts/build.sh bootstrap
```

Build all components:

```sh
./scripts/build.sh build
```

Built binaries are written to `bin/`:

- `bin/raind`
- `bin/condenser`
- `bin/condenser-hook-agent`
- `bin/droplet`
- `bin/raind-ui-gateway`

## Install Locally

Install the built binaries to `/usr/local/bin`:

```sh
sudo ./scripts/build.sh install
```

Create and start the condenser daemon service:

```sh
sudo ./scripts/build.sh enable-service
```

Optionally create and start the UI gateway service:

```sh
sudo ./scripts/build.sh enable-ui-gateway-service
```

Add your user to the `raind` group so the CLI can read the client certificate without running as root:

```sh
sudo usermod -aG raind "$USER"
```

Log out and back in, or start a new group session:

```sh
newgrp raind
```

You can also build, install, and enable the main condenser service in one command:

```sh
sudo ./scripts/build.sh all
```

## First Checks

```sh
raind --version
raind image ls
raind container ls
raind network ls
```

The CLI should be run as a non-root user in the `raind` group. If certificate paths need to be overridden, use:

- `RAIND_CA_CERT`
- `RAIND_CLIENT_CERT`
- `RAIND_CLIENT_KEY`

## Documentation

- [Testing with Workshop](docs/testing.md)
- [Command Reference](docs/commands.md)
- [Usage Examples](docs/examples.md)
