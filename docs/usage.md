# Raind利用方法
Raindは `sudo` による実行が前提となります。
## コンテナ操作
### 作成
```
$ raind container create [options] <image:tag>

[options]
--tty, -t: TTY接続(bash等対話形式での起動)
--name <container-name>: コンテナ名
--publish, -p <host-port:container-port>: ポートフォワード設定
--volume, -v <host-path:container-path>: ホストディレクトリのマウント設定
```

### 起動
```
$ raind container start <container-id|container-name>
```

### 接続
※コンテナ作成時に `--tty,-t` オプションによるTTY接続を行ったコンテナのみ
```
$ raind container attach <container-id|container-name>
```

### 停止
```
$ raind container stop <container-id|container-name>
```

### 作成+起動(+接続)
```
$ raind container run [options] <image:tag>

[options]
--tty, -t: TTY接続(bash等対話形式での起動)
--name <container-name>: コンテナ名
--publish, -p <host-port:container-port>: ポートフォワード設定
--volume, -v <host-path:container-path>: ホストディレクトリのマウント設定
```

### コンテナ内でのコマンド実行
```
$ raind container exec [options] <container-id|container-name> <command[,args1,args2,...]>

[options]
--tty, -t: TTY接続(bash等対話形式での起動)
```

### 削除
```
$ raind container rm <container-id|container-name>
```

### コンテナ一覧
```
$ raind container ls
```

## イメージ操作
### 取得
```
$ raind image pull <image:tag>
```

### 削除
```
$ raind image rm <image:tag>
```

### イメージ一覧
```
$ raind image ls
```

## ポリシー操作
全てのポリシー変更操作は、`commit`コマンドを実行するまでは実際のポリシーに反映されません。
### ポリシー作成
```
$ raind policy create --type <ew|ns-obs|ns-enf> \
-s <container-id|container-name> \
-d <container-id|container-name|address> \
-p <icmp|tcp|udp>
--dport <port>
```

### ポリシー削除
```
$ raind policy rm <policy-id>
```

### North-Southモード変更
```
$ raind policy ns-mode <observe|enforce>
```

### 変更内容の取り消し
```
$ raind policy revert
```

### 変更内容の適用
```
$ raind policy commit
```

### ポリシー一覧
```
$ raind policy ls
```