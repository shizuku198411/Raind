# Raind - Service
Raind supports a Kubernetes-style `Service` manifest, but its runtime
semantics are intentionally simpler than Kubernetes.

In Kubernetes, a Service is primarily a stable L4 endpoint for a set of
Pods. The default Service type is `ClusterIP`, which is reachable only
inside the cluster. External exposure is represented by types such as
`NodePort` and `LoadBalancer`.

Raind currently implements Service as a single-host L4 load-balancing and
host-port publishing mechanism. A Raind Service selects matching Pods and
programs iptables rules to forward traffic from the host port to matching
Pod backends.

Therefore, Raind Service behavior is closer to a simplified single-node
`NodePort` model than to Kubernetes' default `ClusterIP` model.

## Manifest Example
```yaml
apiVersion: v1
kind: Service
metadata:
  name: demo-svc
  namespace: default
spec:
  selector:
    app: demo
  ports:
  - port: 11240
    targetPort: 80
    protocol: TCP
```

## Create
Create from a manifest.
```
$ raind resource apply -f /path/to/service.yaml
resource: demo-svc applied
```

You can also create via the service command.
```
$ raind resource service create -f /path/to/service.yaml
service: <service-id> created
```

## List/Show/Remove
```
$ raind resource service ls
$ raind resource service show <service-id>
$ raind resource service rm <service-id>
```

## Remove via Manifest
```
$ raind resource rm -f /path/to/service.yaml
service: demo-svc removed
```