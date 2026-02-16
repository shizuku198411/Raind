# Raind - Service
ServiceはPodを対象としたL4ロードバランサです。  
ラベルセレクタでPodを選択し、iptables(DNAT)でトラフィックを分配します。

## マニフェスト例
```yaml
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

## 作成
Serviceはマニフェストから作成します。
```
$ raind resource apply -f /path/to/service.yaml
resource: demo-svc applied
```

Service単体の作成コマンドも利用できます。
```
$ raind resource service create -f /path/to/service.yaml
service: <service-id> created
```

## 一覧/詳細/削除
```
$ raind resource service ls
$ raind resource service show <service-id>
$ raind resource service rm <service-id>
```

## マニフェストで削除
```
$ raind resource rm -f /path/to/service.yaml
service: demo-svc removed
```
