# Raind - Service
A Service is an L4 load balancer for Pods.  
It selects Pods by label and distributes traffic via iptables (DNAT).

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
