package pod

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeK8sManifestsDecodesPod(t *testing.T) {
	body := []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: web
  labels:
    app: web
spec:
  volumes:
    - name: data
      hostPath:
        path: /host/data
  containers:
    - name: app
      image: alpine:latest
      command: ["/bin/sh"]
      args: ["-c", "echo ok"]
      env:
        - name: A
          value: B
      ports:
        - containerPort: 80
          hostPort: 8080
      volumeMounts:
        - name: data
          mountPath: /data
          readOnly: true
      securityContext:
        capabilities:
          add: ["CAP_NET_ADMIN"]
          drop: ["CAP_NET_RAW"]
      tty: true
`)

	got, err := DecodeK8sManifests(body)

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "Pod", got[0].Kind)
	assert.Equal(t, "web", got[0].Name)
	assert.Equal(t, "default", got[0].Namespace)
	require.Len(t, got[0].Containers, 1)
	c := got[0].Containers[0]
	assert.Equal(t, []string{"/bin/sh", "-c", "echo ok"}, c.Command)
	assert.Equal(t, []string{"A=B"}, c.Env)
	assert.Equal(t, []string{"8080:80"}, c.Port)
	assert.Equal(t, []string{"/host/data:/data:ro"}, c.Mount)
	assert.True(t, c.Tty)
}

func TestDecodeK8sManifestsDecodesReplicaSetDefaultReplica(t *testing.T) {
	body := []byte(`
apiVersion: apps/v1
kind: ReplicaSet
metadata:
  name: web
spec:
  selector:
    matchLabels:
      app: web
  template:
    metadata:
      labels:
        app: web
    spec:
      containers:
        - name: app
          image: alpine:latest
`)

	got, err := DecodeK8sManifests(body)

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "ReplicaSet", got[0].Kind)
	assert.Equal(t, 1, got[0].Replicas)
	assert.Equal(t, map[string]string{"app": "web"}, got[0].Selector)
}

func TestDecodeK8sManifestsRejectsUnsupportedKind(t *testing.T) {
	_, err := DecodeK8sManifests([]byte("kind: ConfigMap\nmetadata:\n  name: x\n"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported kind")
}
