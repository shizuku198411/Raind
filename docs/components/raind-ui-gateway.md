# Raind UI Gateway

`raind-ui-gateway` is a host-side proxy that listens on a Unix domain socket and forwards requests to Condenser (`https://127.0.0.1:7755`) using mTLS.

## Build
```bash
./scripts/build.sh
```

## Run
```bash
./bin/raind-ui-gateway
```

## Version
```bash
./bin/raind-ui-gateway -version
```
