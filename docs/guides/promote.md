# Promote Workflow

`raind promote` generates reviewable deployment drafts from existing Raind runtime state.

The current MVP supports promoting one or more existing containers to a bottle.yaml and a REVIEW_BOTTLE.md review report:

```sh
raind promote container myapp --to bottle -o bottle.yaml
raind promote container db web --to bottle -o bottle.yaml
```

The generated bottle.yaml is a draft. Raind also writes REVIEW_BOTTLE.md next to the bottle.yaml unless --stdout is used. Review the image, command, environment variables, ports, mounts, dependencies, redacted secret candidates, and TODO comments before sharing or committing it.

## Single container to bottle.yaml

Example:

```sh
raind image build -t myapp:dev .
raind container run --name myapp -p 8080:3000 -e APP_ENV=dev myapp:dev
raind promote container myapp --to bottle -o bottle.yaml
# writes bottle.yaml and REVIEW_BOTTLE.md
```

Useful options:

```sh
raind promote container myapp --to bottle --service-name app --bottle-name myapp -o bottle.yaml
raind promote container myapp --to bottle --stdout
raind promote container myapp --to bottle -o bottle.yaml --force
```

## Multiple containers to bottle.yaml

Promote can also generate a multi-service bottle.yaml from multiple containers:

```sh
raind container run --name db mysql:latest
raind container run --name web -p 8080:80 -e MYSQL_HOST=db myapp:latest
raind promote container db web --to bottle --bottle-name myapp -o bottle.yaml
```

For multiple containers, service names are derived from container names. If an environment variable references another promoted service name, Raind adds a `depends_on` entry while preserving the original environment value.

For example, `MYSQL_HOST=db` remains `MYSQL_HOST=db` in the generated bottle.yaml, and the `web` service gets `depends_on: [db]`. Container-to-container resolution is handled by Raind DNS at runtime.

Secret-like environment variable values are not written directly. They are emitted as redacted TODO comments.

Pod member containers are intentionally not supported in the MVP because their networking and lifecycle are owned by the Pod.

## Bottlefile to Kubernetes-style resource manifests

Promote can generate a reviewable set of Raind-compatible Kubernetes-style manifests from a running Bottle. The Bottlefile supplies the bottle name, and Raind reads the running bottle state from the daemon before generating manifests:

```sh
raind bottle create bottle.yaml
raind bottle start myapp
raind promote bottle bottle.yaml --to resources -o manifests/
```

Resource promotion fails when the bottle is not running. This keeps Promote runtime-aware instead of acting as a static Bottlefile-to-YAML converter.

The generated directory may contain:

```text
00-namespace.yaml
01-configmap.yaml
02-secret.example.yaml
03-pvcs.yaml
04-deployments.yaml
05-services.yaml
06-ingress.yaml
07-networkpolicies.yaml
REVIEW.md
all.yaml
```

Only files with applicable content are generated. For example, `01-configmap.yaml` is omitted when no non-secret environment variables exist, and `06-ingress.yaml` is generated only when `--ingress-host` is provided.

The initial resource promotion intentionally targets Raind's current Kubernetes-style subset and uses runtime details such as running container image, command, port forwards, and bottle policies when available:

- Bottle services become `Deployment` manifests with `replicas: 1`.
- Bottle ports become `ClusterIP` `Service` manifests using container ports.
- Non-secret environment variables become per-service `ConfigMap` manifests.
- Secret-like environment variables become per-service `Secret` examples with placeholder values.
- Bottle mounts become PVC drafts and `volumeMounts`.
- Bottle policies become podSelector-based `NetworkPolicy` egress drafts when they fit the current Raind subset.

Host-published Bottle ports, `depends_on`, resource requests and limits, probes, rollout strategy, storage class decisions, and production secret management are not inferred automatically. Review `REVIEW.md` before applying generated manifests.

To request an Ingress draft:

```sh
raind promote bottle bottle.yaml --to resources -o manifests/ --ingress-host app.raind.local
```

Apply the generated files in order, or use the combined manifest:

```sh
raind resource apply -f manifests/all.yaml
```
