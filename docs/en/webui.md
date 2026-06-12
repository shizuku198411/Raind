# WebUI (Vue + Vite)

Raind WebUI runs as a container and communicates through UDS gateway.

## Prerequisite
```bash
sudo ./scripts/build.sh enable-service
sudo ./scripts/build.sh enable-ui-gateway-service
```

## Build and Run WebUI
```bash
cd webui
raind image build -f . -t raind-webui:latest
raind resource apply -f deploy/manifest.yaml
```

Open `http://<host>:18080`.

## Data Path
1. Browser requests `/api/*` on `raind-webui`.
2. `raind-webui` backend forwards to `/run/raind/ui.sock`.
3. `raind-ui-gateway` forwards to Condenser `https://127.0.0.1:7755` with mTLS.

## Notes
- Browser never connects to Condenser directly.
- Mount only `/run/raind` for UDS access.
- Keep WebUI container hardened (`read_only`, `tmpfs`).
