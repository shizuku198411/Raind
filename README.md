# Raind

<p>
  <img src="./assets/raind_icon.png" alt="Project Icon" width="150">
</p>

Raind is an experimental runtime stack that unifies Docker-style container management and Kubernetes-style Pod/resource management in a single runtime.

The goal of Raind is to make container-level and Pod-level application deployment testable, controllable, and observable through one consistent runtime path. Instead of treating single containers, multi-container application groups, and Kubernetes-like resources as separate operational layers, Raind manages them as related workload units on top of the same runtime foundation.

Raind is still under active development. Dockerfile parsing, image manifest handling, and Kubernetes manifest compatibility are being expanded incrementally, so some Docker/Kubernetes features may be parsed or partially supported before they are fully implemented.

## 1. Raind Concept

Raind is built around a layered runtime architecture:

```text
raind CLI
  -> condenser: high-level runtime API, image/resource/policy/log controller
    -> droplet: low-level OCI-style container runtime
```

`droplet` provides the low-level container runtime layer. It is responsible for OCI-style container lifecycle operations such as create, start, exec, stop, kill, delete, namespace setup, mounts, capabilities, cgroups, hooks, and runtime state.

`condenser` provides the high-level runtime layer. It manages images, containers, bottles, Kubernetes-style resources, networking, policy, logs, and state, then delegates low-level container execution to `droplet`.

`raind` is the user-facing CLI. It exposes container, image, resource, bottle, policy, network, and log workflows through one command surface.

## 2. Why Raind?

Modern container workflows are often split across different layers.

Docker-style tools are convenient for building images and running individual containers, but they usually stop at the container boundary. Kubernetes-style systems are powerful for Pod-level deployment and service-oriented workloads, but they introduce a larger orchestration model even when the user only wants to test a workload locally.

At the same time, policy and communication visibility are often outside the direct runtime path. Container-to-container and Pod-to-Pod traffic can become difficult to understand because deployment, policy, and logs are managed through different tools.

Raind is designed to collapse those concerns into one runtime:

- Run and inspect individual containers.
- Group multiple containers into a local application unit.
- Apply Kubernetes-style manifests for Pod-level resources.
- Reconcile workload resources such as ReplicaSet and Deployment.
- Expose Service-level traffic handling for matching Pods.
- Apply runtime-managed network policy.
- Observe container and Pod communication from runtime logs.

This makes Raind useful as a local testing and deployment runtime for applications that may move between simple containers, multi-container groups, and Kubernetes-style workloads.

## 3. Unifying Docker-Style Container Management and Kubernetes-Style Pod Management

Raind provides Docker-like and Kubernetes-like workflows through one runtime interface.

For container-level workflows, Raind can run individual containers, publish ports, mount volumes, pass environment variables, execute commands, read logs, and manage lifecycle state.

```sh
raind container run --name web -p 8080:80 nginx:latest
raind container exec web /bin/sh
raind container logs web
raind container stop web
raind container rm web
```

For Pod/resource-level workflows, Raind can apply Kubernetes-style YAML manifests and manage resources such as Pod, ReplicaSet, Deployment, and Service.

```sh
raind resource apply -f app.yaml
raind resource pod ls
raind resource replicaset ls
raind resource deployment ls
raind resource service ls
```

The important point is that both paths are handled by the same runtime stack. A container started directly through `raind container` and a container created as part of a Pod are ultimately managed through the same low-level runtime foundation.

Raind also includes Docker/Kubernetes-compatible parsing paths for image builds and manifests. Dockerfile support and Kubernetes resource compatibility are being expanded progressively, with the goal of making existing container and manifest workflows usable inside the Raind runtime model.

## 4. Management Units

Raind organizes workloads into three main management units:

1. `container`: a single runnable container.
2. `bottle`: a local multi-container application group.
3. `resource`: Kubernetes-style resources such as Pod, ReplicaSet, Deployment, and Service.

Each unit has a different scope, but they are intended to share the same runtime concepts: image handling, container lifecycle, networking, policy, logging, and state management.

### 4.1. Container

A container is the smallest runnable unit in Raind.

Container management is intended for Docker-like workflows: start one container, inspect it, execute commands in it, read logs, publish ports, mount volumes, and remove it when finished.

Typical container operations include:

```sh
raind container run --name web -p 8080:80 nginx:latest
raind container ls
raind container exec web /bin/sh
raind container logs web
raind container stop web
raind container rm web
```

Containers are executed by `droplet`, while `condenser` manages higher-level state, image resolution, network configuration, and policy/log integration.

This makes the container unit suitable for:

- Single-container application testing.
- Image validation.
- Runtime behavior checks.
- Low-level container lifecycle testing.
- Simple local services.

### 4.2. Bottle

A bottle is Raind's local multi-container application unit.

It is designed for applications that are larger than one container but do not necessarily need a full Kubernetes-style manifest. A bottle can describe multiple services, dependencies, ports, mounts, environment variables, and policies in one YAML definition.

Example workflows include:

```sh
raind bottle create -f bottle.yaml
raind bottle start wordpress
raind bottle show wordpress
raind bottle stop wordpress
```

Bottle is useful for testing application stacks that need explicit relationships between containers, such as a frontend and backend, an application and database, or multiple internal services.

A key part of the bottle model is communication control. Bottle workloads can be paired with east-west policy so that container-to-container traffic is visible and explicitly controlled by the runtime.

This makes the bottle unit suitable for:

- Multi-container application testing.
- Local service composition.
- Testing internal traffic rules.
- Verifying runtime policy behavior before moving to Pod-level deployment.

### 4.3. Resource: Pod / ReplicaSet / Deployment / Service

A resource is Raind's Kubernetes-style management unit.

Raind supports applying Kubernetes-compatible manifests for resources such as:

- `Pod`
- `ReplicaSet`
- `Deployment`
- `Service`
- `Namespace`

A Pod groups one or more containers into a shared runtime unit. In Raind, Pod containers can share namespaces through an infra container model, allowing Pod-like behavior to be tested through the local runtime.

ReplicaSet and Deployment resources provide reconciliation-oriented workload management. They allow the runtime to maintain a desired number of Pod replicas and update workload state based on resource definitions.

Service resources provide L4 traffic handling for matching Pods, allowing Pod-level workloads to be reached through a stable service abstraction.

Typical resource operations include:

```sh
raind resource apply -f app.yaml
raind resource pod ls
raind resource deployment ls
raind resource service ls
raind resource rm -f app.yaml
```

This resource unit is suitable for:

- Testing Kubernetes-style workload manifests locally.
- Validating Pod composition.
- Testing ReplicaSet and Deployment behavior.
- Testing Service-based traffic routing.
- Moving from container-level tests to Pod-level deployment tests without leaving the Raind runtime.

## 5. Visibility: Policy and Netflow

Raind includes policy and log features to make container and Pod communication more transparent.

Runtime-managed policy allows Raind to control traffic between containers, bottles, namespaces, and resource-managed workloads. Policy support includes east-west container-to-container policy and namespace egress observation/enforcement modes.

Policy types include:

- `ew`: east-west container-to-container policy.
- `ns-obs`: namespace egress observation policy.
- `ns-enf`: namespace egress enforcement policy.

Example policy workflow:

```sh
raind security policy add --type ew \
  --source frontend \
  --destination backend \
  --protocol tcp \
  --dport 8080 \
  --comment 'allow frontend to backend'

raind security policy commit
```

Raind also records network flow logs so communication can be inspected from the runtime itself.

```sh
raind logs netflow --line 50
raind logs netflow --json
raind logs netflow -t web
```

Netflow logs can be enriched with runtime metadata such as container ID, container name, IP address, interface, and identity information when available.

Together, policy and netflow are intended to solve a common problem in local container and Pod testing: the workload may start correctly, but it is still hard to see which component is talking to which other component and whether that communication should be allowed.

Raind makes communication control and communication visibility part of the runtime itself.

## More Details & Information

read [docs](./docs/) for checking more details, information, installation and others.

## License

This project is licensed under the terms in [LICENSE](./LICENSE).
