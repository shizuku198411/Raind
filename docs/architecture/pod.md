# Pod Architecture

Raind Pods are Kubernetes-style workload units built on top of Raind containers. A Pod is represented by a Pod state entry, a Pod template, an infra container, and one or more app containers. The infra container anchors the shared Pod namespaces and owns the Pod IP. App containers join the infra container's namespaces.

## Goals

Raind's Pod implementation provides:

```text
- Kubernetes-style Pod manifests
- shared network namespace
- shared IPC namespace
- shared UTS namespace
- Pod IP owned by the infra container
- app containers joining infra namespaces
- label-based Service endpoint selection
- ReplicaSet / Deployment reconciliation
```

It is intentionally smaller than Kubernetes. It focuses on the runtime mechanics that make Pods different from standalone containers.

## High-level architecture

```text
                   Pod: demo/web

  +------------------------------------------------------------+
  | PSM PodInfo                                                |
  | - podId                                                    |
  | - templateId                                               |
  | - namespace                                                |
  | - labels / annotations                                     |
  | - NetworkNS / IPCNS / UTSNS / UserNS paths                 |
  | - state                                                    |
  +---------------------------+--------------------------------+
                              |
                              v
  +------------------------------------------------------------+
  | infra container                                            |
  | name: condenser-pod-infra-...                              |
  | image: registry.k8s.io/pause:3.9                           |
  | owns Pod IP                                                |
  | owns net/ipc/uts namespace paths                           |
  +---------------------------+--------------------------------+
                              |
              +---------------+----------------+
              |                                |
              v                                v
  +------------------------+       +------------------------+
  | app container A        |       | app container B        |
  | joins infra net/ipc/uts|       | joins infra net/ipc/uts|
  | own mount/pid/cgroup   |       | own mount/pid/cgroup   |
  +------------------------+       +------------------------+
```

## Stores and state

Pod-related state lives in PSM:

```text
/etc/raind/store/psm.json
```

Main objects:

| Object | Meaning |
|---|---|
| `PodTemplateInfo` | Desired Pod template: containers, labels, namespace settings. |
| `PodInfo` | Concrete Pod instance and runtime namespace paths. |
| `ReplicaSetInfo` | Desired replica count for a template. |
| `DeploymentInfo` | Deployment-level desired state and owning ReplicaSet. |

Container state remains in CSM:

```text
/etc/raind/store/csm.json
```

The relationship is:

```text
PodInfo podId
  |
  +-- infra container CSM entry, PodId=<podId>
  +-- app container CSM entries, PodId=<podId>
```

## Pod creation flow

For a manifest Pod:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: web
  namespace: demo
  labels:
    app: web
spec:
  containers:
    - name: app
      image: nginx:latest
```

The flow is:

```text
resource apply
  |
  v
DecodeK8sManifests
  |
  v
PodService.Create
  |
  +-- resolve namespace -> network bridge
  +-- store PodTemplateSpec
  +-- create PodInfo with state=created
  v
PodService.Start or PodController reconcile
  |
  +-- ensure infra container exists
  +-- start infra container
  +-- create missing app containers from template
  +-- start app containers
  +-- update Pod state=running
```

## Infra container

The infra container is the Pod sandbox anchor.

```text
image: registry.k8s.io/pause:3.9
name prefix: condenser-pod-infra-
```

The infra container is created as a normal Raind container with new namespaces:

```text
mount
network
uts
pid
ipc
cgroup
```

It receives the Pod IP from IPAM and attaches to the namespace's bridge network.

```text
namespace demo
  |
  v
network bridge rns...
  |
  v
infra container IP = Pod IP
```

The infra container must stay running for namespace continuity. If it exits, app containers may still exist, but the shared namespace anchor is broken. Raind marks/reconciles such Pods as degraded and can recreate them from template.

## Namespace sharing

After the infra container is available, app containers join selected namespaces using namespace paths from the infra container.

```text
infra container PID = P

/proc/P/ns/net
/proc/P/ns/ipc
/proc/P/ns/uts
```

Condenser stores those paths on the Pod:

```text
PodInfo.NetworkNS
PodInfo.IPCNS
PodInfo.UTSNS
PodInfo.UserNS, when applicable
```

When creating app containers, Condenser passes namespace path options to Droplet:

```text
--ns-path network=/proc/<infra-pid>/ns/net
--ns-path ipc=/proc/<infra-pid>/ns/ipc
--ns-path uts=/proc/<infra-pid>/ns/uts
```

The app container still gets its own:

```text
mount namespace
pid namespace
cgroup namespace
```

Diagram:

```text
                         Pod namespace sharing

  +------------------------ infra container ------------------------+
  | net ns: /proc/P/ns/net                                          |
  | ipc ns: /proc/P/ns/ipc                                          |
  | uts ns: /proc/P/ns/uts                                          |
  | Pod IP: 10.166.x.y                                              |
  +-----------------------------+-----------------------------------+
                                |
               +----------------+----------------+
               |                                 |
               v                                 v
  +---------------------------+      +---------------------------+
  | app container A           |      | app container B           |
  | net -> infra net ns       |      | net -> infra net ns       |
  | ipc -> infra ipc ns       |      | ipc -> infra ipc ns       |
  | uts -> infra uts ns       |      | uts -> infra uts ns       |
  | mnt -> own                |      | mnt -> own                |
  | pid -> own                |      | pid -> own                |
  | cgroup -> own             |      | cgroup -> own             |
  +---------------------------+      +---------------------------+
```

## Pod IP and networking

The Pod IP is the infra container IP.

```text
Service controller
  |
  +-- list Pods
  +-- label selector match
  +-- find infra container by name prefix
  +-- read infra container address from IPAM
  +-- use that address as Service endpoint
```

This is why Services route to Pod IPs rather than to individual app container IPs.

```text
ClusterIP Service
      |
      v
Pod IP = infra container IP
      |
      v
shared network namespace
      |
      v
app containers listening on localhost / Pod IP ports
```

Inside the Pod, app containers share the same network stack. That means:

```text
- they share one IP address
- they share localhost
- port conflicts can happen between containers in the same Pod
- Service targetPort must match a port listened by one app container in the Pod network namespace
```

## Pod hostname and UTS namespace

App containers join the infra UTS namespace, so the Pod has a shared hostname. Condenser uses a short Pod ID-style hostname for Pod-owned containers.

This mirrors the Kubernetes idea that a Pod is a single logical host from the network/hostname perspective.

## Pod lifecycle

```text
created
  |
  | PodService.Start
  v
running
  |
  | user stop / infra down / app failure
  v
stopped or degraded
  |
  | controller reconciliation
  v
running or recreated
```

Current behavior:

```text
- state=created Pods are started by the Pod controller
- stopped Pods may be restarted unless StoppedByUser is set
- degraded Pods are checked for infra status
- if infra is down, the Pod is recreated from template
- if infra is running, app containers are restarted/recovered
```

## Pod controller

The Pod controller runs inside Condenser and reconciles every few seconds.

Responsibilities:

```text
- reconcile Deployments into ReplicaSets
- reconcile ReplicaSets into Pods
- create missing Pods from templates
- start created/stopped Pods
- recover degraded Pods
- scale down excess Pods
- clean up unreferenced template Pods
```

Deployment/ReplicaSet flow:

```text
Deployment
  |
  v
ReplicaSet
  |
  v
PodTemplate
  |
  v
Pod instances
  |
  +-- infra container
  +-- app containers
```

ASCII view:

```text
  Deployment demo-web replicas=2
       |
       v
  ReplicaSet demo-web-rs replicas=2
       |
       v
  +----------------------+   +----------------------+
  | Pod demo-web-abc     |   | Pod demo-web-def     |
  | infra + app          |   | infra + app          |
  +----------------------+   +----------------------+
```

## Pod member container creation

When a Pod starts, Raind ensures all template containers exist.

```text
ensurePodTemplateContainers
  |
  +-- list current containers by podId
  +-- ignore infra container
  +-- for each template container:
        if missing, create app container with PodId
```

For each app container, Condenser calls `ContainerService.Create` with `PodId` set. That triggers Pod-aware behavior:

```text
- ensure infra container exists
- resolve namespace paths from PodInfo
- generate spec with --ns-path network/ipc/uts
- avoid direct host port forwarding for app member containers
- create via nsenter when needed
```

## `nsenter` path

For Pod member creation, Droplet may be invoked through `nsenter` against the infra container PID when the member needs to be created in the Pod context.

Conceptually:

```text
host
  |
  | nsenter -t <infra-pid> -U -- droplet create <app-container-id>
  v
app container create path
```

This lets the app container creation happen with the correct namespace context when joining an existing Pod sandbox.

## Volumes and mounts

Pod manifests support Kubernetes-style `volumes` and `volumeMounts` for hostPath directory mounts.

Example:

```yaml
spec:
  volumes:
    - name: data
      hostPath:
        path: /home/workshop/data
        type: DirectoryOrCreate
  containers:
    - name: app
      image: alpine:latest
      volumeMounts:
        - name: data
          mountPath: /data
          readOnly: true
```

Raind converts this into a Droplet mount string:

```text
/home/workshop/data:/data:ro
```

Droplet parses that as a read-only bind mount.

Supported hostPath types:

```text
Directory
DirectoryOrCreate
```

Other Kubernetes volume types are not supported yet.

## Capabilities and security context

Pod container specs can set capability additions/drops through a Kubernetes-style security context.

```yaml
securityContext:
  capabilities:
    add:
      - NET_ADMIN
    drop:
      - NET_RAW
```

These settings are applied per app container, not globally to the whole Pod. The infra container should remain minimal.

## Service integration

Services select Pods by labels.

```yaml
apiVersion: v1
kind: Service
metadata:
  name: web
  namespace: demo
spec:
  selector:
    app: web
  ports:
    - port: 80
      targetPort: 8080
```

Flow:

```text
Service selector app=web
  |
  v
matching PodInfo objects in namespace demo
  |
  v
infra container for each Pod
  |
  v
Pod IPs from IPAM
  |
  v
iptables service endpoint rules
```

Because app containers share the Pod network namespace, the Service can target the Pod IP and the app's listening port.

## Ingress integration

Ingress routes to ClusterIP Services, not directly to Pod containers.

```text
Ingress host/path
  |
  v
ClusterIP Service
  |
  v
Pod IP endpoints
  |
  v
app containers in Pod network namespace
```

This keeps responsibilities separated:

```text
Ingress  -> L7 host/path routing
Service  -> L4 load balancing
Pod      -> shared network runtime unit
Container -> actual workload process
```

## Failure and recovery model

The infra container is the most important Pod runtime process.

```text
infra running
  -> namespace continuity is intact
  -> app containers can be restarted

infra stopped/missing
  -> namespace continuity is broken
  -> Pod should be recreated from template
```

Raind stores namespace paths and validates them before reuse. If stored namespace paths are stale, it resets the Pod namespace metadata and falls back to creating a new namespace set.

## Differences from Kubernetes

| Area | Kubernetes | Raind |
|---|---|---|
| Infra container | Pause container managed by kubelet/runtime | `registry.k8s.io/pause:3.9` managed by Condenser/PodService |
| Networking | CNI plugin model | Raind IPAM + Linux bridge + veth |
| Pod IP | Assigned by CNI/runtime | Infra container IP from Raind IPAM |
| Service | kube-proxy/IPVS/iptables/eBPF depending cluster | Condenser ServiceController + iptables |
| Ingress | External ingress controller | Embedded Condenser gateway |
| Scheduling | Multi-node scheduler | Single-host controller model |
| Secrets/configmaps | Kubernetes resources | Not fully modeled yet |

## Useful inspection commands

```sh
raind resource pod ls --namespace demo
raind resource deployment ls --namespace demo
raind resource service ls --namespace demo
raind container ls
raind container logs <infra-or-app-container>
raind container spec <container>

sudo iptables -t nat -nvL
ip addr show
sudo ls -l /proc/<infra-pid>/ns
sudo cat /etc/raind/store/psm.json
sudo cat /etc/raind/store/csm.json
```
