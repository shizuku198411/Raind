# Raind - Service

A `Service` provides L4 traffic forwarding for Pods selected by labels.

Raind supports Kubernetes-style Service manifests, but the runtime semantics are intentionally simpler than Kubernetes. In Raind, Services are implemented with host-level `iptables` rules and Pod IPs managed through the Pod infra container.

## Supported Service Types

Raind currently supports the following Service types:

| Type | Default | External exposure | Behavior |
|---|---:|---:|---|
| `ClusterIP` | Yes | No | Allocates or uses a Service VIP and forwards traffic from `clusterIP:port` to matching Pod backends. |
| `NodePort` | No | Yes | Publishes the Service port on the host and forwards traffic to matching Pod backends. |

If `spec.type` is omitted, Raind treats the Service as `ClusterIP`.

Unsupported Kubernetes Service types such as `LoadBalancer` and `ExternalName` are not implemented.

## Kubernetes Compatibility Notes

Raind uses Kubernetes-style manifest syntax, but it does not aim to be a full Kubernetes Service implementation.

In Kubernetes, the default Service type is `ClusterIP`, which exposes a stable virtual IP only inside the cluster. External exposure is handled by Service types such as `NodePort` or `LoadBalancer`.

Raind follows the same default direction:

- `ClusterIP` is the default and is intended for internal Pod-to-Service traffic.
- `NodePort` must be explicitly requested when host-level exposure is needed.

Raind's `ClusterIP` implementation is a single-host Service VIP backed by `iptables` DNAT rules. It is not a full kube-proxy replacement and does not implement the complete Kubernetes networking model.

## ClusterIP Service

A `ClusterIP` Service forwards traffic sent to the Service VIP and Service port to one of the matching Pod backends.

When `spec.clusterIP` is omitted, Raind allocates a Service IP automatically.

```yaml
apiVersion: v1
kind: Service
metadata:
  name: demo-svc
  namespace: demo
spec:
  selector:
    app: demo
  ports:
    - port: 80
      targetPort: 8080
      protocol: TCP
```

This is equivalent to:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: demo-svc
  namespace: demo
spec:
  type: ClusterIP
  selector:
    app: demo
  ports:
    - port: 80
      targetPort: 8080
      protocol: TCP
```

With an explicit ClusterIP:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: demo-svc
  namespace: demo
spec:
  type: ClusterIP
  clusterIP: 10.166.255.10
  selector:
    app: demo
  ports:
    - port: 80
      targetPort: 8080
      protocol: TCP
```

A Pod can access the Service by using the assigned ClusterIP and port:

```sh
raind container exec <container-id> -- sh -c 'wget -qO- http://10.166.255.10:80'
```

## NodePort Service

A `NodePort` Service publishes the Service port on the host and forwards traffic to matching Pod backends.

Use `type: NodePort` when the Service should be reachable through the host IP and port.

```yaml
apiVersion: v1
kind: Service
metadata:
  name: demo-nodeport
  namespace: demo
spec:
  type: NodePort
  selector:
    app: demo
  ports:
    - port: 8081
      targetPort: 80
      protocol: TCP
```

Example access from the host:

```sh
curl http://127.0.0.1:8081
```

Or from another machine that can reach the host:

```sh
curl http://<host-ip>:8081
```

## Manifest Schema

```yaml
apiVersion: v1
kind: Service
metadata:
  name: <service-name>
  namespace: <namespace>      # optional, defaults to default
spec:
  type: ClusterIP | NodePort  # optional, defaults to ClusterIP
  clusterIP: <service-vip>    # optional, only used by ClusterIP
  selector:
    <label-key>: <label-value>
  ports:
    - port: <service-port>
      targetPort: <pod-port>  # optional, defaults to port
      protocol: TCP | UDP     # optional, defaults to TCP
```

### Fields

| Field | Required | Default | Description |
|---|---:|---|---|
| `metadata.name` | Yes | - | Service name. |
| `metadata.namespace` | No | `default` | Namespace where the Service belongs. |
| `spec.type` | No | `ClusterIP` | Service type. Supported values are `ClusterIP` and `NodePort`. |
| `spec.clusterIP` | No | auto-assigned | Service VIP for `ClusterIP` Services. |
| `spec.selector` | Yes | - | Label selector used to find matching Pods in the same namespace. |
| `spec.ports[].port` | Yes | - | Service port. For `ClusterIP`, this is the VIP port. For `NodePort`, this is the host-published port. |
| `spec.ports[].targetPort` | No | `port` | Backend Pod port. |
| `spec.ports[].protocol` | No | `TCP` | L4 protocol. |

## Runtime Behavior

The Service controller periodically reconciles Service state.

For each Service, it:

1. Lists Pods in the same namespace.
2. Selects Pods matching `spec.selector`.
3. Resolves each selected Pod to its infra container IP.
4. Creates or updates a Service-specific `iptables` chain.
5. Adds DNAT rules from the Service frontend to matching Pod backends.

For `ClusterIP`, the frontend is:

```text
<clusterIP>:<port>
```

For `NodePort`, the frontend is:

```text
<host-ip>:<port>
```

If multiple Pod backends match the selector, Raind distributes traffic across them using `iptables` statistic rules.

## CLI Usage

### Apply from Manifest

```sh
raind resource apply -f service.yaml
```

Example output:

```text
service: demo-svc applied
```

### List Services

```sh
raind resource service ls --namespace demo
```

The list output includes the Service type.

```text
SERVICE ID                    NAME           NAMESPACE  TYPE       CLUSTER IP       PORTS          CREATED
01kv...                       demo-svc       demo       ClusterIP  10.166.255.1    80->8080/tcp   less than a minutes
01kv...                       demo-nodeport  demo       NodePort   -               8081->80/tcp   less than a minutes
```

### Show a Service

```sh
raind resource service show <service-id>
```

### Remove a Service

```sh
raind resource service rm <service-id>
```

### Remove via Manifest

```sh
raind resource rm -f service.yaml
```

Example output:

```text
service: demo-svc removed
```

## Complete Example

This example creates a namespace, a Deployment, an internal ClusterIP Service, and an explicitly exposed NodePort Service.

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
        - name: app
          image: nginx:latest
          ports:
            - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: demo-web-internal
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
  name: demo-web-public
  namespace: demo
spec:
  type: NodePort
  selector:
    app: demo-web
  ports:
    - port: 8081
      targetPort: 80
      protocol: TCP
```

Apply:

```sh
raind resource apply -f demo-service.yaml
```

List Services:

```sh
raind resource service ls --namespace demo
```

Access the NodePort Service from the host:

```sh
curl http://127.0.0.1:8081
```

Access the ClusterIP Service from a container or Pod network namespace using the assigned ClusterIP shown by `raind resource service ls`.

## Current Limitations

- `LoadBalancer` is not implemented.
- `ExternalName` is not implemented.
- Kubernetes DNS-based Service discovery is not implemented.
- Session affinity is not implemented.
- Multi-node Service routing is not implemented.
- `spec.ports[].nodePort` is not implemented; for `NodePort`, Raind currently uses `spec.ports[].port` as the host-published port.
- The current implementation is designed for Raind's single-host runtime model.
