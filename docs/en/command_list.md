# Raind - Command List
Raind CLI is expected to run as a non-root user in the `raind` group.

## Container

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| Create | container | create | --network Network name | <image:tag> [command arg1 arg2 ...] |
|||| --volume, -v <host-dir>:<container-dir> Bind mount ||
|||| --publish, -p <sourceport>:<hostport>[:protocol] Port forward ||
|||| --env, -e <KEY=VALUE> Environment variable ||
|||| --tty, -t Attach TTY ||
|||| --interactive, -i Interactive mode ||
|||| --name <container-name> Container name ||
|||| --pod <pod-id> Attach to Pod ||

example: `raind container create -t --name web -v /mnt/web:/var/www/html -p 8080:80 nginx:latest`

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| Start | container | start | --tty, -t Attach TTY | <container-id> |

example: `raind container start -t web`

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| Stop | container | stop || <container-id> |

example: `raind container stop web`

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| Remove | container | rm || <container-id> |

example: `raind container rm web`

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| List | container | ls |||

example: `raind container ls`

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| Attach | container | attach || <container-id> |

example: `raind container attach web`

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| Run (create+start[+attach]) | container | run | --network Network name | <image:tag> [command arg1 arg2 ...] |
|||| --volume, -v <host-dir>:<container-dir> Bind mount ||
|||| --publish, -p <sourceport>:<hostport>[:protocol] Port forward ||
|||| --env, -e <KEY=VALUE> Environment variable ||
|||| --tty, -t Attach TTY ||
|||| --rm Remove on exit ||
|||| --name <container-name> Container name ||

example: `raind container run -t --rm --name web -v /mnt/web:/var/www/html -p 8080:80 nginx:latest`

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| Exec | container | exec | --tty, -t Attach TTY | <container-id> <command arg1 arg2 ...> |

example: `raind container exec -t web /bin/sh -c "echo Hello World! > hello.txt"`

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| Logs | container | logs | --line Line count | <container-id> |
|||| --pager Open with pager ||

example: `raind container logs --line 200 --pager web`

## Bottle
Bottle manages multiple containers as one group (docker-compose-like).  
See [Bottle Usage](bottle.md) for details.

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| Create | bottle | create | --file, -f <bottle-file-path> Bottle definition (*) ||

example: `raind bottle create -f ~/myapp/Dripfile.yaml`

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| Start | bottle | start || <bottle-id|bottle-name> |

example: `raind bottle start myapp`

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| Stop | bottle | stop || <bottle-id|bottle-name> |

example: `raind bottle stop myapp`

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| Remove | bottle | rm || <bottle-id|bottle-name> |

example: `raind bottle rm myapp`

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| List | bottle | ls |||

example: `raind bottle ls`

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| Show | bottle | show || <bottle-id|bottle-name> |

example: `raind bottle show myapp`

## Image

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| Pull | image | pull | --os Target OS | <image:tag> |
|||| --arch Target architecture ||

example: `raind image pull alpine:latest`

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| Build | image | build | --file, -f Context directory (*) ||
|||| --tag, -t <image:tag> Image tag (*) ||

example: `raind image build -f ~/myapp -t myapp:latest`

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| Remove | image | rm || <image:tag> |

example: `raind image rm alpine:latest`

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| List | image | ls |||

example: `raind image ls`

## Network

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| Create | network | create || <network-name> |

example: `raind network create raind0`

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| Remove | network | rm || <network-name> |

example: `raind network rm raind0`

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| List | network | ls |||

example: `raind network ls`

## Resource

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| Apply | resource | apply | --file, -f Resource YAML (*) ||

example: `raind resource apply -f path/to/manifest.yaml`

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| Remove | resource | rm | --file, -f Resource YAML (*) ||

example: `raind resource rm -f path/to/manifest.yaml`

### Resource Pod

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| Create | resource pod | create | --name, -n <pod-name> Pod name (*) ||
|||| --namespace Namespace (default: default) ||
|||| --uid UID ||
|||| --label, -l <key=value> Label ||
|||| --annotation, -a <key=value> Annotation ||

example: `raind resource pod create -n demo -l app=demo`

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| List | resource pod | ls |||

example: `raind resource pod ls`

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| Start | resource pod | start || <pod-id> |

example: `raind resource pod start <pod-id>`

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| Stop | resource pod | stop || <pod-id> |

example: `raind resource pod stop <pod-id>`

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| Remove | resource pod | rm || <pod-id> |

example: `raind resource pod rm <pod-id>`

### Resource ReplicaSet

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| List | resource replicaset | ls(get) |||

example: `raind resource replicaset ls`

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| Show | resource replicaset | show(describe) || <replicaset-id> |

example: `raind resource replicaset show <replicaset-id>`

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| Scale | resource replicaset | scale | --replicas, -r <num> replicas (*) | <replicaset-id> |

example: `raind resource replicaset scale <replicaset-id> -r 3`

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| Remove | resource replicaset | rm(delete) || <replicaset-id> |

example: `raind resource replicaset rm <replicaset-id>`

### Resource Service

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| Create | resource service | create | --file, -f Service YAML (*) ||

example: `raind resource service create -f path/to/service.yaml`

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| List | resource service | ls |||

example: `raind resource service ls`

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| Show | resource service | show || <service-id> |

example: `raind resource service show <service-id>`

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| Remove | resource service | rm || <service-id> |

example: `raind resource service rm <service-id>`

## Policy
All policy changes are not applied to the actual policy until `commit` is executed.

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| Add | policy | add | --type <ew|ns-obs|ns-enf> Policy type (*) ||
|||| --source, -s <container-name> Source container name (*) ||
|||| --destination, -d <container-name> Destination container name (*) ||
|||| --protocol, -p <icmp|tcp|udp> Protocol ||
|||| --dport <dest-port> Destination port ||
|||| --comment <comment> Comment ||

example: `raind policy add --type ew -s web -d db -p tcp --dport 3306 --comment "web->db tcp/3306"`

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| Remove | policy | rm || <policy-id> |

example: `raind policy rm 01kgtyrrnrxvncpjcmjz2rtnmq`

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| Change NS mode | policy | ns-mode || <observe|enforce> |

example: `raind policy ns-mode enforce`

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| Revert | policy | revert |||

example: `raind policy revert`

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| Commit | policy | commit |||

example: `raind policy commit`

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| List | policy | ls | --type <ew|ns-obs|ns-enf> Filter ||

example: `raind policy ls --type ew`

## Logs

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| Netflow logs | logs | netflow | --line Line count ||
|||| --pager Open with pager ||
|||| --json JSON output ||
|||| --target, -t <container-name|address> Filter ||

example: `raind logs netflow --line 200 --pager --json --target web`

## Completion

| Operation | Command Group | Subcommand | Options (* required) | Args |
|:--|:--|:--|:--|:--|
| Generate completion | completion | - || bash|zsh|fish |

example: `raind completion bash > /path/to/raind.bash`
