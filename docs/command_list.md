# Raind - Command List
Raindは `sudo` による実行が前提となります。
## Container

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 作成 | container | create | --tty, -t TTY接続 | \<image:tag> |
|||| --name \<container-name> コンテナ名 ||
|||| --publish, -p \<host-port>:\<container-port>  ポートフォワード ||
|||| --volume, -v \<host-dir>:\<container-dir> ホストディレクトリマウント ||
|||| --environment, -e <KEY=VALUE> 環境変数 ||

example: `raind container create -t --name web -v /mnt/web:/var/www/html -p 8080:80 nginx:latest`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 起動 | container | start || \<container-id\|container-name> |

example: `raind container start web`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 接続(アタッチ) | container | attach || \<container-id\|container-name> |

example: `raind container attach web`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 停止 | container | stop || \<container-id\|container-name> |

example: `raind container stop web`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 作成+起動(+接続) | container | run | --tty, -t TTY接続 | \<image:tag> |
|||| --name \<container-name> コンテナ名 ||
|||| --publish, -p \<host-port>:\<container-port>  ポートフォワード ||
|||| --volume, -v \<host-dir>:\<container-dir> ホストディレクトリマウント ||
|||| --environment, -e <KEY=VALUE> 環境変数 ||

example: `raind container run -t --name web -v /mnt/web:/var/www/html -p 8080:80 nginx:latest`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| コマンド実行 | container | exec | --tty, -t TTY接続 | \<container-id\|container-name> \<command[,arg1,arg2,...]>|

example: `raind container exec web /bin/sh -c "echo Hello World! > hello.txt"`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 削除 | container | rm || \<container-id\|container-name> |

example: `raind container rm web`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| ログ確認 | container | logs || \<container-id\|container-name> |

example: `raind container logs web`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 一覧 | container | ls |||

example: `raind container ls`

## Bottle
Bottleは複数のコンテナを1つのグループとして管理します。(docker-compose相当)  
※詳細は [Bottle Usage](bottle.md) を参照

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 作成 | bottle | create | --file, -f \<bottle-file-path> Bottle定義ファイル ||

example: `raind bottle create -f ~/myapp/Dripfile.yaml`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 起動 | bottle | start || \<bottle-id\|bottle-name> |

example: `raind bottle start myapp`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 停止 | bottle | stop || \<bottle-id\|bottle-name> |

example: `raind bottle stop myapp`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 削除 | bottle | rm || \<bottle-id\|bottle-name> |

example: `raind bottle rm myapp`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 一覧 | bottle | ls |||

example: `raind bottle ls`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 詳細情報 | bottle | show || \<bottle-id\|bottle-name> |

example: `raind bottle show myapp`


## Image

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 取得 | image | pull || \<image:tag> |

example: `raind image pull alpine:latest`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 削除 | image | rm || \<image:tag> |

example: `raind image rm alpine:latest`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 一覧 | image | ls |||

example: `raind image ls`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| ビルド | image | build | --file, -f コンテキストディレクトリ (*) ||
|||| --tag, -t \<image:tag> イメージ名/タグ (*) ||

example: `raind image build -f ~/myapp -t myapp:latest`


## Policy
※全てのポリシー変更操作は、`commit`サブコマンドを実行するまでは実際のポリシーに反映されません。

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 作成 | policy | add | --type <ew\|ns-obs\|ns-enf> ポリシータイプ (*) ||
|||| --source, -s \<container-id\|container-name> 送信元コンテナ (*) ||
|||| --destination, -d \<container-id\|container-name\|ip-address> 宛先コンテナ/アドレス (*) ||
|||| --protocol, -p \<icmp\|tcp\|udp> プロトコル ||
|||| --dport \<dest-port> 宛先ポート ||
|||| --comment \<comment> コメント ||

example: `raind policy add --type ew -s web -d db -p tco --dport 3306 --comment "web->db tcp/3306"`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 削除 | policy | rm || \<policy-id> |

example: `raind policy rm 01kgtyrrnrxvncpjcmjz2rtnmq`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| NS(外部通信)モード変更 | policy | ns-mode || \<observe\|enforce> |

example: `raind policy ns-mode enforce`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 変更取り消し | policy | revert |||

example: `raind policy revert`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 変更適用 | policy | commit |||

example: `raind policy commit`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 一覧 | policy | ls |||

example: `raind policy ls`

