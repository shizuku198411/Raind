# Network Architecture

Raind builds a single-host container network around Linux bridges, per-namespace bridge networks, IPAM-managed container addresses, iptables-based L4 forwarding, a DNS proxy for DNS traffic visibility, ClusterIP/NodePort-style Services, and an embedded HTTP/HTTPS Ingress Gateway in Condenser.

The goal is not to reproduce Kubernetes networking exactly. Raind uses Kubernetes-style resource manifests where useful, while keeping the runtime small enough to inspect directly from the host.

## High-level overview

```text
                              host namespace

        +---------------------------------------------------+
        |                         condenser                 |
        |                                                   |
        |  API :7755      Hook :7756      CA :7757          |
        |      |              |              |              |
        |      |              |              |              |
        |      |        +---------------------------+       |
        |      |        | embedded ingress gateway  |       |
        |      |        | HTTP  :7780               |       |
        |      |        | HTTPS :7443               |       |
        |      |        +-------------+-------------+       |
        |      |                      |                     |
        |      |                      v                     |
        |      |            ClusterIP Service VIP           |
        |      |                      |                     |
        |      |                      v                     |
        |      |              Pod backend IPs               |
        +------+--------------------------------------------+
               |
               | iptables NAT / FORWARD / DNS redirect
               v

  +-------------------+       +-------------------+       +-------------------+
  | bridge raind0     |       | bridge rns...     |       | bridge rns...     |
  | 10.166.0.254/24   |       | namespace network |       | namespace network |
  | default namespace |       | demo namespace    |       | another namespace |
  +---------+---------+       +---------+---------+       +---------+---------+
            |                           |                           |
            | veth                      | veth                      | veth
            v                           v                           v
      containers / pods           containers / pods           containers / pods
```

## Main components

| Component | Role |
|---|---|
| `ipam` store | Owns runtime subnet, bridge pools, DNS proxy address, host interface, and container address allocations. |
| `network` service | Creates/removes bridges and installs iptables rules for masquerade, DNS redirect, and host port forwarding. |
| Namespace resource | Creates a Raind namespace and, when requested, an auto-managed bridge network for that namespace. |
| Container networking | Assigns a container IP and writes container-side network settings into the generated runtime spec. |
| Pod networking | The infra container owns the Pod network namespace and Pod IP; app containers join it. |
| Service controller | Selects Pods by labels and programs iptables L4 load-balancing rules. |
| Ingress gateway | Condenser-embedded L7 HTTP/HTTPS gateway. Routes host/path traffic to ClusterIP Services. |
| DNS proxy | Receives redirected DNS traffic at `10.166.254.254:1053` and logs DNS activity. |

## Runtime subnet and bridge layout

Raind uses a runtime subnet managed by IPAM. In the current default layout, the runtime subnet is typically:

```text
10.166.0.0/16
```

The default network is usually:

```text
raind0 -> 10.166.0.254/24
```

Resource namespaces may create additional bridge networks with generated names, for example:

```text
rns2a97516c354b -> 10.166.x.254/24
rnsd2bfc8025ea4 -> 10.166.y.254/24
```

The bridge address acts as the default gateway and resolver address inside containers on that bridge.

```text
container resolv.conf

  nameserver <bridge-gateway-ip>

example:
  nameserver 10.166.1.254
```

## Namespace resource and network binding

A Raind `Namespace` resource is also a network boundary.

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: demo
```

When no explicit network is provided, Raind creates an auto-managed bridge for the namespace:

```text
namespace demo
  -> network rns<hash>
  -> bridge address allocated by IPAM
```

When the namespace is removed, the auto-managed bridge is removed only after the namespace is empty. DNS redirect rules for the bridge are also cleaned up.

```text
Namespace removal

  resource rm namespace/demo
        |
        v
  check Pods / Services / ReplicaSets / Deployments
        |
        v
  remove auto-created bridge, if empty
        |
        v
  remove DNS redirect rules for that bridge
        |
        v
  remove namespace from NSM store
```

## Container address allocation

When a standalone container is created, Condenser resolves the target network and asks IPAM for an address.

```text
raind container run --network raind1 alpine
        |
        v
condenser ContainerService.Create
        |
        v
IPAM allocates address from raind1 pool
        |
        v
condenser generates droplet config.json
        |
        v
droplet creates container with veth + bridge attachment
```

The generated runtime spec includes:

```text
host interface
bridge interface
container interface name
container interface address
container gateway
container DNS entries
```

For standalone containers, Raind allocates a dedicated IP per container.

## Pod network model

Pods use an infra container as the namespace anchor.

```text
Pod demo/app

  +----------------------------------------------------+
  | Pod network namespace                              |
  |                                                    |
  |  infra container                                   |
  |  - owns network namespace                          |
  |  - owns Pod IP                                     |
  |  - owns shared IPC/UTS namespace paths             |
  |                                                    |
  |  app container A                                   |
  |  - joins infra network/ipc/uts namespace           |
  |                                                    |
  |  app container B                                   |
  |  - joins infra network/ipc/uts namespace           |
  +----------------------------------------------------+
```

The Pod IP is the infra container IP. Service endpoint discovery also uses the infra container IP.

## Host egress and masquerade

Raind installs a MASQUERADE rule for traffic from the runtime subnet to the host's default external interface.

```text
containers / pods
      |
      v
bridge raind0 or rns...
      |
      v
iptables POSTROUTING MASQUERADE
      |
      v
host external interface
      |
      v
outside network
```

Typical rule shape:

```text
iptables -t nat -A POSTROUTING \
  -s <runtime-subnet> \
  -o <host-interface> \
  -j MASQUERADE
```

This allows containers to reach the outside network while keeping their private Raind addresses internal to the host.

## DNS proxy and DNS visibility

Raind deploys a DNS proxy interface and redirects container DNS traffic to it.

Current layout:

```text
raindDns -> 10.166.254.254/32
DNS proxy listens on 10.166.254.254:1053
```

Each bridge gets PREROUTING rules for UDP/TCP port 53:

```text
container on bridge rns...
      |
      | query to nameserver <bridge-gateway-ip>:53
      v
iptables PREROUTING -i rns... --dport 53
      |
      v
DNAT to 10.166.254.254:1053
      |
      v
Raind DNS proxy
      |
      v
upstream resolvers
```

Typical rules:

```text
iptables -t nat -A PREROUTING \
  -i <bridge> \
  -p udp --dport 53 \
  -j DNAT --to-destination 10.166.254.254:1053

iptables -t nat -A PREROUTING \
  -i <bridge> \
  -p tcp --dport 53 \
  -j DNAT --to-destination 10.166.254.254:1053
```

This lets containers keep a simple bridge-gateway resolver address while Raind still observes DNS traffic centrally.

## Management traffic protection

Raind separates management APIs from container workloads.

The management API listens on `127.0.0.1:7755` and uses mTLS client certificates. Raind also installs host-level INPUT rules so containers can reach the hook server while management traffic from the runtime subnet is blocked.

```text
container -> host:7756 hook server      allowed
container -> host:7755 management API   dropped
```

This is important because containers need hook callbacks, but they should not directly reach the management API.

## Standalone container port publishing

Standalone containers can publish ports using host port forwarding.

```text
host:<hostPort>
      |
      v
iptables DNAT
      |
      v
containerIP:<containerPort>
```

Raind installs DNAT rules for traffic from:

```text
- external host interface traffic
- local host OUTPUT traffic
- bridge hairpin traffic from containers to host-local addresses
```

This gives standalone containers Docker-like host port behavior.

## Service architecture

Raind Services are Kubernetes-style L4 resources with Raind-specific single-host semantics.

Raind currently supports:

```text
ClusterIP  -> internal VIP, default
NodePort   -> explicit host-published L4 service
```

### ClusterIP Service

`ClusterIP` is the default when `spec.type` is omitted.

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

The Service store assigns a ClusterIP, and the Service controller programs iptables rules:

```text
client in Raind network
      |
      | dst = <clusterIP>:80
      v
iptables PREROUTING/OUTPUT
      |
      v
RAIND-SVC-<id>-80 chain
      |
      v
random DNAT to one backend Pod IP:8080
```

ASCII view:

```text
                 ClusterIP Service

  client container
        |
        | http://10.166.255.1:80
        v
  +-----------------------------+
  | iptables nat                |
  | -d 10.166.255.1 --dport 80  |
  |       -> RAIND-SVC-...      |
  +---------------+-------------+
                  |
        +---------+----------+
        |                    |
        v                    v
  Pod IP A:8080        Pod IP B:8080
```

The backend set is built from Pods in the same namespace whose labels match the Service selector.

### NodePort Service

`NodePort` is explicit host exposure.

```yaml
apiVersion: v1
kind: Service
metadata:
  name: web-public
  namespace: demo
spec:
  type: NodePort
  selector:
    app: web
  ports:
    - port: 8081
      targetPort: 80
```

Raind treats `port` as the host-facing port for NodePort-style Services.

```text
host:8081
   |
   v
iptables DNAT / service chain
   |
   v
Pod backend IP:80
```

This differs from Kubernetes, where `port`, `targetPort`, and `nodePort` are distinct fields. Raind's model is intentionally smaller.

## Ingress architecture

Ingress is implemented as an embedded gateway inside Condenser, not as a system Pod.

```text
HTTP  :7780
HTTPS :7443
```

Environment overrides:

```text
RAIND_INGRESS_HTTP_ADDR
RAIND_INGRESS_HTTPS_ADDR
```

Ingress resources define host/path routing rules.

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: web
  namespace: demo
spec:
  tls:
    - hosts:
        - web.local
  rules:
    - host: web.local
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: web
                port:
                  number: 80
```

Routing path:

```text
host client
  curl -H 'Host: web.local' http://127.0.0.1:7780/
  curl --resolve web.local:7443:127.0.0.1 https://web.local:7443/
        |
        v
Condenser embedded ingress gateway
        |
        | Host header + URL path match
        v
Service web ClusterIP:80
        |
        v
iptables ClusterIP service chain
        |
        v
Pod backend IP:targetPort
```

Diagram:

```text
                    host namespace

  client
    |
    | HTTP :7780 / HTTPS :7443
    v
  +------------------------------------------------+
  | Condenser embedded ingress gateway             |
  |                                                |
  |  1. select cert by SNI for HTTPS               |
  |  2. match Host header                          |
  |  3. match path, longest Prefix wins            |
  |  4. proxy to ClusterIP Service                 |
  +--------------------------+---------------------+
                             |
                             v
                    ClusterIP Service
                             |
                             v
                       Pod backend IPs
```

### Ingress TLS

Raind uses an Ingress-specific CA, separate from the client certificate CA.

```text
/etc/raind/ingress/certs/
  raindIngressCA.crt
  raindIngressCA.key
  hosts/
    web.local/
      tls.crt
      tls.key
```

When an Ingress includes `spec.tls[].hosts`, Condenser issues server certificates for those hosts. Certificates include the host names as DNS SANs. The HTTPS gateway uses SNI to select the certificate.

```text
TLS ClientHello SNI=web.local
        |
        v
GetCertificate(web.local)
        |
        v
/etc/raind/ingress/certs/hosts/web.local/tls.crt
```

The certificates are Raind-local certificates. They are not publicly trusted unless the Raind Ingress CA is added to the client trust store.

## End-to-end request flows

### Pod-to-Service request

```text
Pod A
  |
  | http://<clusterIP>:80
  v
iptables service chain
  |
  +--> Pod B:targetPort
  +--> Pod C:targetPort
```

### Host-to-NodePort request

```text
host client
  |
  | http://127.0.0.1:<nodePort>
  v
iptables NodePort rule
  |
  v
Service chain
  |
  v
Pod backend
```

### Host-to-Ingress request

```text
host client
  |
  | Host: web.local
  | :7780 or :7443
  v
Condenser ingress gateway
  |
  v
ClusterIP Service
  |
  v
Pod backend
```

### Container DNS request

```text
container
  |
  | DNS query to bridge gateway :53
  v
iptables DNAT on bridge
  |
  v
10.166.254.254:1053
  |
  v
Raind DNS proxy
  |
  v
upstream resolver
```

## iptables objects

Raind uses iptables NAT chains/rules for:

```text
- runtime subnet MASQUERADE
- DNS redirect per bridge
- standalone container port forwarding
- NodePort Service forwarding
- ClusterIP Service forwarding
- Service endpoint load balancing
```

Service-specific chains use names shaped like:

```text
RAIND-SVC-<service-id-prefix>-<port>
```

Endpoint load balancing uses statistic/random rules for all but the last endpoint, with the final endpoint as fallback.

## Design notes

- Raind networking is single-host by design.
- Service semantics are Kubernetes-style but not fully Kubernetes-compatible.
- `ClusterIP` is the default Service type for safer internal-only exposure.
- `NodePort` must be explicitly requested for host exposure.
- Ingress is not a Pod. It is embedded in Condenser and routes to ClusterIP Services.
- DNS traffic is redirected through the Raind DNS proxy for visibility.
- Namespace networks are backed by Linux bridges and are managed by IPAM.

## Useful inspection commands

```sh
raind network ls
raind resource namespace ls
raind resource service ls --namespace demo
raind resource ingress ls --namespace demo

ip addr show
ip route
sudo iptables -t nat -nvL
sudo iptables -t nat -S PREROUTING
sudo iptables -t nat -S OUTPUT
sudo iptables -t nat -S POSTROUTING
```
