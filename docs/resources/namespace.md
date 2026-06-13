# Raind - Namespace

A Namespace groups Raind resources and provides a default network boundary for Pods, ReplicaSets, Deployments, and Services in that namespace.

The built-in `default` namespace uses the `raind0` network. New namespaces create a dedicated network unless they are bound to an existing network.

## Create

```sh
raind resource namespace create demo
```

Bind a namespace to an existing network:

```sh
raind network create sharednet
raind resource namespace create demo --network sharednet
```

## List/Show/Remove

```sh
raind resource namespace ls
raind resource namespace show demo
raind resource namespace rm demo
```

The short alias is also available:

```sh
raind resource ns ls
```

## Manifest Example

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: demo
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: demo
spec:
  replicas: 2
  selector:
    matchLabels:
      app: web
  template:
    metadata:
      labels:
        app: web
    spec:
      containers:
      - name: nginx
        image: nginx:latest
```

Apply and remove the manifest:

```sh
raind resource apply -f /path/to/namespace-workload.yaml
raind resource rm -f /path/to/namespace-workload.yaml
```

## Namespace Filters

```sh
raind resource pod ls --namespace demo
raind resource replicaset ls --namespace demo
raind resource deployment ls --namespace demo
raind resource service ls --namespace demo
```

## Network Behavior

If a Pod container does not specify a network explicitly, Raind uses the namespace network. This keeps workload traffic separated by namespace while preserving explicit network overrides for advanced use cases.
