# Runtime Layout Reference

Default host-side paths:

```text
/etc/raind/
  container/
    <container-id>/
      config.json
      state.json
      rootfs/
      merged/
      diff/
      work/
      logs/
      cert/
  image/
    layers/
  store/
    container/
      csm.json
      bsm.json
    image/
      ilm.json
    network/
      ipam.json
      npm.json
    resource/
      namespace/
        nsm.json
      pod/
        psm.json
      service/
        ssm.json
      ingress/
        ism.json

/var/log/raind/
  raind_audit.jsonl
  raind_dns.jsonl
  raind_netflow.jsonl
```

Rootless-shifted caches are created near image layer rootfs directories:

```text
rootless-shifted/<mapping-cache-key>/rootfs
rootless-shifted/<mapping-cache-key>/.raind-rootless-shift-complete
```

Image deletion removes associated `rootless-shifted` caches.
