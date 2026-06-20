# Promote Workflow

`raind promote` generates reviewable deployment drafts from existing Raind runtime state.

The current MVP supports promoting one or more existing containers to a Dripfile:

```sh
raind promote container myapp --to bottle -o Dripfile
raind promote container db web --to bottle -o Dripfile
```

The generated Dripfile is a draft. Review the image, command, environment variables, ports, mounts, dependencies, and TODO comments before sharing or committing it.

## Single container to Dripfile

Example:

```sh
raind image build -t myapp:dev .
raind container run --name myapp -p 8080:3000 -e APP_ENV=dev myapp:dev
raind promote container myapp --to bottle -o Dripfile
```

Useful options:

```sh
raind promote container myapp --to bottle --service-name app --bottle-name myapp -o Dripfile
raind promote container myapp --to bottle --stdout
raind promote container myapp --to bottle -o Dripfile --force
```

## Multiple containers to Dripfile

Promote can also generate a multi-service Dripfile from multiple containers:

```sh
raind container run --name db mysql:latest
raind container run --name web -p 8080:80 -e MYSQL_HOST=db myapp:latest
raind promote container db web --to bottle --bottle-name myapp -o Dripfile
```

For multiple containers, service names are derived from container names. If an environment variable references another promoted service name, Raind adds a `depends_on` entry while preserving the original environment value.

For example, `MYSQL_HOST=db` remains `MYSQL_HOST=db` in the generated Dripfile, and the `web` service gets `depends_on: [db]`. Container-to-container resolution is handled by Raind DNS at runtime.

Secret-like environment variable values are not written directly. They are emitted as redacted TODO comments.

Pod member containers are intentionally not supported in the MVP because their networking and lifecycle are owned by the Pod.
