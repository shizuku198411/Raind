# CLI Reference

This is a high-level command map for `raind`. For exact flags and current help text, run:

```sh
raind --help
raind <command> --help
raind <command> <subcommand> --help
```

## Global

```sh
raind --version
raind --help
raind completion bash
raind completion zsh
```

## Images

```sh
raind image pull <image:tag>
raind image pull --os linux --arch amd64 <image:tag>
raind image build -t <repo/name:tag> <context-path>
raind image build -f <Dockerfile-or-Dripfile> -t <repo/name:tag> <context-path>
raind image build --dockerfile <path> -t <repo/name:tag> <context-path>
raind image build --dripfile <path> -t <repo/name:tag> <context-path>
raind image ls
raind image rm <image:tag>
```

## Containers

```sh
raind container create [flags] <image:tag> [command]
raind container run [flags] <image:tag> [command]
raind container start [--tty] <container-id>
raind container stop <container-id>
raind container rm <container-id>
raind container ls
raind container ls --all
raind container ls --include-pod
raind container attach <container-id>
raind container exec [--tty] <container-id> [command]
raind container logs [--line <n>] [--pager] <container-id>
raind container inspect <container-id-or-name> [--json]
```

Common create and run flags:

```sh
--name <name>
--network <network>
--pod <pod-id>                  # create only
-p, --publish <host-port:container-port[:protocol]>
-v, --volume <host-path:container-path>
-e, --env <KEY=VALUE>
--device <SRC[:DST[:rwm]]>
--cap-add <CAP_NAME>
--cap-drop <CAP_NAME>
--security-profile <default|dev|deploy|custom-name>
--rootless
--rootless-mode <shifted-root|login-root>
-t, --tty
-i, --interactive
```

`container run` also supports:

```sh
--rm
```

## Promote

Generate reviewable drafts from existing runtime state.

### Containers to Bottle / Compose drafts

```sh
raind promote container <container-id-or-name> --to bottle
raind promote container <container-a> <container-b> --to bottle
raind promote container <container-id-or-name> --to bottle -o raind_promote/bottle/bottle.yaml
raind promote container <container-id-or-name> --to bottle --stdout
```

Useful flags:

```sh
--service-name <name>
--bottle-name <name>
--include-image-env
-o, --output <path>             # bottle.yaml output path
--stdout                        # writes bottle.yaml only
```

Container promotion writes a Bottle draft and, when not using `--stdout`, also writes a Compose-style draft:

```text
raind_promote/bottle/bottle.yaml
raind_promote/bottle/REVIEW.md
raind_promote/compose/compose.yaml
```

Secret-like environment variables are redacted in the reviewable drafts. When multiple containers are promoted, service names are derived from container names and simple service-name references in environment values are converted into `depends_on` entries while keeping the environment values unchanged.

### Bottle to Resource drafts

```sh
raind promote bottle <Bottlefile> --to resources
raind promote bottle <Bottlefile> --to resources -o raind_promote/resources
raind promote bottle <Bottlefile> --namespace <namespace>
raind promote bottle <Bottlefile> --ingress-host <host>
```

Useful flags:

```sh
--to resources
-o, --output <directory>
--namespace <namespace>
--ingress-host <host>
```

### Promote Strategy

Run an end-to-end Promote Strategy workflow from `raind-strategy.yaml`:

```sh
raind promote strategy
raind promote strategy -f <strategy.yaml>
raind promote strategy --dry-run
raind promote strategy --until <container|bottle-draft|bottle|resources-draft>
raind promote strategy --namespace <namespace>
raind promote strategy --ingress-host <host>
```

Strategy promotion targets are inferred from the stages present in the strategy file. Reviewable outputs are written under `raind_promote/`, while internal temporary files are used for runtime validation.

## Bottles

```sh
raind bottle create -f <bottle.yaml-or-compose.yaml>
raind bottle up [-f <bottle.yaml-or-compose.yaml>]
raind bottle down [-f <bottle.yaml-or-compose.yaml>]
raind bottle ls
raind bottle show <bottle-id-or-name>
raind bottle start <bottle-id-or-name>
raind bottle stop <bottle-id-or-name>
raind bottle rm <bottle-id-or-name>
```

`bottle up` creates and starts a Bottle from YAML. `bottle down` stops and removes the Bottle described by YAML. When `-f` is omitted, Raind looks for `bottle.yaml` and then `compose.yaml`.

## Networks

```sh
raind network create <network-name>
raind network ls
raind network rm <network-name>
```

## Resources

Apply or remove resources from YAML:

```sh
raind resource apply -f <resource.yaml>
raind resource delete -f <resource.yaml>
raind resource rm -f <resource.yaml>
```

`raind resource` also supports a kubectl-style command layout. This means a shell alias like `alias kubectl='raind resource'` can be used for supported resource operations:

```sh
raind resource get pods
raind resource get deployments --namespace <namespace>
raind resource describe pvc <pvc-id|name> --namespace <namespace>
raind resource delete pod <pod-id>
raind resource delete pvc <pvc-id|name> --namespace <namespace>
raind resource create namespace <namespace>
raind resource create namespace <namespace> --network <existing-network>
raind resource scale deployment <deployment-id> --replicas <n>
raind resource scale rs <replicaset-id> --replicas <n>
```

Resource aliases follow common kubectl names:

```sh
po/pod/pods
rs/replicaset/replicasets
deploy/deployment/deployments
svc/service/services
cm/configmap/configmaps
secret/secrets
ns/namespace/namespaces
netpol/np/networkpolicy/networkpolicies
pvc/persistentvolumeclaim/persistentvolumeclaims
ing/ingress/ingresses
```

Many resource list commands support namespace filtering and watch-style repeated output:

```sh
--namespace <namespace>
-n <namespace>
--wait
-w
--watch
```

### Pod resources

```sh
raind resource pod
raind resource pod --namespace <namespace>
raind resource pod --wait
raind resource pod create --name <name> [--namespace <namespace>] [--uid <uid>]
raind resource pod create --name <name> --label <key=value> --annotation <key=value>
raind resource pod ls
raind resource pod ls --namespace <namespace>
raind resource pod start <pod-id>
raind resource pod stop <pod-id>
raind resource pod rm <pod-id>
```

### ReplicaSet resources

```sh
raind resource replicaset
raind resource replicaset ls
raind resource replicaset ls --namespace <namespace>
raind resource replicaset show <replicaset-id>
raind resource replicaset scale --replicas <n> <replicaset-id>
raind resource replicaset rm <replicaset-id>
raind resource rs ls
```

### Deployment resources

```sh
raind resource deployment
raind resource deployment ls
raind resource deployment ls --namespace <namespace>
raind resource deployment show <deployment-id>
raind resource deployment scale --replicas <n> <deployment-id>
raind resource deployment rm <deployment-id>
raind resource deploy ls
```

Top-level deployment commands are also available:

```sh
raind deployment ls
raind deployment ls --namespace <namespace>
raind deployment show <deployment-id>
raind deployment scale --replicas <n> <deployment-id>
raind deployment rm <deployment-id>
```

The top-level `deployment` command also has the alias `deploy`.

### Service resources

```sh
raind resource service
raind resource service create -f <service.yaml>
raind resource service ls
raind resource service ls --namespace <namespace>
raind resource service show <service-id>
raind resource service rm <service-id>
```

### Ingress resources

```sh
raind resource ingress
raind resource ingress ls
raind resource ingress ls --namespace <namespace>
raind resource ing ls
```

### ConfigMap resources

```sh
raind resource configmap
raind resource configmap ls
raind resource configmap ls --namespace <namespace>
raind resource configmap show <configmap-id|name> [--namespace <namespace>]
raind resource configmap rm <configmap-id|name> [--namespace <namespace>]
raind resource cm ls
```

### Secret resources

```sh
raind resource secret
raind resource secret ls
raind resource secret ls --namespace <namespace>
raind resource secret show <secret-id|name> [--namespace <namespace>]
raind resource secret rm <secret-id|name> [--namespace <namespace>]
```

Secret values are not shown by default.

### NetworkPolicy resources

```sh
raind resource networkpolicy
raind resource networkpolicy ls
raind resource networkpolicy ls --namespace <namespace>
raind resource networkpolicy show <networkpolicy-id|name> [--namespace <namespace>]
raind resource networkpolicy rm <networkpolicy-id|name> [--namespace <namespace>]
raind resource netpol ls
raind resource np show <name> --namespace <namespace>
```

### PersistentVolumeClaim resources

```sh
raind resource persistentvolumeclaim
raind resource persistentvolumeclaim ls
raind resource persistentvolumeclaim ls --namespace <namespace>
raind resource persistentvolumeclaim show <pvc-id|name> [--namespace <namespace>]
raind resource persistentvolumeclaim rm <pvc-id|name> [--namespace <namespace>]
raind resource pvc ls
raind resource pvc show <pvc-id|name> [--namespace <namespace>]
```

### Namespace resources

```sh
raind resource namespace create <namespace>
raind resource namespace create <namespace> --network <existing-network>
raind resource namespace ls
raind resource namespace show <namespace>
raind resource namespace rm <namespace>
raind resource ns ls
```

## Security

### Policies

```sh
raind security policy add --type <ew|ns-obs|ns-enf> [flags]
raind security policy ls --type <ew|ns-obs|ns-enf>
raind security policy rm <policy-id>
raind security policy commit
raind security policy revert
raind security policy ns-mode <observe|enforce>
```

Policy add flags:

```sh
-s, --source <container-name>
-d, --destination <container-name>
-p, --protocol <protocol>
--dport <port>
--comment <text>
```

### Security profiles

```sh
raind security profile ls
raind security profile show <name>
raind security profile register -f <profile.yaml>
raind security profile delete <name>
raind security profile rm <name>
```

## Logs

```sh
raind logs netflow
raind logs netflow --line <n>
raind logs netflow --pager
raind logs netflow --json
raind logs netflow -t <container-or-address>
```

## Rootless containers

```sh
raind container run --name shifted --rootless alpine:latest /bin/sh -c 'id; sleep 60'
raind container run --name login-root --rootless-mode login-root alpine:latest /bin/sh -c 'id; sleep 60'
```

See [Rootless modes](rootless-modes.md) and [Rootless containers](../guides/rootless-containers.md).
