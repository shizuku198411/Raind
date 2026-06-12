package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeK8sServiceManifest(t *testing.T) {
	body := []byte(`
apiVersion: v1
kind: Service
metadata:
  name: web
spec:
  selector:
    app: web
  ports:
    - port: 80
    - port: 443
      targetPort: 8443
      protocol: TCP
`)

	got, err := DecodeK8sServiceManifest(body)

	require.NoError(t, err)
	assert.Equal(t, "web", got.Name)
	assert.Equal(t, "default", got.Namespace)
	assert.Equal(t, map[string]string{"app": "web"}, got.Selector)
	require.Len(t, got.Ports, 2)
	assert.Equal(t, 80, got.Ports[0].TargetPort)
	assert.Equal(t, "tcp", got.Ports[0].Protocol)
	assert.Equal(t, 8443, got.Ports[1].TargetPort)
}

func TestDecodeK8sServiceManifestRejectsUnsupportedKind(t *testing.T) {
	_, err := DecodeK8sServiceManifest([]byte("kind: Pod\nmetadata:\n  name: x\n"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported kind")
}
