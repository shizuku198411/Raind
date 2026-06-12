# Dockerfile Build Support

`raind image build` can build local image contexts from a `Dripfile` or a Docker-compatible `Dockerfile`.

When no build file is specified, Raind searches in this order:

1. `Dripfile`
2. `Dockerfile`

## Commands

Build with the default file lookup:

```sh
raind image build -t local/app:latest .
```

Build with an explicit Dockerfile path inside the context:

```sh
raind image build -t local/app:latest -f Dockerfile .
raind image build -t local/app:latest -f build/Dockerfile .
```

Build with an explicit Dripfile path:

```sh
raind image build -t local/app:latest --dripfile Dripfile .
```

The build context is uploaded to condenser as a tar archive. Condenser currently applies these limits:

| Limit | Value |
| --- | ---: |
| Build context tar stream | 1 GiB |
| Single file in context | 512 MiB |
| Tar entries | 100,000 |

## Supported Instructions

| Instruction | Status | Notes |
| --- | --- | --- |
| `FROM` | Supported | Supports normal images, `scratch`, `FROM ... AS <stage>`, and `--platform` parsing. Platform is accepted but not used for build-time selection yet. |
| `RUN` | Supported | Shell form and JSON exec form are supported. Each `RUN` is executed before later filesystem instructions, matching Dockerfile ordering more closely. |
| `COPY` | Supported | Supports quoted paths, JSON form, multiple sources, `--from`, and `--chmod`. |
| `ADD` | Partially supported | Behaves like local `COPY`. Remote URL fetching and archive auto-extraction are not implemented yet. |
| `WORKDIR` | Supported | Relative paths are resolved from the current workdir. |
| `ENV` | Supported | Supports `KEY=value` and `KEY value` forms, including quoted values. |
| `CMD` | Supported | Shell form and JSON exec form are supported. |
| `ENTRYPOINT` | Supported | Shell form and JSON exec form are supported. |
| `USER` | Supported | Stored in the built image config. Build-time `RUN` user switching is not implemented yet. |
| `SHELL` | Parsed | JSON form is parsed and stored during build planning, but build-time `RUN` currently still uses Raind's shell execution path. |
| `ARG` | Parsed | Accepted for Dockerfile compatibility. Values are not expanded yet. |
| `LABEL` | Parsed | Accepted but not persisted in image metadata yet. |
| `EXPOSE` | Parsed | Accepted but not persisted in image metadata yet. |
| `VOLUME` | Parsed | Accepted but not persisted in image metadata yet. |
| `STOPSIGNAL` | Parsed | Accepted but not persisted in image metadata yet. |
| `HEALTHCHECK` | Parsed | Accepted but not executed or persisted yet. |
| `ONBUILD` | Parsed | Accepted but not executed or persisted yet. |
| `MAINTAINER` | Parsed | Accepted for legacy Dockerfile compatibility. |

Unsupported instructions fail the build with `unsupported instruction`.

## Supported `FROM` Forms

Basic base image:

```Dockerfile
FROM alpine:latest
```

Named stage:

```Dockerfile
FROM golang:1.22 AS builder
```

Platform option is accepted:

```Dockerfile
FROM --platform=linux/amd64 alpine:latest
```

Scratch final image:

```Dockerfile
FROM scratch
```

## Supported `COPY` and `ADD` Forms

Copy one file:

```Dockerfile
COPY app /usr/local/bin/app
```

Copy a directory:

```Dockerfile
COPY public/ /var/www/public/
```

Copy multiple sources into a directory:

```Dockerfile
COPY index.html styles.css /var/www/html/
```

Quoted paths:

```Dockerfile
COPY "config files/app.conf" /etc/app/app.conf
```

JSON form:

```Dockerfile
COPY ["bin/app", "/usr/local/bin/app"]
```

Copy from a previous stage:

```Dockerfile
COPY --from=builder /out/app /usr/local/bin/app
```

Copy from a stage by index:

```Dockerfile
COPY --from=0 /out/app /usr/local/bin/app
```

Copy from an external image:

```Dockerfile
COPY --from=nginx:latest /usr/share/nginx/html /site
```

Set mode while copying:

```Dockerfile
COPY --chmod=755 bin/app /usr/local/bin/app
```

`--chown` and `--link` are accepted for compatibility, but ownership changes and link-mode copy semantics are not implemented yet.

## Examples

### Basic Static Site

```Dockerfile
FROM nginx:latest
COPY index.html /usr/share/nginx/html/index.html
COPY assets/ /usr/share/nginx/html/assets/
EXPOSE 80
```

Build:

```sh
raind image build -t local/site:latest .
```

### Shell Form `RUN`, `ENV`, and `CMD`

```Dockerfile
FROM alpine:latest
ENV APP_ENV=dev APP_NAME="raind sample"
WORKDIR /app
COPY . .
RUN mkdir -p /app/data && echo "$APP_NAME" > /app/data/name.txt
CMD ["cat", "/app/data/name.txt"]
```

Build:

```sh
raind image build -t local/sample:latest .
```

### Multi-Stage Build

```Dockerfile
FROM golang:1.22 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /out/app ./cmd/app

FROM alpine:latest
COPY --from=builder --chmod=755 /out/app /usr/local/bin/app
ENTRYPOINT ["/usr/local/bin/app"]
```

Build:

```sh
raind image build -t local/go-app:latest -f Dockerfile .
```

### Scratch Final Image

```Dockerfile
FROM alpine:latest AS rootfs
RUN mkdir -p /out && echo hello > /out/message.txt

FROM scratch
COPY --from=rootfs /out/message.txt /message.txt
CMD ["/message.txt"]
```

This is useful for very small filesystem-only images. The final image does not include a shell unless you copy one into it.

### Metadata-Oriented Dockerfile

```Dockerfile
FROM alpine:latest
LABEL org.opencontainers.image.title="raind-demo"
ARG BUILD_VERSION=dev
USER 1000:1000
WORKDIR /home/app
COPY --chown=1000:1000 . .
STOPSIGNAL SIGTERM
HEALTHCHECK CMD ["true"]
CMD ["sh", "-c", "echo ready"]
```

Raind accepts this Dockerfile. `USER` is stored in the image config. `LABEL`, `ARG`, `STOPSIGNAL`, and `HEALTHCHECK` are parsed for compatibility, but metadata persistence and healthcheck execution are not implemented yet.

### Custom Build File Path

Directory layout:

```text
app/
  docker/
    production.Dockerfile
  src/
    main.sh
```

Dockerfile:

```Dockerfile
FROM alpine:latest
COPY src/main.sh /usr/local/bin/main
RUN chmod +x /usr/local/bin/main
ENTRYPOINT ["/usr/local/bin/main"]
```

Build:

```sh
raind image build -t local/app:prod -f docker/production.Dockerfile ./app
```

## Current Compatibility Notes

Raind's builder is intentionally smaller than Docker BuildKit. These features are not implemented yet:

- `.dockerignore`
- build args expansion
- variable expansion in instruction arguments
- Docker cache semantics
- remote URL `ADD`
- automatic archive extraction for `ADD`
- persisted labels, exposed ports, volumes, stopsignal, and healthcheck metadata
- healthcheck execution
- build secrets and SSH mounts
- `RUN --mount=...`
- full `SHELL` override behavior for `RUN`
