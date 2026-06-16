# Rootless Modes Reference

Rootless mode is configured through `raind container create/run` flags.

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--rootless` | bool | `false` | Enable rootless with the default mode. |
| `--rootless-mode` | string | `shifted-root` | Select `shifted-root` or `login-root`. Setting this flag enables rootless. |

## Supported modes

| Mode | UID map with defaults | GID map with defaults | Notes |
|---|---|---|---|
| `shifted-root` | `0 100000 65536` | `0 100000 65536` | Default when `--rootless` is used. |
| `login-root` | `0 <login-uid> 1`, `1 100000 65535` | `0 <login-gid> 1`, `1 100000 65535` | Maps container root to the login user. |

## Environment variables

| Variable | Default | Description |
|---|---:|---|
| `RAIND_ROOTLESS_UID_BASE` | `100000` | Host UID base for subordinate rootless IDs. |
| `RAIND_ROOTLESS_GID_BASE` | `100000` | Host GID base for subordinate rootless IDs. |
| `RAIND_ROOTLESS_ID_MAP_SIZE` | `65536` | Number of container IDs covered by the map. |

## CLI UID/GID detection for `login-root`

For `login-root`, the CLI sends the login user's host UID/GID with the create/run request.

Detection order:

1. `SUDO_UID` / `SUDO_GID`, when set and valid
2. the CLI process UID/GID

## Current support matrix

| Workload type | Rootless support |
|---|---:|
| standalone container | yes |
| Pod app container | no |
| Bottle container | depends on current Bottle-to-container flag support; rootless is not the primary Bottle path yet |
| Kubernetes-style resources | no rootless Pod support yet |
