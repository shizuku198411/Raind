# Raind - NetworkPolicy

A `NetworkPolicy` creates namespace-local east-west allow rules between Pods selected by labels.

Raind maps Kubernetes-style NetworkPolicy manifests to the existing Raind security policy backend. Pod selection uses Pod labels, while enforcement is applied to the Pod infra containers that own each Pod network namespace.

## Supported Resource

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
```

## Supported Fields

### Metadata

| Field | Required | Description |
|---|---:|---|
| `metadata.name` | yes | NetworkPolicy name. |
| `metadata.namespace` | no | Namespace. Defaults to `default`. |

### Spec

| Field | Required | Description |
|---|---:|---|
| `spec.podSelector.matchLabels` | no | Selects destination Pods for ingress rules and source Pods for egress rules. Empty matches all running Pods in the namespace. |
| `spec.ingress[].from[].podSelector.matchLabels` | no | Selects source Pods for ingress allow rules. |
| `spec.ingress[].ports[].protocol` | no | `TCP` or `UDP`. Defaults to `TCP`. |
| `spec.ingress[].ports[].port` | no | Destination port. Empty allows all ports for that protocol rule. |
| `spec.egress[].to[].podSelector.matchLabels` | no | Selects destination Pods for egress allow rules. |
| `spec.egress[].ports[].protocol` | no | `TCP` or `UDP`. Defaults to `TCP`. |
| `spec.egress[].ports[].port` | no | Destination port. Empty allows all ports for that protocol rule. |

## Unsupported Fields

| Field | Behavior |
|---|---|
| `namespaceSelector` | Rejected. NetworkPolicy is namespace-local in the current implementation. |
| `ipBlock` | Rejected. |
| Named ports | Rejected by manifest parsing. Use numeric ports. |
| Cross-namespace policy | Not supported yet. |

## Runtime Behavior

When a NetworkPolicy is applied, Raind:

1. Selects currently running Pods in the NetworkPolicy namespace.
2. Resolves each selected Pod to its running infra container.
3. Generates Raind east-west security policy rules owned by the NetworkPolicy.
4. Commits the security policy backend immediately.
5. Stores generated rule IDs in `/etc/raind/store/resource/networkpolicy/netpol.json`.

Removing the NetworkPolicy removes only the generated rules owned by that NetworkPolicy and commits the backend again.

The current implementation generates rules from Pods that are running at apply time. Reconciliation for Pods created after the NetworkPolicy is applied is planned for a later phase.

## Ingress Example

Allow Pods labeled `role=client` to connect to Pods labeled `role=server` on TCP port `8080` inside the same namespace.

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-client
  namespace: demo
spec:
  podSelector:
    matchLabels:
      role: server
  ingress:
    - from:
        - podSelector:
            matchLabels:
              role: client
      ports:
        - protocol: TCP
          port: 8080
```

## Egress Example

Allow Pods labeled `role=client` to connect to Pods labeled `role=server` on UDP port `5353`.

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-client-egress
  namespace: demo
spec:
  podSelector:
    matchLabels:
      role: client
  egress:
    - to:
        - podSelector:
            matchLabels:
              role: server
      ports:
        - protocol: UDP
          port: 5353
```

## CLI Usage

Apply from a manifest:

```sh
raind resource apply -f networkpolicy.yaml
```

List NetworkPolicies:

```sh
raind resource networkpolicy ls --namespace demo
raind resource netpol ls -n demo
```

Show a NetworkPolicy:

```sh
raind resource networkpolicy show allow-client --namespace demo
```

Remove from a manifest:

```sh
raind resource rm -f networkpolicy.yaml
```

Remove directly:

```sh
raind resource networkpolicy rm allow-client --namespace demo
```
