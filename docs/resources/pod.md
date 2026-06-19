# Raind - Pod

A `Pod` is a workload unit that groups one or more containers.

Raind Pods use an infra container to keep shared namespaces stable. Containers in the same Pod share the Pod network, IPC, and UTS namespaces, so they behave as a single workload unit with a stable Pod IP.

## Supported Resource

```yaml
apiVersion: v1
kind: Pod
```

## Supported Fields

### Metadata

| Field | Required | Description |
|---|---:|---|
| `metadata.name` | yes | Pod name. |
| `metadata.namespace` | no | Namespace. Defaults to `default`. |
| `metadata.labels` | no | Labels used by ReplicaSet, Deployment, and Service selectors. |
| `metadata.annotations` | no | Stored with the Pod metadata. |

### Pod Spec

| Field | Required | Description |
|---|---:|---|
| `spec.containers` | yes | List of container specs. |
| `spec.volumes` | no | Host directory volumes. Only `hostPath` is currently supported. |
| `spec.hostUsers` | no | Set to `false` to run Pod-managed containers in rootless mode. Defaults to `true`. |

### Container Spec

| Field | Required | Description |
|---|---:|---|
| `name` | no | Container name inside the Pod. |
| `image` | yes | Container image. |
| `command` | no | Command array. |
| `args` | no | Arguments appended to `command`. |
| `env` | no | Environment variables as `name` / `value` entries, `valueFrom.configMapKeyRef`, or `valueFrom.secretKeyRef`. |
| `envFrom` | no | Import all keys from a ConfigMap with `configMapRef`, or a Secret with `secretRef`. |
| `ports` | no | Container ports. `hostPort` creates a host port mapping. |
| `mount` | no | Raind low-level mount strings such as `/host:/container[:options]`. |
| `volumeMounts` | no | Kubernetes-style volume mounts referencing `spec.volumes`. |
| `securityContext.capabilities.add` | no | Linux capabilities to add. |
| `securityContext.capabilities.drop` | no | Linux capabilities to drop. |
| `tty` | no | Allocate a TTY for the container. |

### HostPath Volumes

Raind currently supports Kubernetes-style `hostPath` directory volumes.

| Field | Required | Description |
|---|---:|---|
| `volumes[].name` | yes | Volume name. |
| `volumes[].hostPath.path` | yes | Absolute host path. |
| `volumes[].hostPath.type` | no | Supported values: `Directory`, `DirectoryOrCreate`. Empty is treated as `Directory`. |

Supported `hostPath.type` values:

| Type | Behavior |
|---|---|
| `Directory` | The host path must already exist and must be a directory. |
| `DirectoryOrCreate` | Raind creates the directory if it does not exist. |

Unsupported hostPath types include `File`, `FileOrCreate`, `Socket`, `CharDevice`, and `BlockDevice`.

### Volume Mounts

| Field | Required | Description |
|---|---:|---|
| `volumeMounts[].name` | yes | Name of a volume defined in `spec.volumes`. |
| `volumeMounts[].mountPath` | yes | Absolute path inside the container. |
| `volumeMounts[].readOnly` | no | When true, the bind mount is read-only. |

## Complete Example

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: demo-pod
  namespace: demo
  labels:
    app: demo
    tier: web
  annotations:
    raind.dev/example: pod
spec:
  volumes:
    - name: html
      hostPath:
        path: /home/workshop/demo-html
        type: DirectoryOrCreate
  containers:
    - name: web
      image: nginx:latest
      ports:
        - containerPort: 80
          hostPort: 8080
      volumeMounts:
        - name: html
          mountPath: /usr/share/nginx/html
          readOnly: true
      securityContext:
        capabilities:
          add:
            - CAP_NET_BIND_SERVICE
          drop:
            - CAP_NET_RAW
    - name: sidecar
      image: alpine:latest
      command:
        - /bin/sh
        - -c
      args:
        - while true; do date; sleep 30; done
      env:
        - name: APP_ENV
          value: demo
        - name: LOG_LEVEL
          valueFrom:
            configMapKeyRef:
              name: app-config
              key: LOG_LEVEL
        - name: DB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: db-secret
              key: DB_PASSWORD
      envFrom:
        - configMapRef:
            name: app-config
        - secretRef:
            name: db-secret
      tty: true
```

## Rootless Pod Example

Set `spec.hostUsers: false` to request rootless execution for the Pod containers.

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: demo-rootless
---
apiVersion: v1
kind: Pod
metadata:
  name: rootless-worker
  namespace: demo-rootless
  labels:
    app: rootless-worker
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

Apply and verify:

```sh
raind resource apply -f rootless-pod.yaml
raind resource pod ls --namespace demo-rootless
raind resource rm -f rootless-pod.yaml
```

Current rootless Pod behavior:

- `spec.hostUsers: false` enables rootless mode for Pod-managed containers.
- Raind creates user namespaces for the Pod infra container and app containers.
- Rootless Pod app containers share the infra container's network, IPC, and UTS namespaces, so Service and Ingress resources can target the Pod IP.
- Each rootless Pod app container still gets its own user namespace mapping. Raind configures the shared Pod network namespace so rootless workloads can bind normal service ports such as 80.
- The default rootless mapping mode is `shifted-root`, the same as `raind container run --rootless`.
- Rootless Pod manifests should use long-running commands when you want the Pod to remain `Ready`.

## Create

Create from a manifest:

```sh
raind resource apply -f pod.yaml
```

Create only Pod metadata directly:

```sh
raind resource pod create -n demo-pod -l app=demo
```

## List / Start / Stop / Remove

```sh
raind resource pod ls
raind resource pod ls --namespace demo
raind resource pod start <pod-id>
raind resource pod stop <pod-id>
raind resource pod rm <pod-id>
```

## Remove via Manifest

```sh
raind resource rm -f pod.yaml
```

## Notes

- `metadata.namespace` defaults to `default`.
- Pod containers share the Pod network namespace through the infra container.
- Rootless Pods are enabled with `spec.hostUsers: false`.
- `ports[].hostPort` publishes a container port on the host. Prefer a Service for workload-level exposure.
- `hostPath.path` and `volumeMounts.mountPath` must be absolute paths.
- Only host directory mounts are supported through Kubernetes-style `volumes`.
