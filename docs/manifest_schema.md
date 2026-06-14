# Manifest Schema

This document describes the Kubernetes-style resource manifests currently accepted by Raind through:

```sh
raind resource apply -f <manifest.yaml>
raind resource rm -f <manifest.yaml>
```

Raind accepts multi-document YAML separated by `---`. Empty documents are ignored.

> Scope: this document covers the resource manifest path handled by `raind resource apply/rm`. Bottle definition files use a separate schema and are not covered here.

## Supported resource kinds

| Kind | Accepted `apiVersion` | Apply support | Remove support | Notes |
| --- | --- | --- | --- | --- |
| `Namespace` | Usually `v1` | Yes | Yes | Creates/removes a Raind resource namespace. |
| `Pod` | Usually `v1` | Yes | Yes | Creates and starts a Pod immediately. |
| `ReplicaSet` | Usually `apps/v1` | Yes | Yes | Creates a template and ReplicaSet record; the controller reconciles Pods. |
| `Deployment` | Usually `apps/v1` | Yes | Yes | Creates a template and Deployment record; the controller creates/manages the backing ReplicaSet. |
| `Service` | Usually `v1` | Yes | Yes | Stores an L4 Service backed by selector-based Pod endpoints and iptables rules. |

`apiVersion` is parsed but not currently validated by version. Unsupported `kind` values fail with `unsupported kind`.

## Common behavior

### Multi-document apply

`raind resource apply -f` processes each YAML document in order. If a document fails during apply, previously applied resources in the same request are rolled back where possible.

Typical order:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: demo
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo-web
  namespace: demo
# ...
---
apiVersion: v1
kind: Service
metadata:
  name: demo-svc
  namespace: demo
# ...
```

### Default namespace

If `metadata.namespace` is omitted for `Pod`, `ReplicaSet`, `Deployment`, or `Service`, Raind uses `default`.

### Labels and selectors

Raind supports simple string-to-string labels:

```yaml
metadata:
  labels:
    app: demo
    tier: web
```

ReplicaSet and Deployment selectors use:

```yaml
spec:
  selector:
    matchLabels:
      app: demo
```

If `spec.selector.matchLabels` is omitted on ReplicaSet or Deployment, Raind uses the template labels as the selector. If a selector is provided, every selector key/value must exist in the template labels.

### Unsupported Kubernetes fields

Raind accepts a small Kubernetes-style subset. Unknown YAML fields are generally ignored by the current decoder, but they do not affect runtime behavior.

Notable unsupported or ignored fields include, but are not limited to:

- `spec.restartPolicy`
- `spec.nodeSelector`
- `spec.affinity`
- `spec.tolerations`
- `spec.initContainers`
- `spec.resources`
- `spec.livenessProbe` / `readinessProbe` / `startupProbe`
- `imagePullPolicy`
- named ports
- `env.valueFrom`
- ConfigMap / Secret volumes
- PVC / projected / emptyDir volumes
- Service `type`, `clusterIP`, `nodePort`, `sessionAffinity`
- Deployment rollout strategy, revision history, progress deadline, status

## `Namespace`

### Schema

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: <name>                 # required
  labels:                      # optional
    <key>: <value>
  annotations:                 # optional
    <key>: <value>
```

### Field notes

| Field | Required | Notes |
| --- | --- | --- |
| `metadata.name` | Yes | Namespace name. Raind namespace validation is stricter than generic Kubernetes names; lowercase names such as `demo` or `team-a` are safe. |
| `metadata.labels` | No | Stored on namespace creation. |
| `metadata.annotations` | No | Stored on namespace creation. |

A new namespace gets a namespace-scoped network unless it is created through the CLI with an explicit existing network. The manifest path currently creates the namespace with Raind's default namespace network behavior.

## `Pod`

### Schema

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: <name>                 # required
  namespace: <namespace>       # optional, default: default
  labels:                      # optional
    <key>: <value>
  annotations:                 # optional
    <key>: <value>
spec:
  volumes:                     # optional
    - name: <volume-name>
      hostPath:
        path: /absolute/host/path
        type: Directory        # optional; supported: Directory, DirectoryOrCreate
  containers:                  # optional by parser, but practical Pods should define at least one
    - name: <container-name>
      image: <image:tag>
      command: ["/bin/sh", "-c"]
      args: ["echo hello"]
      env:
        - name: KEY
          value: VALUE
      ports:
        - hostPort: 8080
          containerPort: 80
      mount:
        - /host/path:/container/path
      volumeMounts:
        - name: <volume-name>
          mountPath: /absolute/container/path
          readOnly: true
      securityContext:
        capabilities:
          add:
            - CAP_NET_ADMIN
          drop:
            - CAP_NET_RAW
      tty: true
```

### Pod metadata

| Field | Required | Notes |
| --- | --- | --- |
| `metadata.name` | Yes | Pod name. |
| `metadata.namespace` | No | Defaults to `default`. If the namespace has a namespace network, containers use it unless a container-level network is set internally by Raind. |
| `metadata.labels` | No | Used by Services and higher-level selectors. |
| `metadata.annotations` | No | Stored with the Pod. |

### Container fields

| Field | Required | Notes |
| --- | --- | --- |
| `name` | No at parser level | Container name. Recommended for all manifests. |
| `image` | No at parser level | Container image reference. Runtime creation needs a usable image. |
| `command` | No | Array form only. |
| `args` | No | Appended to `command`. If `command: ["/bin/sh", "-c"]` and `args: ["echo ok"]`, Raind stores `["/bin/sh", "-c", "echo ok"]` as the command vector. |
| `env` | No | Supports only `name` + `value`. Empty `name` entries are ignored. |
| `ports` | No | Supports `hostPort` + `containerPort`. Entries with `containerPort: 0` are ignored. Entries without `hostPort` are not published. |
| `mount` | No | Raind raw mount strings, such as `/host:/container` or `/host:/container:ro`. |
| `volumeMounts` | No | Kubernetes-style references to `spec.volumes`. |
| `securityContext.capabilities.add` | No | Passed to container capability add list. |
| `securityContext.capabilities.drop` | No | Passed to container capability drop list. |
| `tty` | No | Enables TTY behavior. |

### Environment variables

Supported:

```yaml
env:
  - name: APP_ENV
    value: dev
```

Unsupported:

```yaml
env:
  - name: POD_NAME
    valueFrom:
      fieldRef:
        fieldPath: metadata.name
```

### Ports

Supported:

```yaml
ports:
  - hostPort: 8080
    containerPort: 80
```

This becomes a Raind port mapping like `8080:80`.

`containerPort` without `hostPort` is currently not used as an exposed-only metadata field by the resource manifest path.

### Raw mounts

`mount` is a Raind extension for passing mount strings directly:

```yaml
mount:
  - /home/workshop/data:/data
  - /home/workshop/read-only:/ro-data:ro
```

For Kubernetes-style host directory mounts, prefer `volumes` + `volumeMounts`.

### Volumes and volumeMounts

Supported volume type: `hostPath` directory volumes.

```yaml
volumes:
  - name: data
    hostPath:
      path: /home/workshop/data
      type: Directory
containers:
  - name: app
    image: alpine:latest
    volumeMounts:
      - name: data
        mountPath: /data
        readOnly: true
```

Supported `hostPath.type` values:

| `hostPath.type` | Behavior |
| --- | --- |
| omitted / empty | Same as `Directory`. The path must already exist and be a directory. |
| `Directory` | The path must already exist and be a directory. |
| `DirectoryOrCreate` | Raind creates the directory with `0755` if missing, then verifies that it is a directory. |

Unsupported `hostPath.type` values currently fail apply:

- `File`
- `FileOrCreate`
- `Socket`
- `CharDevice`
- `BlockDevice`

Validation rules:

- `hostPath.path` must be absolute.
- `volumeMounts[].mountPath` must be absolute.
- duplicate `volumes[].name` values fail.
- `volumeMounts[].name` must refer to an existing volume.
- only `hostPath` volumes are supported.

`readOnly: true` appends `:ro` to the generated Raind mount string.

## `ReplicaSet`

### Schema

```yaml
apiVersion: apps/v1
kind: ReplicaSet
metadata:
  name: <name>                 # required
  namespace: <namespace>       # optional, default: default
spec:
  replicas: 2                  # optional, default: 1; explicit 0 is allowed
  selector:
    matchLabels:
      app: demo
  template:
    metadata:
      name: <pod-template-name> # optional; defaults to metadata.name
      labels:
        app: demo
      annotations:
        <key>: <value>
    spec:
      volumes:
        - name: data
          hostPath:
            path: /home/workshop/data
            type: Directory
      containers:
        - name: app
          image: alpine:latest
          command: ["/bin/sh", "-c"]
          args: ["sleep infinity"]
          env:
            - name: APP_ENV
              value: dev
          ports:
            - hostPort: 8080
              containerPort: 80
          volumeMounts:
            - name: data
              mountPath: /data
              readOnly: false
          securityContext:
            capabilities:
              add: ["CAP_NET_ADMIN"]
              drop: ["CAP_NET_RAW"]
          tty: true
```

### Field notes

| Field | Required | Notes |
| --- | --- | --- |
| `metadata.name` | Yes | ReplicaSet name. Also used as the template name if `spec.template.metadata.name` is omitted. |
| `metadata.namespace` | No | Defaults to `default`. This top-level namespace is the namespace used by the ReplicaSet. |
| `spec.replicas` | No | Defaults to `1`. Must be `>= 0`. Explicit `0` is preserved. |
| `spec.selector.matchLabels` | No | Defaults to template labels if omitted. If present, it must match template labels. |
| `spec.template.metadata.name` | No | Defaults to the ReplicaSet name. |
| `spec.template.metadata.labels` | No | Used by selector matching and Services. |
| `spec.template.metadata.annotations` | No | Stored on the Pod template. |
| `spec.template.spec` | Yes for useful workloads | Same container/volume subset as `Pod.spec`. |

## `Deployment`

### Schema

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: <name>                 # required
  namespace: <namespace>       # optional, default: default
spec:
  replicas: 2                  # optional, default: 1; explicit 0 is allowed
  selector:
    matchLabels:
      app: demo
  template:
    metadata:
      name: <pod-template-name> # optional; defaults to metadata.name
      labels:
        app: demo
      annotations:
        <key>: <value>
    spec:
      volumes:
        - name: data
          hostPath:
            path: /home/workshop/data
            type: DirectoryOrCreate
      containers:
        - name: app
          image: nginx:latest
          ports:
            - hostPort: 8080
              containerPort: 80
          volumeMounts:
            - name: data
              mountPath: /usr/share/nginx/html
              readOnly: true
          tty: true
```

### Field notes

Deployment uses the same template subset as ReplicaSet. Raind stores a Deployment and lets the controller create/manage the backing ReplicaSet.

| Field | Required | Notes |
| --- | --- | --- |
| `metadata.name` | Yes | Deployment name. |
| `metadata.namespace` | No | Defaults to `default`. |
| `spec.replicas` | No | Defaults to `1`. Must be `>= 0`. Explicit `0` is preserved. |
| `spec.selector.matchLabels` | No | Defaults to template labels if omitted. If present, it must match template labels. |
| `spec.template.metadata.name` | No | Defaults to the Deployment name. |
| `spec.template.metadata.labels` | No | Used by selector matching and Services. |
| `spec.template.metadata.annotations` | No | Stored on the Pod template. |
| `spec.template.spec` | Yes for useful workloads | Same container/volume subset as `Pod.spec`. |

Unsupported Deployment-specific Kubernetes fields are ignored by the current manifest decoder, including `strategy`, `minReadySeconds`, `revisionHistoryLimit`, and `progressDeadlineSeconds`.

## `Service`

### Schema

```yaml
apiVersion: v1
kind: Service
metadata:
  name: <name>                 # required by API handler
  namespace: <namespace>       # optional, default: default
  labels:                      # parsed but not currently stored by Service manifest conversion
    <key>: <value>
spec:
  selector:
    app: demo
  ports:
    - port: 8080
      targetPort: 80           # optional, defaults to port
      protocol: TCP            # optional, defaults to tcp
```

### Field notes

| Field | Required | Notes |
| --- | --- | --- |
| `metadata.name` | Yes | Service name. |
| `metadata.namespace` | No | Defaults to `default`. |
| `metadata.labels` | No | Parsed but not currently stored in the Service state. |
| `spec.selector` | No at parser level | Used by the Service controller to select Pods by label. Useful Services should define it. |
| `spec.ports[].port` | Yes for each active port | Entries with `port: 0` are ignored. |
| `spec.ports[].targetPort` | No | Defaults to `port` when omitted or `0`. |
| `spec.ports[].protocol` | No | Defaults to `tcp`. Controller lowercases protocol when applying iptables behavior. |

Service `type` is not currently part of the supported schema. Raind Services behave as Raind-managed L4 forwarding rules for selected Pods.

## Comprehensive example

The following multi-document manifest exercises the currently supported resource kinds and fields.

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: demo
  labels:
    team: platform
  annotations:
    description: demo namespace for Raind manifest schema
---
apiVersion: v1
kind: Pod
metadata:
  name: demo-pod
  namespace: demo
  labels:
    app: demo-pod
    tier: standalone
  annotations:
    raind.dev/example: pod
spec:
  volumes:
    - name: pod-data
      hostPath:
        path: /home/workshop/raind-demo/pod-data
        type: DirectoryOrCreate
  containers:
    - name: app
      image: alpine:latest
      command: ["/bin/sh", "-c"]
      args: ["echo pod started; sleep infinity"]
      env:
        - name: APP_ENV
          value: dev
        - name: LOG_LEVEL
          value: debug
      ports:
        - hostPort: 18080
          containerPort: 80
      mount:
        - /home/workshop/raind-demo/raw:/raw
      volumeMounts:
        - name: pod-data
          mountPath: /data
          readOnly: false
      securityContext:
        capabilities:
          add:
            - CAP_NET_ADMIN
          drop:
            - CAP_NET_RAW
      tty: true
---
apiVersion: apps/v1
kind: ReplicaSet
metadata:
  name: demo-rs
  namespace: demo
spec:
  replicas: 2
  selector:
    matchLabels:
      app: demo-rs
  template:
    metadata:
      name: demo-rs-pod
      labels:
        app: demo-rs
        tier: worker
      annotations:
        raind.dev/example: replicaset
    spec:
      volumes:
        - name: rs-data
          hostPath:
            path: /home/workshop/raind-demo/rs-data
            type: DirectoryOrCreate
      containers:
        - name: worker
          image: alpine:latest
          command: ["/bin/sh", "-c"]
          args: ["echo replicaset worker; sleep infinity"]
          env:
            - name: ROLE
              value: worker
          volumeMounts:
            - name: rs-data
              mountPath: /data
              readOnly: false
          securityContext:
            capabilities:
              add: []
              drop:
                - CAP_NET_RAW
          tty: true
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo-deploy
  namespace: demo
spec:
  replicas: 2
  selector:
    matchLabels:
      app: demo-web
  template:
    metadata:
      name: demo-web-pod
      labels:
        app: demo-web
        tier: web
      annotations:
        raind.dev/example: deployment
    spec:
      volumes:
        - name: web-data
          hostPath:
            path: /home/workshop/raind-demo/web-data
            type: DirectoryOrCreate
      containers:
        - name: web
          image: nginx:latest
          ports:
            - hostPort: 18081
              containerPort: 80
          volumeMounts:
            - name: web-data
              mountPath: /usr/share/nginx/html
              readOnly: true
          tty: true
---
apiVersion: v1
kind: Service
metadata:
  name: demo-web
  namespace: demo
  labels:
    app: demo-web
spec:
  selector:
    app: demo-web
  ports:
    - port: 8081
      targetPort: 80
      protocol: TCP
```

Before applying the example, make sure any `Directory` host paths already exist. Paths using `DirectoryOrCreate` are created by Raind if missing.

```sh
mkdir -p /home/workshop/raind-demo/raw
raind resource apply -f manifest.yaml
```

## Minimal examples

### Namespace

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: demo
```

### Pod

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: app
spec:
  containers:
    - name: app
      image: alpine:latest
      command: ["/bin/sh", "-c"]
      args: ["echo hello; sleep infinity"]
      tty: true
```

### ReplicaSet

```yaml
apiVersion: apps/v1
kind: ReplicaSet
metadata:
  name: app-rs
spec:
  replicas: 2
  selector:
    matchLabels:
      app: app-rs
  template:
    metadata:
      labels:
        app: app-rs
    spec:
      containers:
        - name: app
          image: alpine:latest
          command: ["/bin/sh", "-c"]
          args: ["sleep infinity"]
```

### Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app-deploy
spec:
  replicas: 2
  selector:
    matchLabels:
      app: app
  template:
    metadata:
      labels:
        app: app
    spec:
      containers:
        - name: web
          image: nginx:latest
```

### Service

```yaml
apiVersion: v1
kind: Service
metadata:
  name: app-svc
spec:
  selector:
    app: app
  ports:
    - port: 8080
      targetPort: 80
      protocol: TCP
```

## Implementation notes

- The current manifest support is intentionally a Kubernetes-style subset, not full Kubernetes API compatibility.
- Unknown fields may be accepted by YAML decoding but ignored by Raind.
- `raind resource rm -f` identifies resources by `kind`, `metadata.name`, and `metadata.namespace` where applicable.
- `Namespace` deletion is deferred until after other resources in the manifest are processed, so a manifest can remove workload resources before removing the namespace.
- For ReplicaSet and Deployment, top-level `metadata.namespace` is the namespace used by Raind. Template namespace is not used as the owning resource namespace.
