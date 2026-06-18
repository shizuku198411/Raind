# Manifest Schema

This document describes the Kubernetes-style resource manifests currently accepted by Raind through:

```sh
raind resource apply -f <manifest.yaml>
raind resource rm -f <manifest.yaml>
```

Raind accepts multi-document YAML separated by `---`. Empty documents are ignored.

> Scope: this document covers the resource manifest path handled by `raind resource apply/rm`. Bottle definition files use a separate Raind-native schema and are not covered here.

## Supported resource kinds

| Kind | Accepted `apiVersion` | Apply support | Remove support | Notes |
|---|---|---:|---:|---|
| `Namespace` | Usually `v1` | Yes | Yes | Creates/removes a Raind resource namespace and its namespace network. |
| `Pod` | Usually `v1` | Yes | Yes | Creates and starts a Pod immediately. |
| `ReplicaSet` | Usually `apps/v1` | Yes | Yes | Stores a Pod template and desired replica count; the controller reconciles Pods. |
| `Deployment` | Usually `apps/v1` | Yes | Yes | Stores a Deployment; the controller creates/manages the backing ReplicaSet. |
| `Service` | Usually `v1` | Yes | Yes | Provides L4 forwarding to selected Pods. Supports `ClusterIP` and `NodePort`. |
| `Ingress` | Usually `networking.k8s.io/v1` | Yes | Yes | Provides HTTP/HTTPS host/path routing through the embedded condenser gateway. |

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
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: demo-ingress
  namespace: demo
# ...
```

### Default namespace

If `metadata.namespace` is omitted for `Pod`, `ReplicaSet`, `Deployment`, `Service`, or `Ingress`, Raind uses `default`.

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

Services use `spec.selector` directly:

```yaml
spec:
  selector:
    app: demo
```

### Unsupported Kubernetes fields

Raind accepts a focused Kubernetes-style subset. Unknown YAML fields are generally ignored by the current decoder, but they do not affect runtime behavior.

Notable unsupported or ignored fields include, but are not limited to:

- `spec.restartPolicy`
- `spec.nodeSelector`
- `spec.affinity`
- `spec.tolerations`
- `spec.initContainers`
- `spec.resources`
- `spec.livenessProbe` / `readinessProbe` / `startupProbe`
- `imagePullPolicy`
- named container or Service ports
- `env.valueFrom`
- ConfigMap / Secret volumes
- PVC / projected / emptyDir volumes
- Service `LoadBalancer`, `ExternalName`, `nodePort`, `sessionAffinity`
- Deployment rollout strategy, revision history, progress deadline, status
- Ingress annotations and controller-specific features
- Ingress `pathType: ImplementationSpecific`
- Ingress backend named service ports
- Ingress Secret-based TLS certificates

## `Namespace`

### Schema

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: <name>                 # required
  labels:                      # optional, currently ignored by namespace manifest conversion
    <key>: <value>
  annotations:                 # optional, currently ignored by namespace manifest conversion
    <key>: <value>
```

### Field notes

| Field | Required | Notes |
|---|---:|---|
| `metadata.name` | Yes | Namespace name. Lowercase names such as `demo` or `team-a` are safe. |
| `metadata.labels` | No | Accepted by YAML but not central to current namespace behavior. |
| `metadata.annotations` | No | Accepted by YAML but not central to current namespace behavior. |

A new resource namespace gets a namespace-scoped Raind network. Pods, ReplicaSets, Deployments, Services, and Ingresses in that namespace use the namespace boundary by default.

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
  hostUsers: false             # optional; false enables rootless Pod containers
  volumes:                     # optional
    - name: <volume-name>
      hostPath:
        path: /absolute/host/path
        type: Directory        # optional; supported: Directory, DirectoryOrCreate
  containers:
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
|---|---:|---|
| `metadata.name` | Yes | Pod name. |
| `metadata.namespace` | No | Defaults to `default`. |
| `metadata.labels` | No | Used by Services and higher-level selectors. |
| `metadata.annotations` | No | Stored with the Pod. |

### Pod spec fields

| Field | Required | Notes |
|---|---:|---|
| `spec.hostUsers` | No | Defaults to `true`. Set to `false` to request rootless execution for Pod-managed containers. |
| `spec.containers` | Yes for useful workloads | Container list. |
| `spec.volumes` | No | Kubernetes-style host directory volumes. |

### Container fields

| Field | Required | Notes |
|---|---:|---|
| `name` | No at parser level | Container name. Recommended. |
| `image` | Yes for useful workloads | Container image reference. |
| `command` | No | Array form only. |
| `args` | No | Appended to `command`. |
| `env` | No | Supports only `name` + `value`. Empty `name` entries are ignored. |
| `ports` | No | Supports `hostPort` + `containerPort`. Entries without `hostPort` are not host-published. |
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

`containerPort` without `hostPort` is useful for documentation and Service backend conventions, but it is not host-published by the Pod manifest path.

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
      type: DirectoryOrCreate
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
|---|---|
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
- Duplicate `volumes[].name` values fail.
- `volumeMounts[].name` must refer to an existing volume.
- Only `hostPath` volumes are supported.

`readOnly: true` appends `:ro` to the generated Raind mount string.

### Rootless Pods

Set `spec.hostUsers: false` to run Pod-managed containers in rootless mode:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: rootless-worker
  namespace: demo
spec:
  hostUsers: false
  containers:
    - name: worker
      image: busybox:latest
      command:
        - sh
        - -c
      args:
        - trap "exit 0" TERM INT; while true; do id; cat /proc/self/uid_map; sleep 30; done
```

Current rootless Pod behavior:

- `hostUsers: false` enables rootless mode for Pod-managed containers.
- The default rootless mapping mode is `shifted-root`.
- Rootless Pod app containers share the infra container's network, IPC, and UTS namespaces, so Service and Ingress resources can target the Pod IP.
- Each rootless Pod app container still gets its own user namespace mapping. Raind configures the shared Pod network namespace so rootless workloads can bind normal service ports such as 80.
- Use long-running container commands when you expect the Pod to remain `Ready`.

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
|---|---:|---|
| `metadata.name` | Yes | ReplicaSet name. Also used as the template name if `spec.template.metadata.name` is omitted. |
| `metadata.namespace` | No | Defaults to `default`. |
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
            - containerPort: 80
          volumeMounts:
            - name: data
              mountPath: /usr/share/nginx/html
              readOnly: true
          tty: true
```

### Field notes

Deployment uses the same template subset as ReplicaSet. Raind stores a Deployment and lets the controller create/manage the backing ReplicaSet.

| Field | Required | Notes |
|---|---:|---|
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
  name: <name>                 # required
  namespace: <namespace>       # optional, default: default
spec:
  type: ClusterIP              # optional; supported: ClusterIP, NodePort; default: ClusterIP
  clusterIP: 10.166.255.10     # optional; ClusterIP only
  selector:
    app: demo
  ports:
    - port: 80
      targetPort: 8080         # optional, defaults to port
      protocol: TCP            # optional, defaults to TCP
```

### Field notes

| Field | Required | Notes |
|---|---:|---|
| `metadata.name` | Yes | Service name. |
| `metadata.namespace` | No | Defaults to `default`. |
| `spec.type` | No | Defaults to `ClusterIP`. Supported: `ClusterIP`, `NodePort`. |
| `spec.clusterIP` | No | Optional explicit Service VIP for `ClusterIP` Services. Auto-assigned when omitted. |
| `spec.selector` | No at parser level | Used by the Service controller to select Pods by label. Useful Services should define it. |
| `spec.ports[].port` | Yes for each active port | Entries with `port: 0` are ignored. For `NodePort`, this is the host-published port. |
| `spec.ports[].targetPort` | No | Defaults to `port` when omitted or `0`. |
| `spec.ports[].protocol` | No | Defaults to `TCP`. |

### Service type behavior

| Type | Default | External exposure | Behavior |
|---|---:|---:|---|
| `ClusterIP` | Yes | No | Allocates or uses a Service VIP and forwards traffic from `clusterIP:port` to matching Pod backends. |
| `NodePort` | No | Yes | Publishes the Service port on the host and forwards traffic to matching Pod backends. |

Unsupported Kubernetes Service types such as `LoadBalancer` and `ExternalName` are not implemented.

## `Ingress`

### Schema

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: <name>                 # required
  namespace: <namespace>       # optional, default: default
spec:
  tls:                         # optional; enables HTTPS for listed hosts
    - hosts:
        - demo.local
      secretName: demo-tls     # optional; accepted but not used yet
  rules:
    - host: demo.local
      http:
        paths:
          - path: /
            pathType: Prefix   # optional; supported: Prefix, Exact; default: Prefix
            backend:
              service:
                name: demo-svc
                port:
                  number: 80
```

### Field notes

| Field | Required | Notes |
|---|---:|---|
| `metadata.name` | Yes | Ingress name. |
| `metadata.namespace` | No | Defaults to `default`. |
| `spec.rules` | Yes | At least one host/path rule is required. |
| `spec.rules[].host` | Yes for useful routing | Stored in lowercase and matched against the HTTP Host header. |
| `spec.rules[].http.paths` | Yes | At least one path is required for each rule. |
| `spec.rules[].http.paths[].path` | No | Defaults to `/`. Must start with `/`. |
| `spec.rules[].http.paths[].pathType` | No | Defaults to `Prefix`. Supported: `Prefix`, `Exact`. |
| `spec.rules[].http.paths[].backend.service.name` | Yes | Backend Service name in the same namespace. |
| `spec.rules[].http.paths[].backend.service.port.number` | Yes | Backend Service port number. Named ports are not supported yet. |
| `spec.tls[].hosts` | No | Enables managed TLS cert issuance for each listed host. |
| `spec.tls[].secretName` | No | Accepted for Kubernetes-style compatibility, but not used yet. |

### Ingress behavior

Raind implements Ingress with an embedded gateway inside `condenser`.

| Gateway | Default address | Environment override |
|---|---|---|
| HTTP | `:7780` | `RAIND_INGRESS_HTTP_ADDR` |
| HTTPS | `:7443` | `RAIND_INGRESS_HTTPS_ADDR` |

Ingress routing uses Host header + path matching. TLS certificate selection uses SNI.

For HTTPS, Raind uses a dedicated Ingress CA under `/etc/raind/ingress/certs`. It issues host-specific server certificates with the host set as a DNS SAN. These certificates are local Raind-managed certificates and are not publicly trusted unless the Raind Ingress CA is installed into the client trust store.

When an Ingress is removed, Raind removes per-host TLS certificates for hosts that are no longer referenced by any remaining Ingress.

## Comprehensive example

The following multi-document manifest exercises the currently supported resource kinds and major fields.

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: demo
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
          args: ["while true; do echo rs; sleep 60; done"]
          env:
            - name: ROLE
              value: worker
          volumeMounts:
            - name: rs-data
              mountPath: /data
              readOnly: false
          tty: true
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo-web
  namespace: demo
spec:
  replicas: 2
  selector:
    matchLabels:
      app: demo-web
  template:
    metadata:
      labels:
        app: demo-web
        tier: web
      annotations:
        raind.dev/example: deployment
    spec:
      volumes:
        - name: html
          hostPath:
            path: /home/workshop/raind-demo/html
            type: DirectoryOrCreate
      containers:
        - name: web
          image: nginx:latest
          ports:
            - containerPort: 80
          volumeMounts:
            - name: html
              mountPath: /usr/share/nginx/html
              readOnly: true
          tty: true
---
apiVersion: v1
kind: Service
metadata:
  name: demo-svc
  namespace: demo
spec:
  type: ClusterIP
  selector:
    app: demo-web
  ports:
    - port: 80
      targetPort: 80
      protocol: TCP
---
apiVersion: v1
kind: Service
metadata:
  name: demo-nodeport
  namespace: demo
spec:
  type: NodePort
  selector:
    app: demo-web
  ports:
    - port: 8081
      targetPort: 80
      protocol: TCP
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: demo-ingress
  namespace: demo
spec:
  tls:
    - hosts:
        - demo.local
        - www.demo.local
  rules:
    - host: demo.local
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: demo-svc
                port:
                  number: 80
    - host: www.demo.local
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: demo-svc
                port:
                  number: 80
```

Apply:

```sh
raind resource apply -f manifest.yaml
```

List Services and Ingresses:

```sh
raind resource service ls --namespace demo
raind resource ingress ls --namespace demo
```

HTTP Ingress access:

```sh
curl -H 'Host: demo.local' http://127.0.0.1:7780/
```

HTTPS Ingress access:

```sh
curl -k --resolve demo.local:7443:127.0.0.1 https://demo.local:7443/
```

Remove:

```sh
raind resource rm -f manifest.yaml
```
