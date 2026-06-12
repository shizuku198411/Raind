# Raind - Command List
Raind CLI は `raind` グループに所属する非rootユーザーでの実行が前提となります。

## Container

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 作成 | container | create | --network ネットワーク名 | <image:tag> [command arg1 arg2 ...] |
|||| --volume, -v <host-dir>:<container-dir> ホストディレクトリマウント ||
|||| --publish, -p <sourceport>:<hostport>[:protocol] ポートフォワード ||
|||| --env, -e <KEY=VALUE> 環境変数 ||
|||| --tty, -t TTY接続 ||
|||| --interactive, -i 対話モード ||
|||| --name <container-name> コンテナ名 ||
|||| --pod <pod-id> Podに所属 ||

example: `raind container create -t --name web -v /mnt/web:/var/www/html -p 8080:80 nginx:latest`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 起動 | container | start | --tty, -t TTY接続 | <container-id> |

example: `raind container start -t web`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 停止 | container | stop || <container-id> |

example: `raind container stop web`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 削除 | container | rm || <container-id> |

example: `raind container rm web`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 一覧 | container | ls |||

example: `raind container ls`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 接続(アタッチ) | container | attach || <container-id> |

example: `raind container attach web`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 作成+起動(+接続) | container | run | --network ネットワーク名 | <image:tag> [command arg1 arg2 ...] |
|||| --volume, -v <host-dir>:<container-dir> ホストディレクトリマウント ||
|||| --publish, -p <sourceport>:<hostport>[:protocol] ポートフォワード ||
|||| --env, -e <KEY=VALUE> 環境変数 ||
|||| --tty, -t TTY接続 ||
|||| --rm 終了時に削除 ||
|||| --name <container-name> コンテナ名 ||

example: `raind container run -t --rm --name web -v /mnt/web:/var/www/html -p 8080:80 nginx:latest`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| コマンド実行 | container | exec | --tty, -t TTY接続 | <container-id> <command arg1 arg2 ...> |

example: `raind container exec -t web /bin/sh -c "echo Hello World! > hello.txt"`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| ログ確認 | container | logs | --line 行数 | <container-id> |
|||| --pager ページャで表示 ||

example: `raind container logs --line 200 --pager web`

## Bottle
Bottleは複数のコンテナを1つのグループとして管理します。(docker-compose相当)  
※詳細は [Bottle Usage](bottle.md) を参照

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 作成 | bottle | create | --file, -f <bottle-file-path> Bottle定義ファイル (*) ||

example: `raind bottle create -f ~/myapp/Dripfile.yaml`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 起動 | bottle | start || <bottle-id|bottle-name> |

example: `raind bottle start myapp`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 停止 | bottle | stop || <bottle-id|bottle-name> |

example: `raind bottle stop myapp`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 削除 | bottle | rm || <bottle-id|bottle-name> |

example: `raind bottle rm myapp`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 一覧 | bottle | ls |||

example: `raind bottle ls`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 詳細情報 | bottle | show || <bottle-id|bottle-name> |

example: `raind bottle show myapp`

## Image

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 取得 | image | pull | --os OS指定 | <image:tag> |
|||| --arch ARCH指定 ||

example: `raind image pull alpine:latest`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| ビルド | image | build | --file, -f コンテキストディレクトリ (*) ||
|||| --tag, -t <image:tag> イメージ名/タグ (*) ||

example: `raind image build -f ~/myapp -t myapp:latest`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 削除 | image | rm || <image:tag> |

example: `raind image rm alpine:latest`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 一覧 | image | ls |||

example: `raind image ls`

## Network

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 作成 | network | create || <network-name> |

example: `raind network create raind0`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 削除 | network | rm || <network-name> |

example: `raind network rm raind0`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 一覧 | network | ls |||

example: `raind network ls`

## Resource

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 適用 | resource | apply | --file, -f リソース定義YAML (*) ||

example: `raind resource apply -f path/to/manifest.yaml`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 削除 | resource | rm | --file, -f リソース定義YAML (*) ||

example: `raind resource rm -f path/to/manifest.yaml`

### Resource Pod

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 作成 | resource pod | create | --name, -n <pod-name> Pod名 (*) ||
|||| --namespace Namespace (default: default) ||
|||| --uid UID ||
|||| --label, -l <key=value> ラベル ||
|||| --annotation, -a <key=value> アノテーション ||

example: `raind resource pod create -n demo -l app=demo`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 一覧 | resource pod | ls |||

example: `raind resource pod ls`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 起動 | resource pod | start || <pod-id> |

example: `raind resource pod start <pod-id>`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 停止 | resource pod | stop || <pod-id> |

example: `raind resource pod stop <pod-id>`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 削除 | resource pod | rm || <pod-id> |

example: `raind resource pod rm <pod-id>`

### Resource ReplicaSet

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 一覧 | resource replicaset | ls(get) |||

example: `raind resource replicaset ls`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 詳細 | resource replicaset | show(describe) || <replicaset-id> |

example: `raind resource replicaset show <replicaset-id>`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| スケール | resource replicaset | scale | --replicas, -r <num> レプリカ数 (*) | <replicaset-id> |

example: `raind resource replicaset scale <replicaset-id> -r 3`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 削除 | resource replicaset | rm(delete) || <replicaset-id> |

example: `raind resource replicaset rm <replicaset-id>`

### Resource Service

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 作成 | resource service | create | --file, -f サービス定義YAML (*) ||

example: `raind resource service create -f path/to/service.yaml`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 一覧 | resource service | ls |||

example: `raind resource service ls`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 詳細 | resource service | show || <service-id> |

example: `raind resource service show <service-id>`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 削除 | resource service | rm || <service-id> |

example: `raind resource service rm <service-id>`

## Policy
※全てのポリシー変更操作は、`commit`サブコマンドを実行するまでは実際のポリシーに反映されません。

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 作成 | policy | add | --type <ew|ns-obs|ns-enf> ポリシータイプ (*) ||
|||| --source, -s <container-name> 送信元コンテナ名 (*) ||
|||| --destination, -d <container-name> 宛先コンテナ名 (*) ||
|||| --protocol, -p <icmp|tcp|udp> プロトコル ||
|||| --dport <dest-port> 宛先ポート ||
|||| --comment <comment> コメント ||

example: `raind policy add --type ew -s web -d db -p tcp --dport 3306 --comment "web->db tcp/3306"`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 削除 | policy | rm || <policy-id> |

example: `raind policy rm 01kgtyrrnrxvncpjcmjz2rtnmq`

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| NS(外部通信)モード変更 | policy | ns-mode || <observe|enforce> |

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
| 一覧 | policy | ls | --type <ew|ns-obs|ns-enf> フィルタ ||

example: `raind policy ls --type ew`

## Logs

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| Netflowログ | logs | netflow | --line 行数 ||
|||| --pager ページャで表示 ||
|||| --json JSON表示 ||
|||| --target, -t <container-name|address> フィルタ ||

example: `raind logs netflow --line 200 --pager --json --target web`

## Completion

| 操作 | コマンドグループ | サブコマンド | オプション (*:必須) | 引数 |
|:--|:--|:--|:--|:--|
| 補完生成 | completion | - || bash|zsh|fish |

example: `raind completion bash > /path/to/raind.bash`
