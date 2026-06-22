# Promote Workflow

`raind promote` is Raind's workflow for validating an application as it moves from containers to Compose-style multi-service configuration and then to Kubernetes-style resources.

```text
single containers
  -> Bottle / Compose draft
  -> Kubernetes-style resource drafts
```

Promote is runtime-aware. It does not simply convert one static file into another. It starts from something that actually ran in Raind, reads the observed runtime state, and generates reviewable drafts for the next stage.

The recommended entry point is **Promote Strategy**. A strategy file describes the containers to start, the traffic policies to apply, and the checks that must pass at each stage.

Generated files are drafts. They are useful starting points for review, not production-ready configuration.

Default output layout:

```text
raind_promote/
  bottle/
    bottle.yaml
    REVIEW_BOTTLE.md
  compose/
    compose.yaml
  resources/
    00-namespace.yaml
    01-configmap.yaml
    02-secret.example.yaml
    03-pvcs.yaml
    04-deployments.yaml
    05-services.yaml
    06-ingress.yaml
    07-networkpolicies.yaml
    REVIEW.md
    all.yaml
```

Only applicable resource files are generated. For example, `03-pvcs.yaml` is omitted when no persistent volume claims are needed, and `06-ingress.yaml` is generated only when an ingress host is provided.

Promote Strategy uses unmasked temporary files under `.raind_promote_strategy/` while validating later stages. After a stage passes, it writes masked, reviewable drafts under `raind_promote/`.

## Hands-on: WordPress + MySQL

This section walks through a complete Promote Strategy flow that anyone can run locally with Raind.

The strategy will:

1. create MySQL and WordPress containers
2. allow WordPress to connect to MySQL
3. check that both containers are running
4. check that WordPress responds over HTTP
5. promote the running containers to Bottle and Compose drafts
6. start the generated Bottle and run the same checks
7. promote the running Bottle to Kubernetes-style resources
8. apply the generated resources and validate WordPress again
9. clean up the temporary runtime objects

### 1. Create `raind-strategy.yaml`

Create a file named `raind-strategy.yaml` in your working directory:

```yaml
apiVersion: raind.io/v1alpha1
kind: PromoteStrategy

metadata:
  name: wordpress-stack

source:
  mode: create
  containers:
    - name: mysql
      image: mysql:8
      env:
        MYSQL_ROOT_PASSWORD: root-password
        MYSQL_DATABASE: wordpress-db
        MYSQL_USER: wordpress-user
        MYSQL_PASSWORD: wordpress-password

    - name: wordpress
      image: wordpress:latest
      env:
        WORDPRESS_DB_HOST: mysql
        WORDPRESS_DB_NAME: wordpress-db
        WORDPRESS_DB_USER: wordpress-user
        WORDPRESS_DB_PASSWORD: wordpress-password
      ports:
        - "9850:80"
      dependsOn:
        - mysql

  policies:
    - type: ew
      source: wordpress
      destination: mysql
      protocol: tcp
      destPort: 3306
      comment: allow wordpress database access

stages:
  container:
    checks:
      runtime:
        - name: mysql-running
          type: containerStatus
          target: mysql
          expect:
            state: running
          timeout: 60s
          interval: 2s

        - name: wordpress-running
          type: containerStatus
          target: wordpress
          expect:
            state: running
          timeout: 60s
          interval: 2s

      application:
        - name: wordpress-http
          type: http
          target: http://127.0.0.1:9850/wp-admin/login.php
          expect:
            status: 200
          timeout: 90s
          interval: 2s

  bottle:
    checks:
      runtime:
        - name: bottle-running
          type: bottleStatus
          target: wordpress-stack
          timeout: 60s
          interval: 2s

      application:
        - name: wordpress-http
          type: http
          target: http://127.0.0.1:9850/wp-admin/login.php
          expect:
            status: 200
          timeout: 90s
          interval: 2s

  resources:
    checks:
      application:
        - name: wordpress-http
          type: http
          target: http://127.0.0.1:9850/wp-admin/login.php
          expect:
            status: 200
          timeout: 90s
          interval: 2s
```

`stages.bottle` and `stages.resources` control the promotion path. You do not need to write `promote.to`: Raind infers the next target from the stages that exist.

### 2. Validate the strategy file

```sh
raind promote strategy --dry-run
```

`--dry-run` parses and validates the YAML without creating containers or applying resources.

### 3. Run the full strategy

Make sure local port `9850` is free, then run:

```sh
raind promote strategy
```

Example output:

```text
Promote Strategy: wordpress-stack
[container] create::mysql ... ok
[container] create::wordpress ... ok
[container] runtime ... ok
[container] policies::wordpress-to-mysql-tcp:3306 ... ok
[container] policies-commit ... ok
[container] checks::runtime::mysql-running ... ok
[container] checks::runtime::wordpress-running ... ok
[container] checks::application::wordpress-http ... ok
[container] promote ... raind_promote/bottle/bottle.yaml
[container] policies-delete ... ok
[container] delete ... ok
[bottle] apply ... wordpress-stack
[bottle] checks::runtime::bottle-running ... ok
[bottle] checks::application::wordpress-http ... ok
[bottle] promote ... raind_promote/resources
[bottle] delete ... ok
[resources] apply ... .raind_promote_strategy/resources/all.yaml
[resources] checks::application::wordpress-http ... ok
[resources] delete ... ok
bottle draft: raind_promote/bottle/bottle.yaml
compose draft: raind_promote/compose/compose.yaml
resource drafts: raind_promote/resources
```

### 4. Review generated files

After the strategy passes, review the generated drafts:

```sh
cat raind_promote/compose/compose.yaml
cat raind_promote/bottle/bottle.yaml
cat raind_promote/resources/all.yaml
cat raind_promote/resources/REVIEW.md
```

Secret-like environment values are masked in reviewable outputs. Promote Strategy uses real values only in temporary internal files so that validation can work.

### 5. Stop at an intermediate stage

Use `--until` when you want to inspect generated drafts before continuing.

```sh
# Stop after generating Bottle and Compose drafts.
raind promote strategy --until bottle-draft

# Stop after generating resource drafts.
raind promote strategy --until resources-draft
```

Supported values are `container`, `bottle-draft`, `bottle`, and `resources-draft`.

## Sample patterns

### Container-only smoke test

Use only `stages.container` when you want Raind to create containers, run checks, and clean them up without generating downstream resources.

```yaml
apiVersion: raind.io/v1alpha1
kind: PromoteStrategy

metadata:
  name: nginx-smoke

source:
  mode: create
  containers:
    - name: nginx
      image: nginx:latest
      ports:
        - "9980:80"

stages:
  container:
    checks:
      application:
        - name: nginx-http
          type: http
          target: http://127.0.0.1:9980
          expect:
            status: 200
            bodyContains: "Welcome to nginx"
```

### Container to Bottle / Compose only

Define `stages.bottle` but omit `stages.resources` when you want to validate the generated Bottle and stop before Kubernetes-style resources.

```yaml
apiVersion: raind.io/v1alpha1
kind: PromoteStrategy

metadata:
  name: nginx-bottle

source:
  mode: create
  containers:
    - name: nginx
      image: nginx:latest
      ports:
        - "9980:80"

stages:
  container:
    checks:
      application:
        - name: nginx-container-http
          type: http
          target: http://127.0.0.1:9980

  bottle:
    checks:
      runtime:
        - name: bottle-running
          type: bottleStatus
          target: nginx-bottle
      application:
        - name: nginx-bottle-http
          type: http
          target: http://127.0.0.1:9980
```

This produces:

```text
raind_promote/bottle/bottle.yaml
raind_promote/bottle/REVIEW_BOTTLE.md
raind_promote/compose/compose.yaml
```

### Full application promotion with an Ingress draft

Use `--ingress-host` when you want the resource draft to include an Ingress for the first TCP service port.

```sh
raind promote strategy --ingress-host wordpress.raind.local
```

This can generate `raind_promote/resources/06-ingress.yaml` in addition to the other resource files.

### TCP readiness check

Use a `tcp` check when the application does not expose an HTTP endpoint.

```yaml
stages:
  container:
    checks:
      application:
        - name: redis-tcp
          type: tcp
          target: 127.0.0.1:6379
          timeout: 30s
          interval: 1s
```

### Manual Promote commands

You can also run Promote manually when debugging a single step.

```sh
# Promote running containers to Bottle and Compose drafts.
raind promote container mysql wordpress --to bottle

# Promote a Bottlefile to Kubernetes-style resource drafts.
raind promote bottle raind_promote/bottle/bottle.yaml --to resources --ingress-host app.raind.local
```

Manual `raind promote container` writes:

```text
raind_promote/bottle/bottle.yaml
raind_promote/bottle/REVIEW_BOTTLE.md
raind_promote/compose/compose.yaml
```

Manual `raind promote bottle` writes resource drafts under:

```text
raind_promote/resources/
```

## Strategy YAML reference

This section lists every YAML field supported by Promote Strategy in this release.

### Root object

| Field | Required | Type | Description |
| --- | --- | --- | --- |
| `apiVersion` | No | String | Informational. Use `raind.io/v1alpha1` in current strategy files. |
| `kind` | No | String | Must be empty or `PromoteStrategy` when set. |
| `metadata` | Yes | Object | Strategy metadata. |
| `metadata.name` | Yes | String | Strategy name. Also used as the Bottle name and default resource namespace. |
| `source` | Yes | Object | Source container and policy definition. |
| `containers` | No | List | Backward-compatible alias for `source.containers`. Prefer `source.containers`. |
| `stages` | No | Object | Stage definitions. Supported keys are `container`, `bottle`, and `resources`. |

### `source`

| Field | Required | Type | Description |
| --- | --- | --- | --- |
| `source.mode` | No | String | Source mode. Empty defaults to `create`. `create` is the only supported value in this release. |
| `source.containers` | Yes | List of objects | Containers that Strategy creates, checks, promotes, and then removes. |
| `source.policies` | No | List of objects | Temporary EW policies applied during the container stage. These policies are also captured into generated Bottle and resource drafts. |

### `source.containers[]`

| Field | Required | Type | Description |
| --- | --- | --- | --- |
| `name` | Yes | String | Container name. Checks and policies refer to this value. |
| `image` | Yes | String | Image reference, such as `nginx:latest`, `mysql:8`, or `wordpress:latest`. |
| `command` | No | String, list, or map | Command passed to the container create flow. Prefer list form for command arguments. |
| `network` | No | String | Network name passed to container creation. |
| `volume` | No | String, list, or map | Volume entries passed to container creation. |
| `mount` | No | String, list, or map | Additional mount entries. `volume` and `mount` are merged before creation. |
| `publish` | No | String, list, or map | Published ports, such as `9850:80`. |
| `ports` | No | String, list, or map | Published ports. `publish` and `ports` are merged before creation. |
| `device` | No | String, list, or map | Device entries passed to container creation. |
| `env` | No | String, list, or map | Environment variables. Map form becomes `KEY=value`; list form should use `KEY=value`. |
| `capAdd` | No | String, list, or map | Linux capabilities to add. |
| `capDrop` | No | String, list, or map | Linux capabilities to drop. |
| `securityProfile` | No | String | Security profile name passed to container creation. |
| `tty` | No | Boolean | Whether to allocate/start with TTY. Defaults to `false`. |
| `dependsOn` | No | String, list, or map | Dependency metadata captured into generated drafts. Container creation still follows the order in `source.containers`. |

String-list fields accept scalar, list, and map forms:

```yaml
# scalar form
env: MYSQL_DATABASE=wordpress-db

# list form
env:
  - MYSQL_DATABASE=wordpress-db
  - MYSQL_USER=wordpress-user

# map form
env:
  MYSQL_DATABASE: wordpress-db
  MYSQL_USER: wordpress-user
```

For `command`, list form is recommended:

```yaml
command:
  - nginx
  - -g
  - daemon off;
```

### `source.policies[]`

Promote Strategy currently supports temporary east-west policies only.

| Field | Required | Type | Description |
| --- | --- | --- | --- |
| `type` | No | String | Empty defaults to `ew`. `ew` is the only supported value in this release. |
| `source` | Yes | String | Source container name, such as `wordpress`. |
| `destination` | Yes | String | Destination container name, such as `mysql`. |
| `protocol` | No | String | Empty defaults to `tcp`. Usually `tcp` or `udp`. |
| `destPort` | Yes | Integer | Destination port. Prefer this field in new strategy files. |
| `dport` | Yes if `destPort` is omitted | Integer | Backward-compatible alias for `destPort`. When both are set, `destPort` wins. |
| `comment` | No | String | Optional policy comment. |

Example:

```yaml
source:
  policies:
    - type: ew
      source: wordpress
      destination: mysql
      protocol: tcp
      destPort: 3306
      comment: allow wordpress to reach mysql
```

### Stage lifecycle

| Stage | Lifecycle |
| --- | --- |
| `stages.container` | Create and start `source.containers`, wait for container runtime state, apply `source.policies`, run checks, promote to Bottle and Compose drafts when `stages.bottle` exists, remove temporary policies, then delete the source containers. |
| `stages.bottle` | Apply and start the generated Bottle, run checks, promote the running Bottle to resource drafts when `stages.resources` exists, then delete the Bottle. |
| `stages.resources` | Apply the generated `all.yaml`, run checks, then delete the applied resources. |

Promotion targets are inferred from stage presence:

- if `stages.bottle` exists, the container stage promotes to Bottle and Compose drafts
- if `stages.resources` exists, the Bottle stage promotes to resource drafts
- `stages.resources` requires `stages.bottle`

### `stages.container`

| Field | Required | Type | Description |
| --- | --- | --- | --- |
| `checks.runtime` | No | List of checks | Runtime checks for the container stage. Usually `containerStatus`. |
| `checks.application` | No | List of checks | Application checks for the container stage. Usually `http` or `tcp`. |
| `healthChecks` | No | List of checks | Backward-compatible check list. Prefer `checks.runtime` and `checks.application`. |

### `stages.bottle`

| Field | Required | Type | Description |
| --- | --- | --- | --- |
| `apply.file` | No | String | Bottlefile to apply. Defaults to the internal temporary Bottlefile generated by Strategy. |
| `checks.runtime` | No | List of checks | Runtime checks for the Bottle stage. Usually `bottleStatus`. |
| `checks.application` | No | List of checks | Application checks for the Bottle stage. Usually `http` or `tcp`. |
| `healthChecks` | No | List of checks | Backward-compatible check list. Prefer `checks.runtime` and `checks.application`. |

### `stages.resources`

| Field | Required | Type | Description |
| --- | --- | --- | --- |
| `apply.file` | No | String | Resource manifest to apply. Defaults to the internal temporary `all.yaml` generated by Strategy. |
| `apply.path` | No | String | Directory containing `all.yaml`. When set, Strategy applies `<apply.path>/all.yaml`. |
| `checks.runtime` | No | List of checks | Runtime checks for the resources stage. No resource-specific check type is currently implemented, so use only supported check types. |
| `checks.application` | No | List of checks | Application checks for the resources stage. Usually `http` or `tcp`. |
| `healthChecks` | No | List of checks | Backward-compatible check list. Prefer `checks.runtime` and `checks.application`. |

### `checks.runtime[]`, `checks.application[]`, and `healthChecks[]`

All check entries share the same base shape:

```yaml
- name: wordpress-http
  type: http
  target: http://127.0.0.1:9850/wp-admin/login.php
  expect:
    status: 200
    bodyContains: "Log In"
  timeout: 90s
  interval: 2s
```

| Field | Required | Type | Description |
| --- | --- | --- | --- |
| `name` | No | String | Display name. If omitted, Strategy uses `target`; if `target` is also empty, it uses `type`. |
| `type` | No | String | Check type. Empty defaults to `http`. Supported values are `http`, `tcp`, `containerStatus`, `container-status`, `bottleStatus`, and `bottle-status`. Matching is case-insensitive. |
| `target` | Yes | String | Type-specific target. URL for `http`, `host:port` for `tcp`, container name for `containerStatus`, or Bottle name for `bottleStatus`. |
| `expect` | No | Object | Type-specific expected result. Unsupported fields are ignored. |
| `expect.state` | No | String | Expected container state for `containerStatus`. Defaults to `running`. |
| `expect.status` | No | Integer | Expected HTTP status for `http`. Defaults to `200`. |
| `expect.bodyContains` | No | String | Substring that must be present in the HTTP response body. |
| `timeout` | No | Duration | Overall retry timeout. Defaults to `60s`. Go duration syntax is supported, such as `500ms`, `2s`, or `1m`. |
| `interval` | No | Duration | Retry interval. Defaults to `2s`. For HTTP/TCP checks, this is also used as the per-request/per-dial timeout. |

### Check types

#### `http`

Sends an HTTP `GET` request to `target`.

```yaml
- name: app-http
  type: http
  target: http://127.0.0.1:9850/wp-admin/login.php
  expect:
    status: 200
    bodyContains: "Log In"
```

Supported fields: `target`, `expect.status`, `expect.bodyContains`, `timeout`, `interval`.

#### `tcp`

Attempts to open a TCP connection to `target`.

```yaml
- name: mysql-tcp
  type: tcp
  target: 127.0.0.1:3306
```

Supported fields: `target`, `timeout`, `interval`.

#### `containerStatus`

Checks a Raind container state by name or ID.

```yaml
- name: mysql-running
  type: containerStatus
  target: mysql
  expect:
    state: running
```

Supported fields: `target`, `expect.state`, `timeout`, `interval`.

#### `bottleStatus`

Checks that a Bottle can be fetched as a running Bottle.

```yaml
- name: bottle-running
  type: bottleStatus
  target: wordpress-stack
```

Supported fields: `target`, `timeout`, `interval`.

## Strategy command options

| Option | Default | Description |
| --- | --- | --- |
| `-f`, `--file` | `raind-strategy.yaml` | Strategy YAML file to read. |
| `--dry-run` | `false` | Parse and validate the strategy without creating containers, applying Bottles, or applying resources. |
| `--until` | empty | Stop after a stage. Supported values are `container`, `bottle-draft`, `bottle`, and `resources-draft`. |
| `--namespace` | `metadata.name` | Namespace used while generating resource drafts. |
| `--ingress-host` | empty | Generate an Ingress draft for the first TCP service port using this host. |

## Notes on generated drafts

- `raind_promote/compose/compose.yaml` is Docker Compose-style and omits Raind-only `policies` fields.
- `raind_promote/bottle/bottle.yaml` may include Raind-specific fields such as policies.
- Resource secrets are written as examples with `<replace-me>` placeholders in reviewable outputs.
- Strategy-generated resource Services preserve host-published ports as externally reachable services. Internal-only services remain `ClusterIP`.
- Generated files are intentionally incomplete for production. Add deployment-specific settings such as readiness probes, storage details, jobs, and production-grade secrets before using them outside local validation.
