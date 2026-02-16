# WebUI (Vue + Vite)

Raind WebUI はコンテナとして動作し、UDS gateway 経由で Condenser に接続します。

## 前提
```bash
sudo make enable-service
sudo make enable-ui-gateway-service
```

## WebUIのビルドと起動
```bash
cd runtime_stack/raind-webui
raind image build -f . -t raind-webui:latest
raind resource apply -f deploy/manifest.yaml
```

`http://<host>:18080` にアクセスします。

## 通信経路
1. Browser が `raind-webui` の `/api/*` にリクエスト。
2. `raind-webui` backend が `/run/raind/ui.sock` へ転送。
3. `raind-ui-gateway` が mTLS で Condenser `https://127.0.0.1:7755` に転送。

## 注意点
- Browser から Condenser へ直接接続しない。
- UDS用に `/run/raind` のみをマウントする。
- `read_only`, `tmpfs` などで WebUI コンテナをハードニングする。
