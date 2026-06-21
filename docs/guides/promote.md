# Promote Workflow

`raind promote` helps you move an application through the same stages many projects already use during development:

```text
single container test
  -> multi-service Bottle test
  -> Kubernetes-style resource validation
```

The important part is that Promote is runtime-aware. It does not only translate one configuration file into another file. It starts from something that actually ran in Raind, reads the observed runtime state, and then generates a reviewable draft for the next stage.

```text
actual run
  -> observed runtime state
  -> reviewable deployment draft
```

Promote currently supports two paths:

```sh
raind promote container <container...> --to bottle -o bottle/bottle.yaml
raind promote bottle bottle/bottle.yaml --to resources -o bottle/manifests --ingress-host app.raind.local
```

The generated files are useful starting points, not final production configuration. Always review the generated review files and edit secrets, storage, ingress, and policy assumptions before applying the next stage.

## End-to-end flow

This guide walks through a WordPress + MySQL example using the full workflow:

1. Run and verify the application as normal containers.
2. Promote the running containers to a Bottlefile.
3. Review and edit the generated Bottlefile.
4. Run and verify the Bottle.
5. Promote the running Bottle to Kubernetes-style resources.
6. Review and edit the generated manifests.
7. Apply and verify the resources.

## Stage 1: test the application as containers

Start with ordinary containers. This lets you confirm the images, environment variables, port mappings, and network policy before generating any configuration.

```sh
raind container run --name mysql \
  -e MYSQL_ROOT_PASSWORD=root-password \
  -e MYSQL_DATABASE=wordpress-db \
  -e MYSQL_USER=wordpress-user \
  -e MYSQL_PASSWORD=wordpress-password \
  mysql:8.0

raind container run --name wordpress \
  -e WORDPRESS_DB_HOST=mysql \
  -e WORDPRESS_DB_NAME=wordpress-db \
  -e WORDPRESS_DB_USER=wordpress-user \
  -e WORDPRESS_DB_PASSWORD=wordpress-password \
  -p 9850:80 \
  wordpress:latest
```

Add the east-west policy that represents the expected runtime flow:

```sh
raind security policy add --type ew \
  -s wordpress \
  -d mysql \
  -p tcp --dport 3306 \
  --comment "wordpress -> db 3306/tcp"

raind security policy commit
```

Check that the containers are running and that the expected flow is observed:

```sh
raind container ls
raind security policy ls --type ew
raind logs netflow
```

At this point, confirm the application works through the published port:

```sh
curl http://<host-ip>:9850
```

## Stage 2: promote running containers to a Bottlefile

After the container-level test works, generate a Bottle draft from the running containers:

```sh
mkdir -p bottle
raind promote container wordpress mysql --to bottle --bottle-name wordpress -o bottle/bottle.yaml
```

This writes:

```text
bottle/bottle.yaml
bottle/REVIEW_BOTTLE.md
```

The generated Bottlefile keeps runtime information such as image names, commands, published ports, inferred dependencies, and Raind security policy drafts.

Example output shape:

```yaml
bottle:
  name: "wordpress"

services:
  mysql:
    image: "mysql:8.0"
    command:
      - "docker-entrypoint.sh"
      - "mysqld"
    # TODO: secret candidate redacted from container env: MYSQL_PASSWORD
    # env example: MYSQL_PASSWORD=<redacted>
    # TODO: secret candidate redacted from container env: MYSQL_ROOT_PASSWORD
    # env example: MYSQL_ROOT_PASSWORD=<redacted>
    env:
      - "MYSQL_DATABASE=wordpress-db"
      - "MYSQL_USER=wordpress-user"

  wordpress:
    image: "wordpress:latest"
    command:
      - "docker-entrypoint.sh"
      - "apache2-foreground"
    # TODO: secret candidate redacted from container env: WORDPRESS_DB_PASSWORD
    # env example: WORDPRESS_DB_PASSWORD=<redacted>
    env:
      - "WORDPRESS_DB_HOST=mysql"
      - "WORDPRESS_DB_NAME=wordpress-db"
      - "WORDPRESS_DB_USER=wordpress-user"
    ports:
      - "9850:80"
    depends_on:
      - "mysql"

policies:
  - type: "east-west"
    source: "wordpress"
    destination: "mysql"
    protocol: "tcp"
    dest_port: 3306
    comment: "wordpress -> db 3306/tcp"
```

### Review before running the Bottle

Open both generated files before starting the Bottle:

```sh
cat bottle/REVIEW_BOTTLE.md
$EDITOR bottle/bottle.yaml
```

Review these items:

- `image`: confirm the generated image references are the ones you want to keep.
- `command`: confirm the runtime command should be preserved in the Bottlefile.
- `env`: secret-like values are redacted by default. Restore local test values or replace them with safe values before running the Bottle.
- `ports`: confirm the host-to-container port mappings.
- `depends_on`: confirm inferred service dependencies.
- `policies`: confirm the generated east-west policies match observed runtime traffic.
- `mount`: host paths are machine-specific and should be reviewed carefully.

For the WordPress example, restore the secret values for the local Bottle test:

```yaml
services:
  mysql:
    env:
      - "MYSQL_DATABASE=wordpress-db"
      - "MYSQL_USER=wordpress-user"
      - "MYSQL_PASSWORD=wordpress-password"
      - "MYSQL_ROOT_PASSWORD=root-password"

  wordpress:
    env:
      - "WORDPRESS_DB_HOST=mysql"
      - "WORDPRESS_DB_NAME=wordpress-db"
      - "WORDPRESS_DB_USER=wordpress-user"
      - "WORDPRESS_DB_PASSWORD=wordpress-password"
```

## Stage 3: test the application as a Bottle

Start the Bottle from the reviewed file:

```sh
cd bottle
raind bottle up
```

`raind bottle up` is a wrapper for creating and starting a Bottle. By default it looks for `bottle.yaml`, then `compose.yaml`. Use `-f` when the file has a different name:

```sh
raind bottle up -f ./bottle.yaml
```

Check the running Bottle:

```sh
raind bottle show wordpress
raind security policy ls --type ew
raind logs netflow
```

Confirm the application still works through the Bottle-published port:

```sh
curl http://<host-ip>:9850
```

The important validation here is that the multi-service application works as a Bottle before generating Kubernetes-style resources. Resource promotion requires the Bottle to be running so that Raind can use actual runtime state instead of only converting the file statically.

## Stage 4: promote the running Bottle to resources

Once the Bottle works, generate Kubernetes-style resource drafts:

```sh
raind promote bottle bottle.yaml --to resources -o manifests --ingress-host wordpress.raind.local
```

This writes a directory like this:

```text
manifests/
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

Only applicable files are generated. For example, `03-pvcs.yaml` is omitted when there are no mounts, and `06-ingress.yaml` is generated when `--ingress-host` is provided.

Promote uses the running Bottle state and the reviewed Bottlefile to generate Raind-compatible Kubernetes-style resources:

- Bottle services become `Deployment` resources.
- Non-secret environment variables become per-service `ConfigMap` resources.
- Secret-like environment variables become `Secret` examples with placeholder values.
- Bottle ports become `ClusterIP` `Service` resources.
- Internal destination ports from policies can also produce service ports, such as `mysql:3306`.
- Service-name environment values are promoted to Kubernetes-style service DNS names, such as `mysql.wordpress.svc.cluster.local`.
- Published HTTP-like ports can become an `Ingress` when `--ingress-host` is provided.
- Bottle policies become podSelector-based `NetworkPolicy` drafts when they fit Raind's current subset.

Example generated environment conversion:

```yaml
data:
  WORDPRESS_DB_HOST: "mysql.wordpress.svc.cluster.local"
  WORDPRESS_DB_NAME: "wordpress-db"
  WORDPRESS_DB_USER: "wordpress-user"
```

Example generated Service for MySQL:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: "mysql"
  namespace: "wordpress"
spec:
  type: ClusterIP
  selector:
    app: "mysql"
  ports:
    - port: 3306
      targetPort: 3306
      protocol: "TCP"
```

## Stage 5: review and edit generated resources

Before applying the resources, review the report and manifests:

```sh
cat manifests/REVIEW.md
$EDITOR manifests/02-secret.example.yaml
$EDITOR manifests/all.yaml
```

Review these items:

- `REVIEW.md`: read the list of generated resources, inferred values, skipped fields, and items that need human review.
- `02-secret.example.yaml`: replace `<replace-me>` values before applying, or remove the Secret file from the apply path until you are ready.
- `01-configmap.yaml`: verify promoted environment values, especially service DNS names.
- `03-pvcs.yaml`: verify generated PVC names, storage sizes, and whether host-mounted paths should become persistent volumes.
- `04-deployments.yaml`: review images, commands, replicas, env references, ports, and volume mounts.
- `05-services.yaml`: review ClusterIP ports and target ports.
- `06-ingress.yaml`: review host, path, and TLS assumptions.
- `07-networkpolicies.yaml`: review traffic boundaries and destination ports.
- `all.yaml`: useful for current file-based apply flows, but it should still be reviewed like the individual files.

Promote intentionally does not infer production-only decisions such as resource requests and limits, readiness and liveness probes, rollout strategy, service accounts, RBAC, TLS certificates, cloud storage classes, or production secret management.

For the WordPress example, replace the placeholder values in `02-secret.example.yaml` before applying:

```yaml
stringData:
  MYSQL_PASSWORD: "wordpress-password"
  MYSQL_ROOT_PASSWORD: "root-password"
```

```yaml
stringData:
  WORDPRESS_DB_PASSWORD: "wordpress-password"
```

## Stage 6: apply and test the resources

Apply the combined manifest:

```sh
raind resource apply -f manifests/all.yaml
```

Or apply the generated files in order:

```sh
raind resource apply -f manifests/00-namespace.yaml
raind resource apply -f manifests/01-configmap.yaml
raind resource apply -f manifests/02-secret.example.yaml
raind resource apply -f manifests/03-pvcs.yaml
raind resource apply -f manifests/04-deployments.yaml
raind resource apply -f manifests/05-services.yaml
raind resource apply -f manifests/06-ingress.yaml
raind resource apply -f manifests/07-networkpolicies.yaml
```

Check the generated resources:

```sh
raind resource get -n wordpress deploy
raind resource get -n wordpress configmap
raind resource get -n wordpress secret
raind resource get -n wordpress service
raind resource get -n wordpress ingress
```

Expected resource shape:

```text
Deployment/mysql
Deployment/wordpress
ConfigMap/mysql-config
ConfigMap/wordpress-config
Secret/mysql-secret
Secret/wordpress-secret
Service/mysql        ClusterIP 3306->3306/tcp
Service/wordpress    ClusterIP 80->80/tcp
Ingress/wordpress    wordpress.raind.local -> wordpress:80
NetworkPolicy/allow-wordpress-to-mysql
```

Verify application access through the generated Ingress host:

```sh
curl http://<ingress-host>:7780
```

## Clean up

For generated resources:

```sh
raind resource delete -f manifests/all.yaml
```

For the Bottle:

```sh
raind bottle down
```

For the original container test resources, remove the containers and any policies you no longer need.

## Command reference

Container to Bottle:

```sh
raind promote container wordpress mysql --to bottle --bottle-name wordpress -o bottle/bottle.yaml
raind promote container wordpress mysql --to bottle --stdout
raind promote container wordpress mysql --to bottle -o bottle/bottle.yaml --force
```

Bottle lifecycle wrappers:

```sh
raind bottle up
raind bottle up -f ./bottle.yaml
raind bottle down
raind bottle down -f ./bottle.yaml
```

Bottle to resources:

```sh
raind promote bottle bottle.yaml --to resources -o manifests
raind promote bottle bottle.yaml --to resources -o manifests --ingress-host wordpress.raind.local
raind promote bottle bottle.yaml --to resources -o manifests --namespace wordpress-dev
```

## Design notes

Promote is designed around these principles:

- Runtime-aware: prefer observed runtime state when available.
- Reviewable: generate drafts and reports instead of pretending the output is production-ready.
- Safe by default: redact or placeholder secret-like values.
- Deterministic: keep output stable so generated diffs can be reviewed.
- Compatible with Raind's current Kubernetes-style subset: generate what Raind can apply and validate locally.
