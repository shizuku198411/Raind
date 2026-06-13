# Raind - Pod
A Pod is an orchestration unit that groups multiple containers.  
Containers in the same Pod share Network/UTS/IPC namespaces.  
An infra (pause) container keeps the namespaces stable so containers share the same IP/hostname.

## Manifest Example
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: demo-pod
  namespace: default
  labels:
    app: demo
spec:
  containers:
  - name: web
    image: nginx:latest
  - name: sidecar
    image: alpine:latest
    tty: true
```

## Create
Creating via manifest is recommended.
```
$ raind resource apply -f /path/to/pod.yaml
resource: demo-pod applied
```

To create only the Pod metadata, use:
```
$ raind resource pod create -n demo-pod -l app=demo
pod: <pod-id> created
```

## List/Start/Stop/Remove
```
$ raind resource pod ls
$ raind resource pod ls --namespace default
$ raind resource pod start <pod-id>
$ raind resource pod stop <pod-id>
$ raind resource pod rm <pod-id>
```

## Remove via Manifest
```
$ raind resource rm -f /path/to/pod.yaml
pod: demo-pod removed
```
