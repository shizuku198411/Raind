# Condenser
<p>
  <img src="assets/condenser_icon.png" alt="Project Icon" width="190">
</p>
Condenser は Raind コンテナランタイムスタックの構成要素のひとつであり、高レベルコンテナランタイムとして動作します。  
コンテナのライフサイクル管理、イメージ管理、外部制御のための REST API 提供を担当するとともに、 
低レベルランタイムである Droplet を呼び出してコンテナ操作をオーケストレーションします。  

## ランタイムスタック構成
Raind のコンテナランタイムスタックは 3 層で構成されます。

- Raind CLI – ランタイムスタック全体を操作するユーザインターフェース
- Condenser – コンテナライフサイクルとイメージ管理を担う高レベルランタイム (本リポジトリ)
- Droplet – コンテナの起動/削除などを担う低レベルランタイム  
  (repository: https://github.com/pyxgun/Droplet)

Condenser は Raind スタックのコントロールプレーンとして機能します。  
高レベル API リクエストを OCI 仕様の生成やコンテナ状態管理に変換し、Droplet に実行を委譲します。

### ランタイムスタックインストール
Raindのインストールは、以下のリポジトリから実施することを推奨します。  
[Raind - Zero Trust Oriented Container Runtime](https://github.com/shizuku198411/Raind)

## 機能
Condenser が現在サポートしている機能:

- コンテナライフサイクル管理
  - 作成/起動/停止/削除/接続/exec/log
  - Droplet と連携した低レベル実行
  - ランタイムのフックによる状態更新

- イメージ管理
  - Docker Hub からのイメージ取得
  - イメージレイヤと root filesystem の管理

- Pod オーケストレーション (Kubernetes 互換のセマンティクス)
  - Pod 作成/起動/停止/削除、一覧/詳細取得
  - 複数コンテナで Network/UTS/IPC 名前空間を共有
  - Infra (pause) コンテナにより名前空間を安定化

- ReplicaSet オーケストレーション (Kubernetes 互換のセマンティクス)
  - ReplicaSet 作成/スケール/削除、一覧/詳細取得
  - Controller による desired replicas を維持し、必要に応じて Pod を再作成

- Service (L4 ロードバランシング)
  - selectorによる Pod の振り分け
  - iptables DNAT による Pod infra IP への分配
  - TCP/UDP ポートをサポート

- Bottle オーケストレーション (Compose 風のセマンティクス)
  - 複数コンテナを 1 単位として管理
  - 各コンテナは独立した名前空間で動作
  - Bottle API で一括操作

- REST API
  - コンテナ/Pod/Bottle/イメージを制御する HTTP API
  - Raind CLI または外部ツールからの利用を想定

## ビルド
要件:

- 名前空間 & cgroup をサポートする Linux カーネル
- Go (1.25 以上)
- root 権限 (または適切な capability)
- `swag` (Swagger generator) が `PATH` にあること

```bash
git clone https://github.com/your-org/condenser.git
cd condenser
./scripts/build.sh
```

## 使い方

Condenser は常駐サービスとして稼働し、REST API 経由で利用する想定です (Raind CLI からの利用が主)。

### Condenser の起動
```bash
sudo ./bin/condenser
```

デフォルトでは HTTP サーバが起動し、コンテナ/イメージ操作の API リクエストを待ち受けます。

## 一般的なワークフロー
Raind スタックでのコンテナ起動の流れ:

- クライアント (Raind CLI または外部ツール) が Condenser に API リクエストを送信
- Condenser が必要なイメージを Docker Hub から取得 (未取得の場合)
- Condenser が OCI 互換の `config.json` を生成し、バンドルを準備
- Condenser が Droplet を呼び出してコンテナを作成/起動
- Condenser が状態を追跡し API で公開

## Pod/ReplicaSet/Service

### Pod
- Pod は Network/UTS/IPC 名前空間を共有する論理的なコンテナグループです。
- メンバーの名前空間を安定させるために infra コンテナが作成されます。

### ReplicaSet
- ReplicaSet はtemplateとselectorに基づき、Pod 数を維持します。
- ステータス:
  - `desired`: 目標 replicas 数
  - `current`: マッチした Pod 数
  - `ready`: `runningContainers == desiredContainers` の Pod 数

### Service
- Pod のラベルセレクタを使う L4 ロードバランサです。
- iptables DNAT を用いてサービスチェーン (`RAIND-SVC-*`) を構成します。

### Apply / Delete (YAML)
Condenser は kubectl 互換の YAML マニフェストをサポートします。1 つの YAML に複数リソースを含められます。

Apply:
```bash
curl -X POST https://localhost:7755/v1/resource/apply \
  -H "Content-Type: text/plain" \
  --data-binary @/path/to/manifest.yaml
```

Delete:
```bash
curl -X POST https://localhost:7755/v1/resource/delete \
  -H "Content-Type: text/plain" \
  --data-binary @/path/to/manifest.yaml
```

対応する kind:
- `Pod`
- `ReplicaSet`
- `Service`

マニフェスト例:
```yaml
apiVersion: apps/v1
kind: ReplicaSet
metadata:
  name: test-rs
  namespace: default
spec:
  replicas: 2
  selector:
    matchLabels:
      app: demo
  template:
    metadata:
      name: test-pod
      labels:
        app: demo
    spec:
      containers:
      - name: nginx
        image: nginx:latest
      - name: ubuntu
        image: ubuntu:latest
        tty: true
---
apiVersion: v1
kind: Service
metadata:
  name: demo-svc
  namespace: default
spec:
  selector:
    app: demo
  ports:
  - port: 11240
    targetPort: 80
    protocol: TCP
```

## Pod vs Bottle

- **Pod**: Network/UTS/IPC を共有すべき密結合コンポーネント向け (sidecar, helper, 同一 IP/hostname)
- **Bottle**: 互いに独立したままグルーピングしたい疎結合サービス向け

## Status
Condenser および Raind コンテナランタイムスタックは現在アクティブに開発中です。  
API、ストレージ形式、挙動は予告なく変更される可能性があります。
