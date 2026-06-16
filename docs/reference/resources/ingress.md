# Raind - Ingress

An `Ingress` provides HTTP and HTTPS host/path routing for Services.

Raind implements Ingress as an embedded gateway inside `condenser`. It does not create a Kubernetes-style ingress controller Pod and it does not create one ingress Pod per manifest. Ingress manifests are stored as routing rules for the built-in gateway.

## Supported Resource

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
```

`apiVersion` is parsed but not strictly version-validated. The supported `kind` value is `Ingress`.

## Runtime Model

Raind Ingress routes traffic as follows:

```text
client
  -> condenser embedded ingress gateway
    -> Ingress host/path rule
      -> ClusterIP Service
        -> Service iptables L4 load balancing
          -> Pod backend
```

The Ingress gateway is a host-level gateway managed by `condenser`.

| Gateway | Default address | Environment override | Purpose |
|---|---|---|---|
| HTTP | `:7780` | `RAIND_INGRESS_HTTP_ADDR` | Plain HTTP host/path routing. |
| HTTPS | `:7443` | `RAIND_INGRESS_HTTPS_ADDR` | TLS host/path routing using SNI and Raind-managed certificates. |

Ingress backends should normally point to `ClusterIP` Services. The Ingress gateway proxies HTTP traffic to the selected Service ClusterIP and port.

## Supported Fields

### Metadata

| Field | Required | Description |
|---|---:|---|
| `metadata.name` | yes | Ingress name. |
| `metadata.namespace` | no | Namespace. Defaults to `default`. |

### Spec

| Field | Required | Description |
|---|---:|---|
| `spec.rules` | yes | Host/path routing rules. At least one rule is required. |
| `spec.rules[].host` | yes for useful routing | HTTP host name. Stored in lowercase. |
| `spec.rules[].http.paths` | yes | HTTP path rules for the host. At least one path is required per rule. |
| `spec.rules[].http.paths[].path` | no | Path to match. Defaults to `/`. Must start with `/`. |
| `spec.rules[].http.paths[].pathType` | no | Supported values: `Prefix`, `Exact`. Defaults to `Prefix`. |
| `spec.rules[].http.paths[].backend.service.name` | yes | Backend Service name in the same namespace. |
| `spec.rules[].http.paths[].backend.service.port.number` | yes | Backend Service port number. |
| `spec.tls[].hosts` | no | Hosts for which Raind should issue managed TLS certificates. |
| `spec.tls[].secretName` | no | Accepted for Kubernetes-style compatibility, but not used yet. |

Named backend service ports are not supported yet. Use `backend.service.port.number`.

## HTTP Ingress

A minimal HTTP Ingress routes by Host header and path.

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: demo-ingress
  namespace: demo
spec:
  rules:
    - host: demo.local
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: demo-svc
                port:
                  number: 80
```

Example request:

```sh
curl -H 'Host: demo.local' http://127.0.0.1:7780/
```

## HTTPS Ingress

When `spec.tls[].hosts` is set, Raind creates a managed certificate for each host and serves HTTPS through the embedded gateway.

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: demo-ingress
  namespace: demo
spec:
  tls:
    - hosts:
        - demo.local
  rules:
    - host: demo.local
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: demo-svc
                port:
                  number: 80
```

Example request:

```sh
curl -k --resolve demo.local:7443:127.0.0.1 https://demo.local:7443/
```

`-k` is needed unless the Raind Ingress CA is trusted by the client system.

## TLS Certificate Model

Raind uses a dedicated Ingress CA, separate from client certificate issuance.

```text
/etc/raind/ingress/certs/
  raindIngressCA.crt
  raindIngressCA.key
  hosts/
    demo.local/
      tls.crt
      tls.key
```

For each TLS host, Raind issues a server certificate with the host set as a DNS SAN. The HTTPS gateway uses SNI to select the matching certificate.

Important notes:

- Ingress certificates are issued by Raind's local Ingress CA.
- They are not public CA certificates.
- Browsers and clients will not trust them unless the Raind Ingress CA is installed into the client trust store.
- The Ingress CA is intentionally separate from Raind's client certificate machinery.
- Certificates are cleaned up when the last Ingress referencing a host is removed.

## Path Matching

Raind supports two path types.

| `pathType` | Behavior |
|---|---|
| `Prefix` | Matches requests whose path begins with the configured path. |
| `Exact` | Matches only the exact path. |

When multiple paths match, Raind chooses the longest matching path first. This makes more specific routes win over broader routes.

Example:

```yaml
paths:
  - path: /api
    pathType: Prefix
    backend:
      service:
        name: api-svc
        port:
          number: 80
  - path: /
    pathType: Prefix
    backend:
      service:
        name: web-svc
        port:
          number: 80
```

A request to `/api/users` routes to `api-svc`; a request to `/` routes to `web-svc`.

## Complete Example

This example creates a Namespace, Deployment, ClusterIP Service, and HTTP/HTTPS Ingress.

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
  name: demo-svc
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
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: demo-ingress
  namespace: demo
spec:
  tls:
    - hosts:
        - demo.local
  rules:
    - host: demo.local
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: demo-svc
                port:
                  number: 80
```

Apply:

```sh
raind resource apply -f ingress-demo.yaml
```

Example output:

```text
namespace: demo applied (network: rns...)
deployment: 01kv... applied
service: 01kv... applied
ingress: 01kv... applied (tls: enabled, hosts: demo.local)
```

HTTP:

```sh
curl -H 'Host: demo.local' http://127.0.0.1:7780/
```

HTTPS:

```sh
curl -k --resolve demo.local:7443:127.0.0.1 https://demo.local:7443/
```

## CLI Usage

### Apply from Manifest

```sh
raind resource apply -f ingress.yaml
```

### List Ingresses

```sh
raind resource ingress ls
raind resource ingress ls --namespace demo
raind resource ing ls -n demo
```

Example output:

```text
INGRESS ID                    NAME          NAMESPACE  HOSTS       TLS         PATHS                 BACKENDS     CREATED
01kv...                       demo-ingress  demo       demo.local  demo.local  demo.local/(Prefix)   demo-svc:80  less than a minutes
```

### Remove from Manifest

```sh
raind resource rm -f ingress.yaml
```

Example output:

```text
ingress: 01kv... removed
```

When an Ingress is removed, Raind removes per-host TLS certificates for hosts that are no longer referenced by any remaining Ingress.

## Kubernetes Compatibility Notes

Raind uses Kubernetes-style Ingress syntax but does not implement a full Kubernetes ingress controller model.

Key differences:

- Raind does not create or require a system ingress controller Pod.
- Raind does not create one ingress Pod per Ingress manifest.
- Raind uses an embedded gateway inside `condenser`.
- Backends are expected to be Raind `ClusterIP` Services.
- TLS is managed by Raind's local Ingress CA rather than Kubernetes Secrets.
- `spec.tls[].secretName` is not used yet.
- Only HTTP/HTTPS routing is supported.
- Advanced Kubernetes Ingress features and controller-specific annotations are ignored.

## Current Limitations

- HTTP and HTTPS only.
- TLS certificates are local Raind-managed certificates, not public CA certificates.
- Wildcard hosts are not currently documented as supported.
- Backend named ports are not supported.
- `pathType: ImplementationSpecific` is not supported.
- Ingress annotations are ignored.
- No rewrite, redirect, rate limit, authentication, or middleware features yet.
- No user-provided certificate Secret support yet.
