# Security profiles

Raind security profiles bundle container security settings into a reusable profile.

A profile currently resolves the following settings before a container spec is handed to Droplet:

- Linux capabilities
- seccomp deny filter
- AppArmor profile name

Use profiles when you want to apply the same security posture to multiple containers without repeating low-level capability, seccomp, or AppArmor options each time.

## Built-in profiles

Raind provides these built-in profiles:

| Profile | Capabilities | seccomp | AppArmor | Intended use |
|---|---:|---|---|---|
| `default` | default Raind capability set | enabled | `raind-default` | Normal container execution. |
| `dev` | same as `default` | enabled | `raind-default` | Development-friendly baseline. |
| `deploy` | `default` minus `CAP_NET_RAW` and `CAP_MKNOD` | enabled | `raind-default` | Safer application deployment baseline. |
| `restricted` | empty base capability set | enabled | `raind-default` | Stronger least-privilege baseline. |
| `privileged` | all known capabilities | disabled | disabled | Highly trusted workloads that need broad host privileges. |
| `unconfined` | same capabilities as `default` | disabled | disabled | Keep default capabilities but disable seccomp and AppArmor confinement. |

If no profile is specified, Raind resolves to `default`.

## List profiles

Show built-in and registered custom profiles:

```sh
raind security profile ls
```

Alias:

```sh
raind security profile list
```

The output includes:

| Column | Meaning |
|---|---|
| `NAME` | Profile name. |
| `TYPE` | `built-in` or `custom`. |
| `CAPABILITIES` | Number of base capabilities after the profile is resolved. |
| `SECCOMP` | Whether a seccomp profile is applied. |
| `APPARMOR` | AppArmor profile name, or `-` when disabled. |

Example:

```text
NAME          TYPE      CAPABILITIES  SECCOMP   APPARMOR
default       built-in  14 caps       enabled   raind-default
dev           built-in  14 caps       enabled   raind-default
deploy        built-in  12 caps       enabled   raind-default
restricted    built-in  0 caps        enabled   raind-default
privileged    built-in  41 caps       disabled  -
unconfined    built-in  14 caps       disabled  -
custom-dev    custom    14 caps       enabled   raind-default
```

## Show profile details

Show a resolved profile as YAML:

```sh
raind security profile show <name>
```

Example:

```sh
raind security profile show deploy
```

For a custom profile, the output contains the final resolved capability list and inherited seccomp/AppArmor settings, plus the custom delta fields:

```yaml
name: custom-dev
type: custom
extends: dev
addCap:
  - CAP_SYS_PTRACE
dropCap:
  - CAP_NET_RAW
capabilities:
  base:
    - CAP_CHOWN
    - CAP_DAC_OVERRIDE
    # ...
seccomp:
  defaultAction: SCMP_ACT_ALLOW
  # ...
apparmorProfile: raind-default
```

## Apply a profile to a container

Use `--security-profile` with `container create` or `container run`.

```sh
raind container run --security-profile deploy nginx:latest
```

```sh
raind container create --name web --security-profile restricted nginx:latest
raind container start web
```

You can still use `--cap-add` and `--cap-drop` on the container command. The selected security profile provides the base capability set, and the per-container capability flags are applied on top of that container spec.

```sh
raind container run \
  --security-profile deploy \
  --cap-add CAP_NET_BIND_SERVICE \
  --cap-drop CAP_NET_RAW \
  nginx:latest
```

## Create a custom profile

A custom profile extends an existing built-in or custom profile, then adjusts the inherited capability set with `add-cap` and `drop-cap`.

Custom profiles currently customize capabilities only. seccomp and AppArmor are inherited from the parent profile.

Create a YAML file:

```yaml
apiVersion: raind.io/v1
kind: SecurityProfile
metadata:
  name: custom-dev
spec:
  extends: dev
  add-cap:
    - CAP_SYS_PTRACE
  drop-cap:
    - CAP_NET_RAW
```

Register it:

```sh
raind security profile register -f custom-dev.yaml
```

Alias for the file flag:

```sh
raind security profile register --file custom-dev.yaml
```

Confirm it:

```sh
raind security profile ls
raind security profile show custom-dev
```

Use it:

```sh
raind container run --security-profile custom-dev alpine:latest sh
```

## Custom profile manifest fields

Preferred manifest shape:

| Field | Required | Description |
|---|---:|---|
| `apiVersion` | no | Recommended value: `raind.io/v1`. |
| `kind` | no | Recommended value: `SecurityProfile`. |
| `metadata.name` | yes | Custom profile name. |
| `spec.extends` | yes | Parent profile name. Can be built-in or another custom profile. |
| `spec.add-cap` | no | Capabilities to add to the inherited base set. |
| `spec.drop-cap` | no | Capabilities to remove from the inherited base set. |

A compatibility shape is also accepted:

```yaml
name: custom-dev
extends: dev
add-cap:
  - CAP_SYS_PTRACE
drop-cap:
  - CAP_NET_RAW
```

## Name and validation rules

Profile names must match this format:

```text
^[a-z0-9][a-z0-9_.-]{0,62}$
```

That means:

- Use lower-case letters, digits, `_`, `.`, and `-`.
- Start with a lower-case letter or digit.
- Keep the name to 63 characters or fewer.
- Do not reuse a built-in profile name.

Capability names must:

- Start with `CAP_`.
- Use only upper-case letters, digits, and `_` after `CAP_`.

The profile must also satisfy these rules:

- `extends` is required.
- A profile cannot extend itself.
- The parent profile must already exist.
- Cyclic custom-profile inheritance is rejected.
- Registering an already existing custom profile name is rejected.

If the same capability appears in both `add-cap` and `drop-cap`, `drop-cap` wins.

## Delete a custom profile

Delete a custom profile:

```sh
raind security profile delete <name>
```

Alias:

```sh
raind security profile rm <name>
```

Built-in profiles cannot be deleted.

## Storage location

Custom profiles are stored by the Condenser side under:

```text
/etc/raind/security-profiles
```

Set `RAIND_SECURITY_PROFILE_DIR` in the Condenser process environment to override the directory:

```sh
export RAIND_SECURITY_PROFILE_DIR=/path/to/security-profiles
```

Each registered profile is stored as `<name>.yaml` in that directory.

## Operational notes

- `raind security profile register` reads a local YAML file and sends it to the Raind daemon.
- The daemon validates and stores the manifest, then resolves it against the parent profile.
- `raind security profile show` prints the resolved profile, not only the original manifest.
- AppArmor profiles must already be loaded on the host. Raind applies the configured AppArmor profile name when AppArmor is enabled.
- The current seccomp implementation applies deny rules from the resolved seccomp profile. Built-in confined profiles deny selected sensitive syscalls and otherwise allow.
