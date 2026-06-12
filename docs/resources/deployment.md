# Raind - Deployment

A Deployment manages the desired number of Pods through an owned ReplicaSet.  
The Deployment controller creates the backing ReplicaSet and keeps its replica count in sync.

## Manifest Example

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo-deploy
  namespace: default
spec:
  replicas: 2
  selector:
    matchLabels:
      app: demo
  template:
    metadata:
      labels:
        app: demo
    spec:
      containers:
      - name: nginx
        image: nginx:latest
```

## Create

Create from a manifest.

```sh
raind resource apply -f /path/to/deployment.yaml
```

## List/Show/Scale/Remove

```sh
raind resource deployment ls
raind resource deployment show <deployment-id>
raind resource deployment scale <deployment-id> -r 3
raind resource deployment rm <deployment-id>
```

You can also use the short alias:

```sh
raind resource deploy ls
```

## Remove via Manifest

```sh
raind resource rm -f /path/to/deployment.yaml
```
