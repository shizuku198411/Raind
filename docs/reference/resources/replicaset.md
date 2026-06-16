# Raind - ReplicaSet

A `ReplicaSet` keeps a desired number of Pods running from a Pod template.

The ReplicaSet controller reconciles the current Pod count against `spec.replicas` and creates replacement Pods when needed.

## Supported Resource

```yaml
apiVersion: apps/v1
kind: ReplicaSet
```

## Supported Fields

### Metadata

| Field | Required | Description |
|---|---:|---|
| `metadata.name` | yes | ReplicaSet name. |
| `metadata.namespace` | no | Namespace. Defaults to `default`. |

### Spec

| Field | Required | Description |
|---|---:|---|
| `spec.replicas` | no | Desired Pod count. Defaults to `1`. Must be `>= 0`. |
| `spec.selector.matchLabels` | no | Labels used to match owned Pods. Defaults to template labels when omitted. |
| `spec.template.metadata.name` | no | Pod template name. Defaults to ReplicaSet name when omitted. |
| `spec.template.metadata.labels` | no | Pod labels. Must match `spec.selector.matchLabels` when selector is provided. |
| `spec.template.metadata.annotations` | no | Pod annotations. |
| `spec.template.spec.containers` | yes | Container specs for Pods created by the ReplicaSet. |
| `spec.template.spec.volumes` | no | HostPath directory volumes for Pods created by the ReplicaSet. |

The Pod template supports the same container and volume fields as the Pod manifest.

## Complete Example

```yaml
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
        tier: web
      annotations:
        raind.dev/example: replicaset
    spec:
      volumes:
        - name: html
          hostPath:
            path: /home/workshop/demo-rs-html
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
        - name: worker
          image: alpine:latest
          command:
            - /bin/sh
            - -c
          args:
            - while true; do echo replica worker; sleep 30; done
          env:
            - name: ROLE
              value: worker
          tty: true
```

## Create

Create from a manifest:

```sh
raind resource apply -f replicaset.yaml
```

## List / Show / Scale / Remove

```sh
raind resource replicaset ls
raind resource replicaset ls --namespace demo
raind resource replicaset show <replicaset-id>
raind resource replicaset scale <replicaset-id> -r 3
raind resource replicaset rm <replicaset-id>
```

## Remove via Manifest

```sh
raind resource rm -f replicaset.yaml
```

## Notes

- `spec.replicas` defaults to `1` when omitted.
- `spec.replicas: 0` is valid and scales the ReplicaSet to zero Pods.
- If `spec.selector.matchLabels` is set, it must match the template labels.
- ReplicaSet Pods use the namespace network unless a lower-level container network override is provided.
