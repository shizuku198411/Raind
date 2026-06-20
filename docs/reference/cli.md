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
raind container attach <container-id>
raind container exec [--tty] <container-id> [command]
raind container logs [--line <n>] [--pager] <container-id>
raind container inspect <container-id-or-name> [--json]
```

Common create and run flags:

```sh
--name <name>
--network <network>
-p, --publish <host-port:container-port[:protocol]>
-v, --volume <host-path:container-path>
-e, --env <KEY=VALUE>
--device <SRC[:DST[:rwm]]>
--cap-add <CAP_NAME>
--cap-drop <CAP_NAME>
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

Generate reviewable drafts from existing runtime state:

```sh
raind promote container <container-id-or-name> --to bottle -o bottle.yaml
raind promote container <container-a> <container-b> --to bottle -o bottle.yaml
raind promote container <container-id-or-name> --to bottle --stdout
raind promote container <container-id-or-name> --to bottle -o bottle.yaml --force
```

Useful flags:

```sh
--service-name <name>
--bottle-name <name>
--include-image-env
-o, --output <path>
--stdout
--force
```

The generated bottle.yaml is a draft. Secret-like environment variables are redacted into TODO comments. When multiple containers are promoted, service names are derived from container names and simple service-name references in environment values are converted into `depends_on` entries while keeping the environment values unchanged.

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

`raind resource` also supports a kubectl-style command layout. This means a
shell alias like `alias kubectl='raind resource'` can be used for the supported
resource operations:

```sh
raind resource get pods
raind resource get deployments --namespace <namespace>
raind resource describe pvc <pvc-id|name> --namespace <namespace>
raind resource delete pod <pod-id>
raind resource delete pvc <pvc-id|name> --namespace <namespace>
raind resource create namespace <namespace>
raind resource scale deployment <deployment-id> --replicas <n>
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

Pod commands:

```sh
raind resource pod create --name <name> [--namespace <namespace>] [--uid <uid>]
raind resource pod ls
raind resource pod ls --namespace <namespace>
raind resource pod start <pod-id>
raind resource pod stop <pod-id>
raind resource pod rm <pod-id>
```

Pod create also supports repeated labels and annotations:

```sh
--label <key=value>
--annotation <key=value>
```

ReplicaSet commands:

```sh
raind resource replicaset ls
raind resource replicaset ls --namespace <namespace>
raind resource replicaset show <replicaset-id>
raind resource replicaset scale --replicas <n> <replicaset-id>
raind resource replicaset rm <replicaset-id>
```

The short alias `rs` can be used for `replicaset`:

```sh
raind resource rs ls
```

Deployment commands:

```sh
raind resource deployment ls
raind resource deployment ls --namespace <namespace>
raind resource deployment show <deployment-id>
raind resource deployment scale --replicas <n> <deployment-id>
raind resource deployment rm <deployment-id>
```

The short alias `deploy` can be used for `deployment`:

```sh
raind resource deploy ls
```

Service commands:

```sh
raind resource service create -f <service.yaml>
raind resource service ls
raind resource service ls --namespace <namespace>
raind resource service show <service-id>
raind resource service rm <service-id>
```

ConfigMap commands:

```sh
raind resource configmap ls
raind resource configmap ls --namespace <namespace>
raind resource configmap show <configmap-id|name> [--namespace <namespace>]
raind resource configmap rm <configmap-id|name> [--namespace <namespace>]
```

The short alias `cm` can be used for `configmap`:

```sh
raind resource cm ls
```

Secret commands:

```sh
raind resource secret ls
raind resource secret ls --namespace <namespace>
raind resource secret show <secret-id|name> [--namespace <namespace>]
raind resource secret rm <secret-id|name> [--namespace <namespace>]
```

Secret values are not shown by default.

NetworkPolicy commands:

```sh
raind resource networkpolicy ls
raind resource networkpolicy ls --namespace <namespace>
raind resource networkpolicy show <networkpolicy-id|name> [--namespace <namespace>]
raind resource networkpolicy rm <networkpolicy-id|name> [--namespace <namespace>]
```

The short aliases `netpol` and `np` can be used for `networkpolicy`:

```sh
raind resource netpol ls
raind resource np show <name> --namespace <namespace>
```

PersistentVolumeClaim commands:

```sh
raind resource pvc ls
raind resource pvc ls --namespace <namespace>
raind resource pvc show <pvc-id|name> [--namespace <namespace>]
raind resource pvc rm <pvc-id|name> [--namespace <namespace>]
```

Namespace commands:

```sh
raind resource namespace create <namespace>
raind resource namespace create <namespace> --network <existing-network>
raind resource namespace ls
raind resource namespace show <namespace>
raind resource namespace rm <namespace>
```

The short alias `ns` can be used for `namespace`:

```sh
raind resource ns ls
```

## Policies

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

## Bottles

```sh
raind bottle create -f <bottle.yaml>
raind bottle ls
raind bottle show <bottle-id-or-name>
raind bottle start <bottle-id-or-name>
raind bottle stop <bottle-id-or-name>
raind bottle rm <bottle-id-or-name>
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
