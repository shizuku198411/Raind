# Support

Raind is currently an experimental developer-preview project. Support is best-effort and primarily handled through GitHub.

## Where to ask questions

Use GitHub issues for:

- reproducible bugs
- documentation problems
- feature requests
- installation problems
- runtime behavior that appears incorrect

Before opening an issue, please check the existing issues and the documentation under `docs/`.

## Security vulnerabilities

Do not report security vulnerabilities in public issues.

Use GitHub Private Vulnerability Reporting from the repository's **Security** tab. See [`SECURITY.md`](SECURITY.md).

## Helpful information for bug reports

Please include as much of the following as possible:

```sh
raind --version
uname -a
go version
raind image ls
raind container ls
raind network ls
```

If installed with the systemd script:

```sh
sudo systemctl status raind-daemon.service --no-pager
sudo journalctl -u raind-daemon.service --no-pager -n 200
sudo tail -200 /etc/raind/log/droplet_audit.log 2>/dev/null || true
```

If installed as a snap:

```sh
snap list raind
snap services raind
sudo snap logs raind.condenser -n 200
```

For container runtime issues, please include:

- the image name and tag
- the exact `raind` command used
- whether rootless mode was used
- relevant container logs
- relevant host networking or cgroup errors
- whether the issue reproduces in Workshop

## Project status

Raind is under active development. Some features may be incomplete, experimental, or subject to change. Production support is not currently provided.
