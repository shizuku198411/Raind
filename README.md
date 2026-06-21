# Raind

<p align="center">
  <img src="./assets/raind_icon.png" alt="Raind" width="140">
</p>

<p align="center">
  <strong>Promote what actually ran: containers → compose → Kubernetes-style resources in one local runtime.</strong>
</p>

<p align="center">
  <a href="./LICENSE"><img alt="License: MIT" src="https://img.shields.io/badge/license-MIT-blue.svg"></a>
  <img alt="Status: experimental" src="https://img.shields.io/badge/status-experimental-orange.svg">
  <img alt="Go" src="https://img.shields.io/badge/go-1.25%2B-00ADD8.svg">
  <img alt="Platform" src="https://img.shields.io/badge/platform-linux-lightgrey.svg">
  <img alt="Promote" src="https://img.shields.io/badge/workflow-runtime--promote-6A5ACD.svg">
  <img alt="Resources" src="https://img.shields.io/badge/resources-Kubernetes--style-326CE5.svg">
</p>

> [!WARNING]
> Raind is experimental. It is not a Kubernetes distribution and does not aim to be fully Kubernetes-compatible. It focuses on a growing local subset that is useful for validating application deployment shape before a real cluster.

Raind is a local pre-Docker/pre-Kubernetes validation and promotion tool that runs applications as containers/Bottles, verifies basic runtime behavior, and generates reviewable Kubernetes-style deployment drafts.

## Why Raind?

Application deployment usually moves through several disconnected stages:

```text
container run
  → compose.yaml
  → kind / minikube / Kubernetes manifests
  → cluster validation
```

That workflow often turns into copy-and-edit drift. Ports, secrets, service names, volumes, and network rules are repeated across files, and generated manifests often do not know whether the application ever worked.

Raind takes a different path:

```text
actual run
  → observed runtime state
  → reviewable deployment draft
```

Raind gives you one local runtime for:

- single-container application tests(Docker-style),
- multi-service Bottle stacks(Compose-style),
- Deployments, Services, Ingress, PVCs, Secrets, ConfigMaps, and NetworkPolicies(Kubernetes-style),
- runtime security policy and netflow logs,
- promotion from one stage to the next.

## Promote workflow

Raind's signature workflow is **Promote**.

Promote does not try to generate perfect production configuration. It generates useful, reviewable drafts from known runtime state and tells you what still needs review.

### 1. Test real containers (Docker-style)

Run the application the simplest way first.

```sh
raind container run --name mysql \
  -e MYSQL_ROOT_PASSWORD=root-password \
  -e MYSQL_DATABASE=wordpress-db \
  -e MYSQL_USER=wordpress-user \
  -e MYSQL_PASSWORD=wordpress-password \
  mysql:8.0

raind container run --name wordpress \
  -e WORDPRESS_DB_HOST=mysql \
  -e WORDPRESS_DB_NAME=wordpress-db \
  -e WORDPRESS_DB_USER=wordpress-user \
  -e WORDPRESS_DB_PASSWORD=wordpress-password \
  -p 9850:80 \
  wordpress:latest
```

Add the traffic relationship that actually worked.

```sh
raind security policy add --type ew \
  -s wordpress \
  -d mysql \
  -p tcp --dport 3306 \
  --comment "wordpress -> db 3306/tcp"

raind security policy commit
```

### 2. Promote containers to a Bottle (Compose-style)

`Bottle` is a multi-container application unit like Docker Compose.

Once the containers work, generate a Bottle draft from the running containers.

```sh
raind promote container wordpress mysql \
  --to bottle \
  --bottle-name wordpress \
  -o bottle/bottle.yaml
```

Raind preserves runtime intent such as images, commands, ports, dependencies, and security policy. Secret-like environment values are redacted into review comments.

```yaml
bottle:
  name: "wordpress"

services:
  mysql:
    image: "mysql:8.0"
    command:
      - "docker-entrypoint.sh"
      - "mysqld"
    # TODO: secret candidate redacted from container env: MYSQL_PASSWORD
    env:
      - "MYSQL_DATABASE=wordpress-db"
      - "MYSQL_USER=wordpress-user"

  wordpress:
    image: "wordpress:latest"
    command:
      - "docker-entrypoint.sh"
      - "apache2-foreground"
    env:
      - "WORDPRESS_DB_HOST=mysql"
      - "WORDPRESS_DB_NAME=wordpress-db"
      - "WORDPRESS_DB_USER=wordpress-user"
    ports:
      - "9850:80"
    depends_on:
      - "mysql"

policies:
  - type: "east-west"
    source: "wordpress"
    destination: "mysql"
    protocol: "tcp"
    dest_port: 3306
```

The promotion also writes `REVIEW_BOTTLE.md`, because the generated Bottlefile is a draft, not a claim of production readiness.

### 3. Run the Bottle

A Bottle is a local multi-service application shape.

```sh
cd bottle
raind bottle up
raind bottle show wordpress
```

`raind bottle up` is a convenience wrapper for creating the Bottle from `bottle.yaml` or `compose.yaml` and starting it.

### 4. Promote the running Bottle to resources (Kubernetes-style)

Raind promotes Bottles only after the Bottle is running. That keeps Promote runtime-aware instead of becoming a static file converter.

```sh
raind promote bottle bottle.yaml \
  --to resources \
  -o manifests \
  --ingress-host wordpress.raind.local
```

Generated output is ordered and reviewable:

```text
manifests/
  00-namespace.yaml
  01-configmap.yaml
  02-secret.example.yaml
  04-deployments.yaml
  05-services.yaml
  06-ingress.yaml
  07-networkpolicies.yaml
  REVIEW.md
  all.yaml
```

Example generated resource shape:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: "wordpress-config"
  namespace: "wordpress"
data:
  WORDPRESS_DB_HOST: "mysql.wordpress.svc.cluster.local"
  WORDPRESS_DB_NAME: "wordpress-db"
  WORDPRESS_DB_USER: "wordpress-user"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: "wordpress"
  namespace: "wordpress"
spec:
  replicas: 1
  selector:
    matchLabels:
      app: "wordpress"
  template:
    metadata:
      labels:
        app: "wordpress"
    spec:
      containers:
        - name: "wordpress"
          image: "wordpress:latest"
          command:
            - "docker-entrypoint.sh"
            - "apache2-foreground"
          envFrom:
            - configMapRef:
                name: "wordpress-config"
            - secretRef:
                name: "wordpress-secret"
          ports:
            - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: "mysql"
  namespace: "wordpress"
spec:
  type: ClusterIP
  selector:
    app: "mysql"
  ports:
    - port: 3306
      targetPort: 3306
      protocol: "TCP"
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: "allow-wordpress-to-mysql"
  namespace: "wordpress"
spec:
  podSelector:
    matchLabels:
      app: "wordpress"
  egress:
    - to:
        - podSelector:
            matchLabels:
              app: "mysql"
      ports:
        - protocol: "TCP"
          port: 3306
```

Raind maps service references into Kubernetes-style ClusterIP DNS names, generates internal Services from observed/policy-based relationships, and keeps secret values in example Secret manifests with placeholders.

### 5. Apply and validate locally

```sh
raind resource apply -f manifests/all.yaml

raind resource get -n wordpress deploy
raind resource get -n wordpress service
raind resource get -n wordpress ingress
raind security policy ls --type ew
raind logs netflow
```

At this point the same application has moved through three local validation shapes:

```text
containers worked (Docker-style)
  → multi containers service worked (Compose-style)
  → resources worked (Kubernetes-style)
```

That is the Raind loop.

## Design principles

### Runtime-aware, not static translation

Promote starts from things Raind has actually run or observed.

### Reviewable, not magical

Generated files are drafts. Raind writes review reports and TODOs where production decisions are still required.

### Preserve intent

Names, images, commands, ports, service boundaries, and traffic relationships are kept whenever possible.

### Avoid leaking secrets

Secret-like environment variables are redacted from Bottle promotion and emitted as placeholder Secret examples for resource promotion.

## Core capabilities

### Containers

```sh
raind container run --name web -p 8080:80 nginx:latest
raind container exec web /bin/sh
raind container logs web
raind container stop web
raind container rm web
```

### Bottles

```sh
raind bottle up
raind bottle show wordpress
raind bottle down
```

Default file discovery:

1. `bottle.yaml`
2. `compose.yaml`
3. explicit `-f <path>`

### Kubernetes-style resources

Raind supports a growing local subset including:

- Namespace
- Pod
- Deployment
- ReplicaSet
- Service
- Ingress
- ConfigMap
- Secret
- PersistentVolumeClaim
- NetworkPolicy

```sh
raind resource apply -f manifests/all.yaml
raind resource get pod
raind resource get deploy
raind resource get service
raind resource delete -f manifests/all.yaml
```

### Network visibility and policy

Raind connects NetworkPolicy-style manifests and Bottle policies to runtime-managed east-west security policy.

```sh
raind security policy ls --type ew
raind logs netflow
```

Example:

```text
POLICY TYPE : East-West
CURRENT MODE: deny_by_default

FLAG  SRC CONTAINER  DST CONTAINER  PROTOCOL  DST PORT  ACTION
[*]   wordpress      mysql          tcp       3306      ALLOW
  >> DENY ALL EAST-WEST TRAFFIC <<
```

## What Raind is not

Raind is not a replacement for Docker, Podman, kind, minikube, or Kubernetes.

| Tool | Primary role |
|---|---|
| Docker / Podman | General container engine |
| kind / minikube | Run a real Kubernetes cluster locally |
| Kubernetes | Production-grade orchestration platform |
| Raind | Runtime-promoted local deployment validation |

Raind is for the space where you want to know:

- Did the container actually run?
- Did the multi-service stack actually communicate?
- What should the next deployment draft look like?
- Can the Kubernetes-style shape be validated locally before a real cluster?

## Demo

Raind Promote is the main story, but the same runtime can also be used for normal development checks.  
You can run containers, start Bottle stacks, and apply Kubernetes-style resources directly, then promote the working runtime state when the application shape is ready.

![Raind quickstart demo](./assets/demo/raind-quickstart.gif)

## Documentation

- [Documentation index](./docs/)
- [Promote workflow](./docs/guides/promote.md)
- [Bottles](./docs/guides/bottles.md)
- [Containers](./docs/guides/containers.md)
- [Architecture](./docs/architecture/)
- [Resource reference](./docs/resources/)
- [CLI reference](./docs/reference/cli.md)
- [Manifest schema](./docs/reference/manifest-schema.md)
- [Security](./SECURITY.md)
- [Contributing](./CONTRIBUTING.md)

## Project status

Raind is experimental and evolving quickly.

Current focus areas include:

- strengthening the Promote workflow,
- expanding the Kubernetes-style resource subset,
- improving validation and review reports,
- improving Service, DNS, and Ingress behavior,
- improving NetworkPolicy reconciliation and netflow visibility,
- hardening runtime state management and security-sensitive paths.

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](./CONTRIBUTING.md), [CODE_OF_CONDUCT.md](./CODE_OF_CONDUCT.md), and [SECURITY.md](./SECURITY.md).
