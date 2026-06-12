# WebUI via UDS Gateway

Raind keeps Condenser management API bound to `127.0.0.1:7755` with mTLS.
For a containerized WebUI, use `raind-ui-gateway` to expose a Unix domain socket:

- Condenser: `https://127.0.0.1:7755` (host local, mTLS)
- Gateway: `/run/raind/ui.sock` (host UDS)
- WebUI container: mount `/run/raind/ui.sock` and forward UI actions through it

## Build & Install
```bash
workshop run raind-dev -- build
sudo ./scripts/build.sh install
sudo ./scripts/build.sh enable-service
sudo ./scripts/build.sh enable-ui-gateway-service
```

## Socket Access from WebUI Backend (Node.js)
Use a backend route in your Vue+Vite stack (do not call UDS directly from browser).

```js
import http from "node:http";

export async function callRaind(path, method = "GET", body) {
  return new Promise((resolve, reject) => {
    const req = http.request(
      {
        socketPath: "/run/raind/ui.sock",
        path,
        method,
        headers: { "Content-Type": "application/json" },
      },
      (res) => {
        let data = "";
        res.on("data", (c) => (data += c));
        res.on("end", () => resolve({ status: res.statusCode, body: data }));
      }
    );
    req.on("error", reject);
    if (body) req.write(JSON.stringify(body));
    req.end();
  });
}
```

## Current Gateway Policy
- Allowed path: `/v1/*`
- Blocked: websocket attach endpoints (`/v1/containers/*/attach`, `/v1/containers/*/exec/attach`)
- Allowed methods: `GET`, `POST`, `DELETE`

For a ready-to-run Vue + Vite container example, see [WebUI](./webui.md).

## Environment Variables
- `RAIND_UI_SOCKET_PATH` (default: `/run/raind/ui.sock`)
- `RAIND_UI_SOCKET_MODE` (default: `0660`)
- `RAIND_UI_CONDENSER_URL` (default: `https://127.0.0.1:7755`)
- `RAIND_UI_CA_CERT` (default: `/etc/raind/cert/raind.crt`)
- `RAIND_UI_CLIENT_CERT` (default: `/etc/raind/cert/raindClient.crt`)
- `RAIND_UI_CLIENT_KEY` (default: `/etc/raind/cert/raindClient.key`)
