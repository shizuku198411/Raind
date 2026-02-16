# Raind - Pod
Podは複数コンテナを1つの論理単位として扱うオーケストレーションです。  
同一Pod内のコンテナは Network/UTS/IPC ネームスペースを共有します。  
Infra(=pause)コンテナが名前空間を維持し、Pod内のコンテナは同一IP/ホスト名として動作します。

## マニフェスト例
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: demo-pod
  namespace: default
  labels:
    app: demo
spec:
  containers:
  - name: web
    image: nginx:latest
  - name: sidecar
    image: alpine:latest
    tty: true
```

## 作成
Podはマニフェストからの作成を推奨します。
```
$ raind resource apply -f /path/to/pod.yaml
resource: demo-pod applied
```

Podメタデータのみを作成する場合は以下のコマンドを使用します。
```
$ raind resource pod create -n demo-pod -l app=demo
pod: <pod-id> created
```

## 一覧/起動/停止/削除
```
$ raind resource pod ls
$ raind resource pod start <pod-id>
$ raind resource pod stop <pod-id>
$ raind resource pod rm <pod-id>
```

## マニフェストで削除
```
$ raind resource rm -f /path/to/pod.yaml
pod: demo-pod removed
```
