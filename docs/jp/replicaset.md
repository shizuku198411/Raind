# Raind - ReplicaSet
ReplicaSetは指定されたテンプレートとセレクタに基づき、Podのレプリカ数を維持します。  
Controllerが希望レプリカ数に合わせてPodを自動作成/再作成します。

## マニフェスト例
```yaml
apiVersion: apps/v1
kind: ReplicaSet
metadata:
  name: demo-rs
  namespace: default
spec:
  replicas: 2
  selector:
    matchLabels:
      app: demo
  template:
    metadata:
      name: demo-pod
      labels:
        app: demo
    spec:
      containers:
      - name: nginx
        image: nginx:latest
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

ReplicaSetとServiceを1つのマニフェストに含めることも可能です。

## 作成
ReplicaSetはマニフェストから作成します。
```
$ raind resource apply -f /path/to/replicaset.yaml
resource: demo-rs applied
```

## スケール
ReplicaSetは`scale`コマンドにより動的に変更することが可能です。
```
$ raind resource replicaset scale <replicaset-id> -r <num-replicas>
```
Replicas: 0とした場合、そのReplicaSetのリソースは全て停止されます。

## 一覧/詳細/削除
```
$ raind resource replicaset ls
$ raind resource replicaset show <replicaset-id>
$ raind resource replicaset rm <replicaset-id>
```

## マニフェストによる削除
```
$ raind resource rm -f /path/to/replicaset.yaml
replicaset: demo-rs removed
```
