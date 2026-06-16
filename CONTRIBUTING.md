# Contributing to Raind

Thank you for your interest in contributing to Raind.

Raind is an experimental container runtime and lightweight workload orchestration project for Linux. It includes security-sensitive code paths such as namespace creation, cgroup management, mount/rootfs setup, AppArmor/seccomp handling, network namespace setup, iptables/nftables integration, and rootless container execution. Contributions are welcome, but changes in these areas should be made carefully and with tests whenever possible.

## Project status

Raind is currently under active development and should be treated as a developer-preview project. APIs, command behavior, runtime layout, and resource schemas may still change between releases.

The current focus areas are:

- container runtime stability
- rootless container execution
- image and Dockerfile workflows
- Kubernetes-style resource workflows
- networking, policy, and traffic observability
- documentation, test coverage, and release maturity

## Ways to contribute

You can help by:

- reporting bugs with clear reproduction steps
- improving documentation and examples
- adding tests for existing behavior
- fixing small CLI, documentation, or runtime issues
- proposing design changes through issues before opening large PRs
- testing Raind on different Linux distributions and architectures
- improving rootless, networking, policy, and workload-resource behavior

## Security issues

Do not report suspected vulnerabilities in public issues, pull requests, or discussions.

Please use GitHub Private Vulnerability Reporting from the repository's **Security** tab. See [`SECURITY.md`](SECURITY.md) for details.

Security-sensitive examples include, but are not limited to:

- container escape or host filesystem access beyond intended mounts
- namespace, cgroup, mount, capability, seccomp, or AppArmor bypass
- certificate, token, or private-key disclosure
- unintended remote access to the management API
- network policy bypass
- rootless container isolation issues

## Before opening an issue

Before opening a bug report, please check:

1. whether the issue already exists
2. whether the behavior is documented in `docs/`
3. whether the latest release or `main` branch has already changed the behavior
4. whether the issue involves a security vulnerability, in which case use private reporting instead

For runtime issues, please include logs when possible:

```sh
raind --version
raind container ls
raind network ls
raind image ls
sudo systemctl status raind-daemon.service --no-pager
sudo journalctl -u raind-daemon.service --no-pager -n 200
sudo tail -200 /etc/raind/log/droplet_audit.log 2>/dev/null || true
```

For snap installations, include:

```sh
snap list raind
snap services raind
sudo snap logs raind.condenser -n 200
```

## Development environment

The recommended development and test environment is Canonical Workshop using `workshop.yaml`.

```sh
workshop run raind-dev -- test-unit
workshop run raind-dev -- test-integ
workshop run raind-dev -- test-e2e
```

You can also open a development shell:

```sh
workshop shell raind-dev
```

Manual setup inside the Workshop environment:

```sh
workshop run raind-dev -- dev-setup
raind image ls
raind container ls
raind network ls
```

Cleanup:

```sh
workshop run raind-dev -- dev-cleanup
```

## Building locally

Build all components:

```sh
./scripts/build.sh build
```

Install binaries and enable the systemd service on a development host:

```sh
sudo ./scripts/build.sh all
```

This installs the main binaries:

- `raind`
- `condenser`
- `condenser-hook-agent`
- `droplet`

It also creates and starts `raind-daemon.service` by default.

> Be careful when running Raind directly on your host. Integration and e2e tests can create and remove network devices, iptables/nftables rules, cgroups, runtime state under `/etc/raind`, and logs under `/var/log/raind`.

## Testing

Run unit tests:

```sh
workshop run raind-dev -- test-unit
```

Run component-focused tests:

```sh
workshop run raind-dev -- test-droplet
workshop run raind-dev -- test-condenser
workshop run raind-dev -- test-raind
```

Run integration tests:

```sh
workshop run raind-dev -- test-integ
```

Run e2e tests:

```sh
workshop run raind-dev -- test-e2e
```

If you are changing low-level runtime behavior, please run the most relevant component tests and, when possible, the e2e test suite.

## Areas that need extra care

Please be especially careful when changing:

- namespace setup
- `pivot_root`, mount, bind mount, and overlay behavior
- rootless UID/GID mappings
- `exec` / `exec-shim` behavior
- AppArmor or seccomp handling
- Linux capabilities
- cgroup v2 behavior
- veth, bridge, route, DNS, iptables/nftables setup
- certificate generation and API authentication
- cleanup logic for runtime state, interfaces, cgroups, and iptables chains

For these areas, include a clear explanation of the runtime impact in the PR description.

## Pull request guidelines

Before opening a PR:

1. keep the change focused
2. explain the problem and the solution
3. include tests or explain why tests are not practical
4. update documentation when CLI behavior, runtime behavior, or configuration changes
5. avoid mixing refactoring and behavior changes in the same PR when possible
6. mention any host-level side effects, such as cgroup, mount, or network changes

PRs that affect security-sensitive code paths may need more review time.

## Commit style

There is no strict commit message format yet. Please use clear, action-oriented commit messages, for example:

```text
fix exec namespace entry for rootless containers
add rootless login-root documentation
update policy command reference
```

## Documentation changes

Documentation lives under `docs/`.

When adding or changing functionality, please update the relevant docs:

- `docs/guides/` for user-facing workflows
- `docs/architecture/` for design and implementation details
- `docs/reference/` for CLI, schema, and runtime-layout references
- `README.md` for high-level project overview changes

## Compatibility

Raind is still evolving, so compatibility is best-effort. Please call out possible breaking changes in PR descriptions.

Breaking changes include:

- CLI flag or command changes
- manifest schema changes
- runtime state layout changes
- image cache format changes
- certificate or API authentication changes
- network, policy, or rootless behavior changes

## License

By contributing to Raind, you agree that your contributions are provided under the same license as the project, currently MIT. See [`LICENSE`](LICENSE).
