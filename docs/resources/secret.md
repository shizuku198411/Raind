# Raind - Secret

A `Secret` stores sensitive key/value configuration for Pods, ReplicaSets, and Deployments.

Raind supports environment injection from Secrets, but normal CLI and API output does not reveal raw Secret values.

## Supported Resource

```yaml
apiVersion: v1
kind: Secret
```

## Supported Fields

### Metadata

| Field | Required | Description |
|---|---:|---|
| `metadata.name` | yes | Secret name. |
| `metadata.namespace` | no | Namespace. Defaults to `default`. |

### Secret Data

| Field | Required | Description |
|---|---:|---|
| `type` | no | Only `Opaque` is supported. Empty defaults to `Opaque`. |
| `stringData` | no | Plain string key/value pairs. |
| `data` | no | Base64-encoded key/value pairs. |

If the same key exists in both `data` and `stringData`, `stringData` wins.

## Unsupported Fields

| Field | Behavior |
|---|---|
| Non-`Opaque` secret types | Rejected. |
| Secret volume projection | Not supported yet. |
| TLS Secret integration with Ingress | Not supported yet. |

## Security Behavior

- `raind resource secret ls` shows metadata and key counts only.
- `raind resource secret show` shows key names only, not values.
- `raind resource apply -f` does not print Secret values.
- The runtime Secret store is created under `/etc/raind/store/resource/secret/sec.json` with restricted file permissions where possible.

## Apply Example

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: demo
---
apiVersion: v1
kind: Secret
metadata:
  name: db-secret
  namespace: demo
type: Opaque
stringData:
  DB_PASSWORD: password
data:
  API_TOKEN: dG9rZW4=
---
apiVersion: v1
kind: Pod
metadata:
  name: app
  namespace: demo
spec:
  containers:
    - name: app
      image: busybox:latest
      command:
        - sh
        - -c
        - env; sleep 3600
      envFrom:
        - secretRef:
            name: db-secret
      env:
        - name: DB_PASSWORD_COPY
          valueFrom:
            secretKeyRef:
              name: db-secret
              key: DB_PASSWORD
```

Apply and inspect without revealing values:

```sh
raind resource apply -f secret.yaml
raind resource secret ls --namespace demo
raind resource secret show db-secret --namespace demo
raind resource rm -f secret.yaml
```

## Environment Precedence

When both `envFrom` and explicit `env` define the same key, explicit `env` wins.

```yaml
envFrom:
  - secretRef:
      name: db-secret
env:
  - name: DB_PASSWORD
    value: local-override
```
