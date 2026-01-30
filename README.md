# Raind - Zero Trust Oriented Container Runtime
<p>
  <img src="assets/raind_icon.png" alt="Project Icon" width="190">
</p>

[version](https://img.shields.io/badge/version-v0.1.1-blue)

**Raind** は **Zero Trust コンテナ** をコンセプトに設計された、実験的なコンテナランタイムです。  
従来の「ネットワークは外部で守る」という前提ではなく、**ランタイム自身が通信を制御・観測するセキュリティ境界になる** ことを目指しています。

本プロジェクトは「実用的な代替ランタイム」を目指すものではなく、

> コンテナランタイム自身がセキュリティ境界になり得るか

という設計思想に基づくPoCです。

## 問題意識
多くのコンテナ環境では、以下が暗黙の前提になっています。

- コンテナ間通信は基本的に自由 (flat network)
- ネットワークセキュリティは上位 (FW / Service Mesh / CNI) に委ねる
- ランタイムは「起動と隔離」までが責務

しかしこの前提では、

- **どのコンテナが、どこへ通信したか** をランタイム単位で把握できない
- 通信ログとコンテナ実体の突合が困難 (SNATによるコンテナアドレスの隠蔽)
- 「許可されている通信」と「偶然通っている通信」の区別が曖昧

という問題が残ります。  
Raindはこれらを **ランタイムレイヤで解決できるか** を検証します。

## Raindの特徴
Raindで最も特徴的な機能は以下です。

- ポリシーの明示的宣言
- 通信の可視化

データベースを利用するWebサーバ(Wordpress等)の構築を例に、Raindの特徴を見てみます。

### 1. ポリシー定義
Raindでは、コンテナ間通信(East-West)はデフォルトで拒否されます。  
そのため、明示的に許可ポリシーを作成します。
```
// wordpress → databaseへのtcp/3306を許可するポリシーの作成
$ raind policy add --type ew -s wordpress -d wp_database -p tcp --dport 3306
policy: 01kg6m673gr5y0cbh62dyeakth created

// 設定反映
$ raind policy commit
This operation will affect the container network.
Are you sure you want to commit? (y/n): y
policy commit success

// ポリシー確認
$ raind policy ls --type ew
FLAG: [*] - Applied, [+] - Apply next commit, [-] - Remove next commit, [ ] - Not applied

POLICY TYPE : East-West
CURRENT MODE: deny_by_default

FLAG  POLICY ID                   SRC CONTAINER  DST CONTAINER  PROTOCOL  DST PORT  ACTION  COMMENT  REASON
[ ]   01kg6m673gr5y0cbh62dyeakth  wordpress      wp_database    tcp       3306      ALLOW            container: wordpress not found
  >> DENY ALL EAST-WEST TRAFFIC <<
```
作成予定のコンテナ名をキーにすることが可能で、該当のコンテナが作成された際に自動でポリシーが適用されます。
これにより、アップタイム時のポリシー適用までのラグを最小限に抑えることが可能です。

### 2. コンテナ作成 & 起動
```
// MySQLコンテナ作成
$ raind container create --name wp_database \
-e MYSQL_ROOT_PASSWORD=wordpress \
-e MYSQL_USER=wordpress -e MYSQL_PASSWORD=wordpress -e MYSQL_DATABASE=wordpress \
mysql:latest

// wordpressコンテナ作成
$ raind container create --name wordpress \
-p 10080:80
-e WORDPRESS_DB_HOST=10.166.0.1:3306
-e WORDPRESS_DB_USER=wordpress -e WORDPRESS_DB_PASSWORD=wordpress -e WORDPRESS_DB_NAME=wordpress \
wordpress:latest

// コンテナ起動
$ raind container start wp_database
$ raind container start wordpress

// コンテナステータス確認
$ raind container ls
CONTAINER ID  IMAGE             COMMAND                  CREATED              STATUS   PORTS                  NAME
01kg6mv5pmpy  mysql:latest      "docker-entrypoint.sh…"  less than a minutes  running                         wp_database
01kg6mka4jfc  wordpress:latest  "docker-entrypoint.sh…"  less than a minutes  running  0.0.0.0:10080->80/tcp  wordpress

// ポリシー適用確認
$ raind policy ls --type ew
FLAG: [*] - Applied, [+] - Apply next commit, [-] - Remove next commit, [ ] - Not applied

POLICY TYPE : East-West
CURRENT MODE: deny_by_default

FLAG  POLICY ID                   SRC CONTAINER  DST CONTAINER  PROTOCOL  DST PORT  ACTION  COMMENT  REASON
[*]   01kg6m673gr5y0cbh62dyeakth  wordpress      wp_database    tcp       3306      ALLOW 
  >> DENY ALL EAST-WEST TRAFFIC <<
```
RaindのコマンドラインはDockerと類似の設計としているため、Dockerユーザにとって馴染みのあるコマンドで作成ができます。

### 3. トラフィックログ確認
```
$ raind logs netflow
  :
2026-01-30 13:40:48     ALLOW   FROM: wordpress => TO: 8.8.8.8 {UDP/53}
2026-01-30 13:40:49     ALLOW   FROM: wordpress => TO: 65.21.231.50 {TCP/443}
2026-01-30 13:40:52     ALLOW   FROM: wordpress => TO: wp_database {TCP/3306}
2026-01-30 13:40:54     ALLOW   FROM: wordpress => TO: wp_database {TCP/3306}
2026-01-30 13:40:58     ALLOW   FROM: wordpress => TO: wp_database {TCP/3306}
```
Raindのトラフィックログは、コンテナのIPに対しコンテナID/コンテナ名がマッピングされます。  
「どのコンテナが」「どのコンテナ/アドレスに」通信を行っているか、直感的に確認することが可能です。

## 設計思想
### 1. Zero Trustは「デフォルトで拒否」から始まる
Raindでは以下を前提にします。

- コンテナは **互いに信頼しない**
- 通信は **明示的なポリシーでのみ許可**
- 暗黙の許可は存在しない

この思想は特に **East-West (コンテナ間通信)** に強く反映しています。

### 2. ランタイム自身を通信の強制ポイントにする
Raindは以下を重視しています。

- 通信制御を「外部コンポーネント」に一任しない
- ランタイム自身が
    - 通信を止められる
    - 通信を観測できる
    - 判定理由をログに残せる

ポイントは、**「通信が必ず通過するポイントをランタイムが握る」** ことです。

### 3. 観測(Observe)と強制(Enforce)を分離する
RaindはNorth-South(外部通信)に対して、

- **Observe** (全許可・ログのみ)
- **Enforce** (ポリシー未定義は拒否)

を切り替え可能にしています。これは、

- いきなり遮断しない
- 実トラフィックを見ながらポリシーを設計する

という実運用を意識した設計思想です。

## ネットワークモデル
### East-West (コンテナ間通信)
- デフォルト: **Deny**
- 方向性を持った明示ポリシーによってのみAllow

### North-South (外部通信)
- デフォルト: **Observeモード**
- Enforceモードへの切り替えが可能

### ポリシーは「コンテナ起点」で定義可能
ポリシーはアドレス起点ではなく、「コンテナ起点」で定義が可能  
例: 送信元コンテナ: Web、宛先コンテナ: DB、プロトコル：TCP/3306、アクション: Allow

## ログ設計
Raindは通信ログを **ランタイムの一次成果** として扱います。
### ログの特徴
- 5-tuple (src/dst IP, src/dst port, protocol)
- コンテナID / コンテナ名 / veth
- East-West / North-Southの区別
- Allow /Deny
- 適用されたポリシーID

ログは単なるデバッグ用途ではなく、

> 「なぜその通信が許可/拒否されたか」を説明できる監査ログ

として設計しています。

## ランタイムとしての機能範囲
Raindは一般的なコンテナランタイムの機能として以下を実装しています。

- Linux Namespaceによる隔離
- cgroup v2によるリソース制限
- Capability / Seccomp / AppArmor
- Docker Hubからのイメージ取得

### OCI準拠のコンテナ起動
コンテナ起動におけるライフサイクルおよび設定ファイルは、[OCI Runtime Spec](https://github.com/opencontainers/runtime-spec/tree/main)に準拠しています。

## 位置づけ (v0.1.x)
Raind v0.1.0は以下を目的としたリリースです。

- Zero Trust を コンテナランタイムの責務として実装可能か の検証
- ネットワーク制御・観測をランタイムに統合する設計の検証
- 実運用ではなく 設計・実装・思想の PoC

### ロードマップ
- 安定性・パフォーマンス最適化
- 高度なL7ポリシー
- eBPF / nftables等への拡張
- コンテナ隔離処理のセキュリティ向上

## ドキュメント
- [Raindセットアップ](./docs/install.md)
- [Raind利用方法](./docs/usage.md)
- [デザインドキュメント](./docs/design.md)

## ステータス
- Status: Experimental / PoC (Proof of Concept)
- Version: v0.1.0
- 対象読者:
    - コンテナランタイム開発者
    - コンテナセキュリティ設計者
    - Zero Trust / ネットワーク制御に関心のあるエンジニア
