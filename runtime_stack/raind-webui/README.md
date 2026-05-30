# Raind WebUI (Vue + Vite)

Containerized WebUI for Raind.

## Architecture
- Browser -> `raind-webui` HTTP server
- `raind-webui` backend -> UDS `/run/raind/ui.sock`
- `raind-ui-gateway` -> Condenser (`https://127.0.0.1:7755`, mTLS)

## Local Dev
```bash
npm install
# gateway
./scripts/run.sh gateway

# web-ui
./scripts/run.sh vite
```

## Build Container
```bash
docker build -t raind-webui .
```

## Deploy with Manifest (ReplicaSet + Service)
```bash
raind image build -f . -t raind-webui:latest
raind resource apply -f deploy/manifest.yaml
```

Before applying, ensure `raind-ui-gateway` is active on host.
