# Raind - ConfigMap

A `ConfigMap` stores non-sensitive key/value configuration for Pods, ReplicaSets, and Deployments.

## Supported Resource

```yaml
apiVersion: v1
kind: ConfigMap
```

## Supported Fields

### Metadata

| Field | Required | Description |
|---|---:|---|
| `metadata.name` | yes | ConfigMap name. |
| `metadata.namespace` | no | Namespace. Defaults to `default`. |

### Data

| Field | Required | Description |
|---|---:|---|
| `data` | no | String key/value pairs injected into container environment variables. |

## Unsupported Fields

| Field | Behavior |
|---|---|
| `binaryData` | Ignored with a warning. |
| `immutable` | Accepted with a warning. Immutable update semantics are not enforced. |
| Volume projection | Not supported yet. |

## Apply Example

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: demo
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
  namespace: demo
data:
  APP_ENV: local
  LOG_LEVEL: debug
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
        - configMapRef:
            name: app-config
      env:
        - name: APP_ENV_COPY
          valueFrom:
            configMapKeyRef:
              name: app-config
              key: APP_ENV
```

Apply and remove:

```sh
raind resource apply -f configmap.yaml
raind resource configmap ls --namespace demo
raind resource configmap show app-config --namespace demo
raind resource rm -f configmap.yaml
```

## Environment Precedence

When both `envFrom` and explicit `env` define the same key, explicit `env` wins.

```yaml
envFrom:
  - configMapRef:
      name: app-config
env:
  - name: LOG_LEVEL
    value: trace
```
