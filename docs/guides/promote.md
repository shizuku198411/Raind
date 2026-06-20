# Promote Workflow

`raind promote` generates reviewable deployment drafts from existing Raind runtime state.

The first MVP supports promoting a single container to a single-service Dripfile:

```sh
raind promote container myapp --to bottle -o Dripfile
```

The generated Dripfile is a draft. Review the image, command, environment variables, ports, mounts, and TODO comments before sharing or committing it.

## Container to Dripfile

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

Secret-like environment variable values are not written directly. They are emitted as redacted TODO comments.

Pod member containers are intentionally not supported in the MVP because their networking and lifecycle are owned by the Pod.
