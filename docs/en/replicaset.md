# Raind - ReplicaSet
A ReplicaSet keeps the desired number of Pods based on a template and selector.  
The controller reconciles the desired count and recreates Pods as needed.

## Manifest Example
```yaml
apiVersion: apps/v1
kind: ReplicaSet
metadata:
  name: demo-rs
  namespace: default
spec:
  replicas: 2
  selector:
    matchLabels:
      app: demo
  template:
    metadata:
      name: demo-pod
      labels:
        app: demo
    spec:
      containers:
      - name: nginx
        image: nginx:latest
      - name: ubuntu
        image: ubuntu:latest
        tty: true
```

## Create
Create from a manifest.
```
$ raind resource apply -f /path/to/replicaset.yaml
resource: demo-rs applied
```

## List/Show/Scale/Remove
```
$ raind resource replicaset ls
$ raind resource replicaset show <replicaset-id>
$ raind resource replicaset scale <replicaset-id> -r 3
$ raind resource replicaset rm <replicaset-id>
```

## Remove via Manifest
```
$ raind resource rm -f /path/to/replicaset.yaml
replicaset: demo-rs removed
```
