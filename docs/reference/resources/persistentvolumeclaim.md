# Raind - PersistentVolumeClaim

A `PersistentVolumeClaim` allocates a Raind-managed local host directory and lets Pods mount it as persistent data.

Raind's PVC support is intentionally local and simple. It is useful for database-like demos and stateful development workloads, but it is not a full Kubernetes storage implementation.

## Supported Resource

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
```

## Supported Fields

### Metadata

| Field | Required | Description |
|---|---:|---|
| `metadata.name` | yes | PVC name. |
| `metadata.namespace` | no | Namespace. Defaults to `default`. |
| `metadata.annotations.raind.dev/reclaimPolicy` | no | `Retain` or `Delete`. Defaults to `Retain`. |

### Spec

| Field | Required | Description |
|---|---:|---|
| `spec.accessModes` | yes | Only `ReadWriteOnce` is supported. |
| `spec.resources.requests.storage` | yes | Stored as requested size metadata. Quota is not enforced in the MVP. |
| `spec.storageClassName` | no | Stored as metadata only. |
| `spec.volumeMode` | no | Only `Filesystem` is supported. Empty defaults to `Filesystem`. |

## Storage Quantity

Raind parses and stores both the original storage string and a normalized byte value.

Supported examples:

| Input | Bytes |
|---|---:|
| `1024` | `1024` |
| `1K` | `1000` |
| `1M` | `1000000` |
| `1G` | `1000000000` |
| `1Ki` | `1024` |
| `1Mi` | `1048576` |
| `1Gi` | `1073741824` |

Quota enforcement is not implemented yet.

## Host Storage Layout

PVC data is allocated under Raind's runtime-owned volume root:

```text
/etc/raind/volume/pvc/<namespace>/<pvc-id>/data
```

PVC metadata is stored in:

```text
/etc/raind/store/resource/volume/vsm.json
```

The host path uses the generated PVC ID instead of the PVC name to avoid collisions and path-safety issues.

## Pod Usage

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: db-data
  namespace: demo
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
---
apiVersion: v1
kind: Pod
metadata:
  name: db
  namespace: demo
spec:
  volumes:
    - name: data
      persistentVolumeClaim:
        claimName: db-data
  containers:
    - name: db
      image: busybox:latest
      command:
        - sh
        - -c
      args:
        - trap "exit 0" TERM INT; while true; do date >> /data/log.txt; sleep 30; done
      volumeMounts:
        - name: data
          mountPath: /data
```

## Reclaim Policy

Default behavior is `Retain`.

```yaml
metadata:
  annotations:
    raind.dev/reclaimPolicy: Retain
```

With `Retain`, removing the PVC removes Raind state but leaves the host data directory on disk.

Use `Delete` when the host data directory should be removed with the PVC:

```yaml
metadata:
  annotations:
    raind.dev/reclaimPolicy: Delete
```

PVC removal fails while a running Pod still references the PVC.

## CLI Usage

Apply:

```sh
raind resource apply -f pvc.yaml
```

List:

```sh
raind resource pvc ls --namespace demo
```

Show:

```sh
raind resource pvc show db-data --namespace demo
```

Remove:

```sh
raind resource rm -f pvc.yaml
raind resource pvc rm db-data --namespace demo
```

## Limitations

- Only local directory-backed PVCs are supported.
- Only `ReadWriteOnce` is supported.
- Requested size is metadata only.
- `PersistentVolume` objects are not supported.
- `subPath` is not supported.
- Cross-namespace PVC mounts are not supported.
- Automatic adoption of retained data is not supported.
