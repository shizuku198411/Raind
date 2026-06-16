# Exec Architecture

Raind supports `raind container exec` for non-TTY and TTY commands.

## Non-TTY exec

For non-TTY commands, Condenser sends an exec request to Droplet. Droplet reads container state, resolves the target process PID, loads the container spec, builds an `nsenter` command, and waits for the command to exit.

High-level flow:

```text
raind container exec <container> <cmd...>
  -> Condenser API
    -> Droplet exec
      -> load state.json
      -> load config.json
      -> resolve entrypoint in container PATH
      -> nsenter target namespaces/root/cwd
      -> run command and wait
```

Exec output is currently handled by the Droplet exec path. TTY streaming uses the exec-shim path below.

## TTY exec and exec-shim

TTY exec starts an `exec-shim` process. The shim owns the pseudo-terminal and a Unix socket used by the API websocket attach path.

High-level flow:

```text
raind container exec --tty <container> /bin/sh
  -> Condenser starts Droplet exec-shim
  -> exec-shim listens on exec_tty.sock
  -> websocket attach connects to the socket
  -> exec-shim runs nsenter command behind a pty
```

## Namespace entry

The nsenter command is built explicitly rather than using `--all`:

```text
nsenter -t <pid> -m -u -i -n -p -C --root --wd=<cwd> -- <command>
```

For rootless containers, user namespace entry and credential switching are added:

```text
nsenter -t <pid> -U --setuid 0 --setgid 0 -m -u -i -n -p -C --root --wd=<cwd> -- <command>
```

Important details:

- `--root` switches to the target process root.
- `--wd=<cwd>` uses the OCI process working directory.
- bare commands are resolved against the container rootfs and container `PATH` before `nsenter` runs.
- rootless exec must enter the user namespace and switch to namespace root.

This keeps rootfull and rootless exec behavior on the same code path.
