# Raind - Deployment

A `Deployment` manages the desired number of Pods through an owned ReplicaSet.

The Deployment controller creates the backing ReplicaSet and keeps its replica count in sync with `spec.replicas`.

## Supported Resource

```yaml
apiVersion: apps/v1
kind: Deployment
```

## Supported Fields

### Metadata

| Field | Required | Description |
|---|---:|---|
| `metadata.name` | yes | Deployment name. |
| `metadata.namespace` | no | Namespace. Defaults to `default`. |

### Spec

| Field | Required | Description |
|---|---:|---|
| `spec.replicas` | no | Desired Pod count. Defaults to `1`. Must be `>= 0`. |
| `spec.selector.matchLabels` | no | Labels used to match Pods. Defaults to template labels when omitted. |
| `spec.template.metadata.name` | no | Pod template name. Defaults to Deployment name when omitted. |
| `spec.template.metadata.labels` | no | Pod labels. Must match `spec.selector.matchLabels` when selector is provided. |
| `spec.template.metadata.annotations` | no | Pod annotations. |
| `spec.template.spec.containers` | yes | Container specs for Pods created by the Deployment. |
| `spec.template.spec.volumes` | no | HostPath directory volumes for Pods created by the Deployment. |

The Pod template supports the same container and volume fields as the Pod manifest.

## Complete Example

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo-deploy
  namespace: demo
spec:
  replicas: 2
  selector:
    matchLabels:
      app: demo-deploy
  template:
    metadata:
      labels:
        app: demo-deploy
        tier: web
      annotations:
        raind.dev/example: deployment
    spec:
      volumes:
        - name: html
          hostPath:
            path: /home/workshop/demo-deploy-html
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
        - name: sidecar
          image: alpine:latest
          command:
            - /bin/sh
            - -c
          args:
            - while true; do echo deployment sidecar; sleep 30; done
          env:
            - name: APP_ENV
              value: demo
          tty: true
```

## Create

Create from a manifest:

```sh
raind resource apply -f deployment.yaml
```

## List / Show / Scale / Remove

```sh
raind resource deployment ls
raind resource deployment ls --namespace demo
raind resource deployment show <deployment-id>
raind resource deployment scale <deployment-id> -r 3
raind resource deployment rm <deployment-id>
```

The short alias is also available:

```sh
raind resource deploy ls
```

## Remove via Manifest

```sh
raind resource rm -f deployment.yaml
```

## Notes

- `spec.replicas` defaults to `1` when omitted.
- `spec.replicas: 0` is valid and scales the Deployment to zero Pods.
- If `spec.selector.matchLabels` is set, it must match the template labels.
- Raind currently implements a simple Deployment model backed by a ReplicaSet. Full Kubernetes rollout strategies, revisions, conditions, and update strategies are not currently implemented.
