# Raind Documentation

Raind is an experimental runtime stack that combines Docker-style container workflows and Kubernetes-inspired workload resources in one local runtime.

This documentation is organized by reader intent:

| Section | Use it when you want to... |
|---|---|
| [Getting started](getting-started/quickstart.md) | Install Raind, start the runtime, and run the first container. |
| [Guides](guides/containers.md) | Use Raind features in day-to-day workflows. |
| [Architecture](architecture/runtime-stack.md) | Understand how the CLI, Condenser, and Droplet fit together. |
| [Reference](reference/cli.md) | Check exact command shapes, manifests, and runtime behavior. |
| [Resource reference](reference/resources/) | See Kubernetes-style resource fields supported by Raind. |

## Recommended reading path

1. [Runtime stack](architecture/runtime-stack.md)
2. [Installation](getting-started/installation.md)
3. [Quickstart](getting-started/quickstart.md)
4. [Container workflows](guides/containers.md)
5. [Rootless containers](guides/rootless-containers.md)
6. [Rootless implementation](architecture/rootless.md)

## Current scope

Raind currently focuses on:

- Docker-like single-container workflows
- local multi-container Bottle workflows
- Kubernetes-style Namespace, Pod, ReplicaSet, Deployment, Service, and Ingress resources
- runtime-managed container networking and policy
- container and Pod traffic observation
- rootless standalone containers with explicit ID mapping modes

Raind is still under active development. Some Docker/Kubernetes-compatible syntax may be parsed before every behavior is fully implemented.
