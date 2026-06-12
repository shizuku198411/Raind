# Usage Examples

These examples assume the condenser service is running and the current user belongs to the `raind` group.

## Check the Runtime

```sh
raind --version
raind image ls
raind container ls
raind network ls
```

## Pull an Image

```sh
raind image pull nginx:latest
```

Pull for a specific platform:

```sh
raind image pull --os linux --arch amd64 nginx:latest
```

## Run a Container

Run a container and publish a port:

```sh
raind container run --name web -p 8080:80 nginx:latest
```

Run a short-lived command and remove the container when it exits:

```sh
raind container run --rm busybox:latest /bin/sh -c 'echo hello from raind'
```

Pass environment variables:

```sh
raind container run --name app -e APP_ENV=dev busybox:latest env
```

Mount a host directory:

```sh
raind container run --name data -v /tmp/raind-data:/data busybox:latest ls /data
```

## Manage Containers

```sh
raind container ls
raind container logs --line 100 web
raind container exec web /bin/sh
raind container stop web
raind container rm web
```

## Manage Networks

```sh
raind network create appnet
raind network ls
raind container run --name app --network appnet nginx:latest
raind network rm appnet
```

## Apply Resource YAML

```sh
raind resource apply -f examples/app.yaml
raind resource pod ls
raind resource replicaset ls
raind resource deployment ls
raind resource service ls
```

Remove the same resources:

```sh
raind resource rm -f examples/app.yaml
```

## Scale a ReplicaSet

```sh
raind resource replicaset ls
raind resource replicaset scale --replicas 3 <replicaset-id>
raind resource replicaset show <replicaset-id>
```

## Scale a Deployment

```sh
raind resource deployment ls
raind resource deployment scale --replicas 3 <deployment-id>
raind resource deployment show <deployment-id>
```

## Work with Policies

List policies:

```sh
raind policy ls --type ew
raind policy ls --type ns-obs
raind policy ls --type ns-enf
```

Add and commit an east-west policy:

```sh
raind policy add --type ew \
  --source frontend \
  --destination backend \
  --protocol tcp \
  --dport 8080 \
  --comment 'allow frontend to backend'

raind policy commit
```

Revert pending policy changes:

```sh
raind policy revert
```

Change namespace policy mode:

```sh
raind policy ns-mode observe
raind policy ns-mode enforce
```

## View Netflow Logs

```sh
raind logs netflow --line 50
raind logs netflow --json
raind logs netflow -t web
```

## Workshop Manual Flow

Use Workshop when you want to test runtime behavior without touching a host installation:

```sh
workshop run raind-dev -- dev-setup
workshop shell raind-dev
raind image ls
raind container run --name web -p 8080:80 nginx:latest
```

Clean up when finished:

```sh
workshop run raind-dev -- dev-cleanup
```
