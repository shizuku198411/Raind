# Raind - Namespace

A `Namespace` groups Raind resources and provides a default network boundary for Pods, ReplicaSets, Deployments, and Services.

The built-in `default` namespace uses the default Raind network. User-created namespaces can create their own namespace network or be bound to an existing network.

## Supported Resource

```yaml
apiVersion: v1
kind: Namespace
```

## Supported Fields

| Field | Required | Description |
|---|---:|---|
| `metadata.name` | yes | Namespace name. |

Raind currently ignores other Kubernetes namespace fields such as labels, annotations, status, and finalizers.

## Behavior

When a namespace is created, Raind tracks it as a resource namespace. Workloads created in that namespace use the namespace network by default unless a lower-level container spec explicitly overrides the network.

This means Pods created by a Deployment, ReplicaSet, or Pod manifest in the same namespace are placed into the same namespace-level network boundary.

## Complete Example

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
spec:
  replicas: 2
  selector:
    matchLabels:
      app: demo-web
  template:
    metadata:
      labels:
        app: demo-web
    spec:
      containers:
        - name: web
          image: nginx:latest
          ports:
            - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: demo-web
  namespace: demo
spec:
  type: ClusterIP
  selector:
    app: demo-web
  ports:
    - port: 80
      targetPort: 80
      protocol: TCP
```

## Create

Create a namespace directly:

```sh
raind resource namespace create demo
```

Create a namespace bound to an existing network:

```sh
raind network create sharednet
raind resource namespace create demo --network sharednet
```

Create from a manifest:

```sh
raind resource apply -f namespace.yaml
```

## List / Show / Remove

```sh
raind resource namespace ls
raind resource namespace show demo
raind resource namespace rm demo
```

The short alias is also available:

```sh
raind resource ns ls
```

## Namespace Filters

Most resource list commands can be filtered by namespace:

```sh
raind resource pod ls --namespace demo
raind resource replicaset ls --namespace demo
raind resource deployment ls --namespace demo
raind resource service ls --namespace demo
```

## Remove via Manifest

```sh
raind resource rm -f namespace.yaml
```

## Notes

- `metadata.name` is required.
- The `default` namespace is used when a supported workload manifest omits `metadata.namespace`.
- Namespace manifests are Kubernetes-style, but Raind does not currently implement full Kubernetes namespace semantics such as labels, quotas, finalizers, or status.
