# Security Policy

Raind is an experimental local container runtime and lightweight workload orchestrator for Linux. It manages Linux namespaces, mounts, cgroups, container networking, runtime state, security profiles, and container process execution.

Because Raind operates in a security-sensitive area, please report suspected vulnerabilities privately and responsibly.

## Project status

Raind is currently under active development and should be treated as **experimental / developer preview** software.

Raind is intended for local development, testing, and research workloads. It is not yet recommended for production workloads or multi-tenant environments.

The security model, rootless mode, networking model, and workload isolation behavior are still being improved. Security reports are welcome, especially when they affect container isolation, host access, runtime state integrity, credential handling, or network policy enforcement.

## Supported versions

Security fixes are currently applied to the latest released version and the `main` branch.

| Version | Supported |
|---|---:|
| Latest release | Yes |
| `main` branch | Yes |
| Older releases | Best effort |

If a vulnerability affects an older release, please still report it. The maintainer will decide whether to backport a fix based on severity, exploitability, and project status.

## Reporting a vulnerability

Please do **not** open a public GitHub issue, public pull request, public discussion, or social media post for a suspected vulnerability.

Raind uses **GitHub Private Vulnerability Reporting** for security reports.

To report a vulnerability privately:

1. Open the Raind repository on GitHub.
2. Go to the **Security** tab.
3. Select **Report a vulnerability**.
4. Submit the report through GitHub's private vulnerability reporting flow.

This creates a private security advisory thread visible to the maintainer and the reporter. The maintainer can use that thread to discuss impact, reproduction steps, patches, disclosure timing, and possible credit.

If GitHub Private Vulnerability Reporting is temporarily unavailable, please contact the maintainer through the GitHub repository or maintainer profile and ask to arrange a private reporting channel before sharing technical details.

When reporting, please include as much of the following as possible:

- Raind version, release tag, or commit SHA
- Host distribution, kernel version, architecture, and cgroup mode
- Installation method, such as source build, release artifact, local package, or snap
- Whether the issue affects rootful containers, rootless containers, or both
- Whether the issue affects standalone containers, bottles, or resource-managed Pods
- Exact commands, manifests, images, Dockerfiles, or network policies needed to reproduce the issue
- Relevant logs from `/var/log/raind`, `/etc/raind/log`, and container bundle logs, if available
- Expected behavior and actual behavior
- Whether the issue requires local user access, daemon/API access, a crafted image, a crafted manifest, a malicious container workload, or network access
- Any known impact, such as host filesystem access, container escape, policy bypass, credential exposure, privilege escalation, denial of service, or persistent host network/cgroup modification

Please avoid sharing exploit details publicly until a fix, mitigation, or advisory has been released.

## What to report privately

Examples of issues that should be reported privately include:

- Container escape or host privilege escalation
- Bypass of namespace, mount, cgroup, seccomp, AppArmor, or capability restrictions
- Incorrect UID/GID mapping in rootless mode that gives a container unintended host access
- Incorrect bind mount, hostPath, or rootfs handling that exposes unintended host paths
- Policy bypass in east-west policy, namespace egress policy, service routing, ingress routing, or published-port handling
- Incorrect iptables/nftables rule generation that exposes traffic unexpectedly
- Unauthorized access to the Condenser API, hook API, or PKI signing API
- Forged or incorrectly accepted client identities, SPIFFE-style URI identities, or TLS client certificates
- Leakage of private keys, client certificates, generated CA material, image credentials, or container credentials
- Insecure file permissions under `/etc/raind`, `/var/log/raind`, or container bundle directories
- Unsafe handling of image layers, Dockerfile builds, tar extraction, paths, symlinks, or archives
- Runtime state corruption that can be triggered by an untrusted image, manifest, local user, or API request
- Denial-of-service issues that can crash the daemon, corrupt runtime state, or leave persistent unsafe host networking/cgroup state

Public issues are fine for general bugs, documentation issues, feature requests, build failures, and non-security failures that do not expose sensitive behavior.

## Security model overview

Raind is built as a layered runtime stack:

```text
raind CLI
  -> condenser: high-level runtime API, image/resource/policy/log controller
    -> droplet: low-level OCI-style container runtime
```

### Control plane

Condenser exposes local runtime APIs for containers, images, resources, networking, policy, logs, security profiles, hooks, and PKI signing.

Raind uses TLS client certificates and SPIFFE-style URI identities for internal API access control. The API router checks client certificate identity and scopes for CLI operations. Hook and PKI routes also validate expected SPIFFE identity prefixes.

Security-sensitive control-plane assets are stored under `/etc/raind`, including runtime state, generated certificates, client certificates, container state, and image/layer metadata.

### Runtime state and logs

Raind stores runtime state under paths such as:

```text
/etc/raind
/etc/raind/container
/etc/raind/image
/etc/raind/store
/etc/raind/cert
/etc/raind/cli
/etc/raind/ingress/cert
/var/log/raind
/sys/fs/cgroup/raind
```

These paths are security-sensitive. Incorrect permissions or unintended write access to these directories can affect container execution, image/layer state, runtime credentials, network policy, and audit/log integrity.

### Container isolation

Droplet creates container execution environments using Linux primitives such as:

- namespaces
- cgroups v2
- mount setup and rootfs switching
- bind mounts and image layers
- Linux capabilities
- seccomp deny filters
- AppArmor on-exec profiles
- container lifecycle hooks
- `nsenter`-based exec/attach paths

The default security profile applies a bounded capability set, enables seccomp, and applies the `raind-default` AppArmor profile when AppArmor is available and the profile is loaded on the host.

Security profiles can change the capability, seccomp, and AppArmor behavior of containers. Profiles such as `privileged` or `unconfined` intentionally relax restrictions and should only be used for trusted workloads.

### Rootless containers

Raind supports rootless standalone containers. Rootless mode uses a user namespace and UID/GID mappings.

Current modes include:

- `shifted-root`: maps container IDs into a subordinate host ID range.
- `login-root`: maps container root to the login user's UID/GID and maps the remaining container IDs into a subordinate host ID range.

Rootless mode is designed to reduce the host impact of container root and improve host-friendly bind mount behavior. It is not a complete replacement for a mature production container sandbox, and it currently applies to standalone containers only.

Reports involving incorrect rootless ID mapping, unexpected host ownership, bind mount exposure, or `exec` behavior inside rootless containers are especially valuable.

### Networking and policy

Raind manages local container networking with bridges, veth pairs, container IP assignment, routes, DNS behavior, and iptables/nftables-based traffic handling.

Raind also supports runtime-managed network policy and netflow logging. Security reports should include any case where policy mode, east-west policy, namespace egress policy, service routing, ingress routing, or published-port handling allows traffic that should be denied.

## Known limitations

Raind is still experimental. The following limitations are currently expected and should be considered when evaluating security impact:

- Raind is intended for local development, testing, and research, not production multi-tenant isolation.
- Some Docker and Kubernetes compatibility paths are incomplete or evolving.
- Rootless support currently targets standalone containers; Pod-managed rootless containers require additional design.
- AppArmor enforcement depends on AppArmor being enabled on the host and the configured profile being loaded.
- Security profiles can intentionally relax isolation; `privileged` and `unconfined` should be treated as trusted-workload modes.
- HostPath/bind mount behavior gives containers access to user-selected host paths by design.
- The daemon/runtime requires privileged host operations for rootful containers, networking, cgroup setup, mount setup, and policy management.
- Snap classic confinement is currently required for the full runtime model; strict confinement does not provide the full set of host operations needed by Raind.

A limitation may still be a vulnerability if Raind grants more access than documented, bypasses a selected policy/profile, exposes credentials, or allows an untrusted workload to affect host state outside the requested runtime configuration.

## Recommended deployment posture

Until Raind is more mature, please use it with the following assumptions:

- Use Raind on development or test machines, not production multi-tenant hosts.
- Do not run untrusted images or manifests unless you are specifically testing isolation behavior.
- Prefer the default, `deploy`, or `restricted` security profiles for ordinary workloads.
- Avoid `privileged` and `unconfined` unless the workload is fully trusted.
- Prefer rootless mode for development workloads when possible.
- Keep `/etc/raind`, `/var/log/raind`, generated certificates, and container bundle directories protected from untrusted users.
- Review bind mounts and hostPath volumes carefully before running workloads.
- Review published ports and network policies before exposing services.

## Disclosure process

After receiving a valid security report through GitHub Private Vulnerability Reporting, the maintainer will try to:

1. Acknowledge the report.
2. Reproduce and assess the issue.
3. Discuss impact, affected versions, and possible mitigations privately.
4. Prepare a fix or mitigation.
5. Publish a release and GitHub Security Advisory when appropriate.
6. Credit the reporter if they want to be credited.

Raind is currently maintained as a small project, so response times may vary. Reports with clear reproduction steps and impact analysis are greatly appreciated.

## Security contact

Please use **GitHub Private Vulnerability Reporting** for this repository:

```text
Security -> Report a vulnerability
```

Please do not disclose suspected vulnerabilities publicly until a fix, mitigation, or advisory is available.
