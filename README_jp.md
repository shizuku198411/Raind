# Raind - Zero Trust Oriented Container Runtime
<p>
  <img src="./docs/assets/raind_icon.png" alt="Project Icon" width="190">
</p>

![version](https://img.shields.io/badge/version-v0.2.0-blue) ![PoC](https://img.shields.io/badge/PoC-00ac97)

Linux 向け Zero Trust 指向のコンテナランタイムです。  
Raind はオーケストレーション層やアプリ層だけでなく、ランタイム層でコンテナネットワークの制御と可視化を行うことに重点を置いています。

## コンセプト

Raind が提供する価値:

- ランタイム層でのネットワークポリシー制御（East-West / North-South）
- ランタイム層での通信可視化（Traffic/DNS/Audit ログ）
- コンテナライフサイクルとグループワークロードの統合運用
- WebUI 向けに UDS Gateway パターンを使った localhost 制限のコントロールプレーン

## 含まれるコンポーネント

Raind は `runtime_stack/` 配下の以下コンポーネントで構成されます。

- `raind-cli`: ユーザー向け CLI
- `condenser`: 高レベルランタイムデーモン（API・状態管理・ポリシー・リソース制御）
- `droplet`: 低レベル OCI ランタイム実行層
- `raind-ui-gateway`: Condenser への UDS Gateway（上流は mTLS）
- `raind-webui`: Vue + Vite ベースの Web UI

## 主な機能

- コンテナライフサイクル: create/start/stop/delete, attach, exec, logs, stats
- イメージ管理: pull/build/list/remove
- オーケストレーション:
  - Bottle（Docker Compose 相当の複数コンテナ一括運用）
  - ReplicaSet / Pod / Service（Desired 状態管理と selector ベースのサービスルーティング）
- リソース管理: ReplicaSet / Pod / Service の apply/delete/list/show
- Bottle 管理: 複数コンテナのグループ運用
- OCI 準拠の低レベルランタイムセキュリティ（Droplet）:
  - Namespace / cgroup による隔離とリソース制御
  - Capability 制御
  - Seccomp による syscall フィルタ
  - AppArmor プロファイル連携
  - OCI lifecycle hooks
- ポリシー管理:
  - `RAIND-EW`（Inter Connect）
  - `RAIND-NS-OBS`（External Observe）
  - `RAIND-NS-ENF`（External Enforce）
  - commit/revert ワークフロー
- セキュリティ重視のログ:
  - Audit ログ（`/var/log/raind/raind_audit.jsonl`）
  - Netflow ログ（`/var/log/raind/raind_netflow.jsonl`）
  - DNS ログ（`/var/log/raind/raind_dns.jsonl`）
- WebUI ページ:
  - Dashboard / Container / Resource / Bottle / Image / Policy / Audit Log / Network Log
  - フィルター・ページング・関連表示・各種オーバーレイ（操作/詳細/ログ）
  - WebSocket による attach/exec ターミナル UX

## アーキテクチャ

```text
raind-cli
  -> Condenser API (https://127.0.0.1:7755, mTLS)
    -> Droplet (OCI runtime execution)

Browser
  -> raind-webui (HTTPS)
    -> /run/raind/ui.sock (UDS)
      -> raind-ui-gateway
        -> Condenser API (https://127.0.0.1:7755, mTLS)
          -> Droplet (OCI runtime execution)
```

## クイックスタート

### 1. ビルドとインストール

```bash
git clone https://github.com/shizuku198411/Raind.git
cd Raind
make bootstrap
make build
sudo make install
sudo make enable-service
sudo make enable-ui-gateway-service
```

または:

```bash
sudo make all
```

### 2. 動作確認

```bash
raind container run -p 9988:80 nginx:latest
raind container ls
```

### 3. WebUI 起動

`runtime_stack/raind-webui` をビルドし、manifest でデプロイします。

```bash
cd runtime_stack/raind-webui
raind image build -f . -t raind-webui:latest
raind resource apply -f deploy/manifest.yaml
```

## WebUI
### Dashboard
![dashboard](./docs/assets/raind_webui_dashboard.png)

### Container Page
![container](./docs/assets/raind_webui_container.png)

![container_attach](./docs/assets/raind_webui_container_attach.png)

### Resource Relations
![resource](./docs/assets/raind_webui_resource.png)

### Policy Page
![policy](./docs/assets/raind_webui_policy.png)

### Audit / Network Log Pages
![audit](./docs/assets/raind_webui_audit.png)

![network](./docs/assets/raind_webui_network.png)
## セキュリティモデル（要約）

- Condenser の制御 API は localhost/mTLS 前提です。
- WebUI はコンテナから Condenser へ直接アクセスしません。
- `raind-ui-gateway` が WebUI バックエンド向けに制御された UDS を提供します。
- ポリシーはランタイムのネットワーク経路で適用されます。
- ランタイム層ログにより操作と通信の追跡性を確保します。

## ドキュメント

- Install: [EN](docs/en/install.md) / [JP](docs/jp/install.md)
- WebUI: [EN](docs/en/webui.md) / [JP](docs/jp/webui.md)
- UDS Gateway: [EN](docs/en/webui_gateway.md) / [JP](docs/jp/webui_gateway.md)
- Command list: [EN](docs/en/command_list.md) / [JP](docs/jp/command_list.md)
