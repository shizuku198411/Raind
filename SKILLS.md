# SKILLS.md

## Workshop-based validation

Use Canonical Workshop for build and runtime validation in this repository.

The Workshop definition is `workshop.yaml`, and the project workshop name is:

```sh
raind-dev
```

Prefer Workshop commands over direct host execution for integration, e2e, and runtime checks. Low-level runtime tests may create namespaces, cgroups, network devices, iptables/nftables rules, runtime state, and logs.

## Canonical Workshop command pattern

Workshop actions are named commands defined under `actions:` in `workshop.yaml`.

Use this form:

```sh
workshop run raind-dev -- <action> [args...]
```

The `--` separator is intentional. It separates the workshop name from the action and forwards trailing arguments to the action.

Useful Workshop commands:

```sh
workshop actions raind-dev
workshop run raind-dev -- <action> [args...]
workshop shell raind-dev
```

If Workshop reports that the workshop is not ready, start it first:

```sh
workshop start raind-dev
```

If the local Codex sandbox reports a `snap-confine` capability error when invoking `workshop`, rerun the same `workshop ...` command with escalated permissions. Do not replace Workshop validation with host-side integration or e2e commands.

## Available actions

The actions below are defined in `workshop.yaml`.

Build and install:

```sh
workshop run raind-dev -- bootstrap
workshop run raind-dev -- build
workshop run raind-dev -- install
workshop run raind-dev -- enable-service
workshop run raind-dev -- all
```

Unit and component tests:

```sh
workshop run raind-dev -- test-unit
workshop run raind-dev -- test-droplet
workshop run raind-dev -- test-condenser
workshop run raind-dev -- test-raind
```

Integration tests:

```sh
workshop run raind-dev -- test-droplet-integ
workshop run raind-dev -- test-condenser-integ
workshop run raind-dev -- test-raind-integ
workshop run raind-dev -- test-integ
```

E2E and OCI validation:

```sh
workshop run raind-dev -- test-e2e
workshop run raind-dev -- test-oci-runtime-tools
```

Manual development environment:

```sh
workshop run raind-dev -- dev-install
workshop run raind-dev -- dev-start
workshop run raind-dev -- dev-setup
workshop run raind-dev -- dev-cleanup
workshop run raind-dev -- dev-reset
```

## Choosing validation

For ordinary Go-only changes, start with:

```sh
workshop run raind-dev -- test-unit
```

For `internal/droplet` or low-level runtime changes, start with:

```sh
workshop run raind-dev -- test-droplet
```

When changes affect namespaces, cgroups, mounts, rootless behavior, shim/exec/attach, AppArmor, seccomp, or real runtime lifecycle behavior, also run the relevant integration suite:

```sh
workshop run raind-dev -- test-droplet-integ
```

For changes that affect the user-visible `raind` workflow, condenser API wiring, deployments, or cross-component behavior, run:

```sh
workshop run raind-dev -- test-integ
workshop run raind-dev -- test-e2e
```

For OCI runtime compatibility work, run:

```sh
workshop run raind-dev -- test-oci-runtime-tools
```

## Passing focused test arguments

Trailing arguments after the action are forwarded to the underlying script.

Example:

```sh
workshop run raind-dev -- test-droplet -run TestName
```

Before relying on focused arguments, check the target script to confirm how it forwards arguments.

## Manual runtime checks

To manually inspect behavior inside Workshop:

```sh
workshop run raind-dev -- dev-setup
workshop shell raind-dev
```

Inside the shell, run checks such as:

```sh
raind image ls
raind container ls
raind network ls
```

Clean up when finished:

```sh
workshop run raind-dev -- dev-cleanup
```

