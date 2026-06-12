# UDS Gateway 経由の WebUI

Raind では Condenser 管理 API を `127.0.0.1:7755` + mTLS で待ち受けています。
コンテナとして起動する WebUI から利用する場合は `raind-ui-gateway` を利用します。

- Condenser: `https://127.0.0.1:7755`（ホストローカル + mTLS）
- Gateway: `/run/raind/ui.sock`（ホスト UDS）
- WebUIコンテナ: `/run/raind/ui.sock` をマウントして操作を転送

## ビルド & インストール
```bash
workshop run raind-dev -- build
sudo ./scripts/build.sh install
sudo ./scripts/build.sh enable-service
sudo ./scripts/build.sh enable-ui-gateway-service
```

## WebUI バックエンド(Node.js)からのアクセス例
ブラウザから直接 UDS は扱えないため、Vue+Vite 側のバックエンド経由で呼び出します。

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

## 現在の Gateway ポリシー
- 許可パス: `/v1/*`
- 拒否: WebSocket attach系（`/v1/containers/*/attach`, `/v1/containers/*/exec/attach`）
- 許可メソッド: `GET`, `POST`, `DELETE`

Vue + Vite で実行可能なWebUIコンテナ例は [WebUI](./webui.md) を参照してください。

## 環境変数
- `RAIND_UI_SOCKET_PATH`（既定: `/run/raind/ui.sock`）
- `RAIND_UI_SOCKET_MODE`（既定: `0660`）
- `RAIND_UI_CONDENSER_URL`（既定: `https://127.0.0.1:7755`）
- `RAIND_UI_CA_CERT`（既定: `/etc/raind/cert/raind.crt`）
- `RAIND_UI_CLIENT_CERT`（既定: `/etc/raind/cert/raindClient.crt`）
- `RAIND_UI_CLIENT_KEY`（既定: `/etc/raind/cert/raindClient.key`）
