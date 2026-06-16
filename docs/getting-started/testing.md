# Testing

This project uses Canonical Workshop for automated tests and manual runtime checks. Do not run integration or e2e tests directly on the host machine unless you intentionally want to affect the host runtime environment.

## Workshop Project

The Workshop definition is `workshop.yaml`.

The current project name is:

```sh
raind-dev
```

## Unit Tests

Run all unit tests:

```sh
workshop run raind-dev -- test-unit
```

Run component-focused unit tests:

```sh
workshop run raind-dev -- test-droplet
workshop run raind-dev -- test-condenser
workshop run raind-dev -- test-raind
```

## Integration Tests

Integration tests verify the droplet, condenser, and raind CLI/API wiring without covering every real deployment workflow.

Run all integration tests:

```sh
workshop run raind-dev -- test-integ
```

Run each component integration suite:

```sh
workshop run raind-dev -- test-droplet-integ
workshop run raind-dev -- test-condenser-integ
workshop run raind-dev -- test-raind-integ
```

The integration scripts build the required binaries inside Workshop before running their checks.

## E2E Tests

E2E tests deploy real containers, bottles, and resources, then verify runtime behavior such as published HTTP access.

Run all e2e tests:

```sh
workshop run raind-dev -- test-e2e
```

The e2e script builds the required binaries inside Workshop before running its checks.

## Manual Runtime Verification

Install binaries into the Workshop environment:

```sh
workshop run raind-dev -- dev-install
```

Start the services:

```sh
workshop run raind-dev -- dev-start
```

Or install and start in one step:

```sh
workshop run raind-dev -- dev-setup
```

Open a shell inside Workshop:

```sh
workshop shell raind-dev
```

Then run CLI checks as a non-root user:

```sh
raind image ls
raind container ls
raind network ls
```

Reset the manual runtime environment:

```sh
workshop run raind-dev -- dev-reset
```

Clean up services, files, cgroups, and network artifacts:

```sh
workshop run raind-dev -- dev-cleanup
```

The cleanup script removes Workshop runtime artifacts such as `/etc/raind`, `/run/raind`, Workshop logs, installed binaries, `raind0`, `raindDns`, `rd_*` and `rns*` interfaces, RAIND iptables chains, and the Workshop systemd service files.
