# Image Workflows

Raind image commands manage local image state used by containers and resources.

```sh
raind image pull <image:tag>
raind image pull --os linux --arch amd64 <image:tag>
raind image build -t <repo/name:tag> <context-path>
raind image ls
raind image rm <image:tag>
```

For Dockerfile build details, see [Dockerfile build](dockerfile-build.md).

Rootless containers may create shifted image layer caches. These caches are removed when the source image/layer is removed.
