# Documentation Navigation

This file is a compact map for maintainers. User-facing entry points should link from [README.md](README.md).

```text
docs/
  README.md                         documentation entry point
  NAVIGATION.md                     maintainer map
  getting-started/
    installation.md                 build/install/runtime setup
    quickstart.md                   first commands
    testing.md                      Workshop and manual tests
  guides/
    containers.md                   day-to-day container workflows
    rootless-containers.md          rootless user guide
    dockerfile-build.md             image build guide
    promote.md                      container-to-Dripfile promotion workflow
  architecture/
    runtime-stack.md                raind/condenser/droplet overview
    container.md                    container lifecycle internals
    exec.md                         exec and exec-shim internals
    network.md                     bridge/IPAM/iptables/DNS/Ingress
    pod.md                         Pod infra-container model
    rootless.md                    rootless implementation details
  reference/
    cli.md                         command map
    manifest-schema.md             resource apply/rm schema
    runtime-layout.md              host-side files/directories
    rootless-modes.md              rootless mode reference
    resources/                     resource-specific references
  resources/                       compatibility path for older README links
  image/                           compatibility path for older README links
```

## Structure rules

- Put task-oriented material in `guides/`.
- Put implementation design and control/data-plane flow in `architecture/`.
- Put exact fields, command maps, and compatibility notes in `reference/`.
- Keep compatibility aliases for old paths until the root README is updated.
