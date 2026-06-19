package secret

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeK8sSecretManifestNormalizesDataAndStringData(t *testing.T) {
	manifest, err := DecodeK8sSecretManifest([]byte(`
apiVersion: v1
kind: Secret
metadata:
  name: db-secret
  namespace: demo
type: Opaque
data:
  DB_PASSWORD: cGFzc3dvcmQ=
  OVERRIDE_ME: ZnJvbS1kYXRh
stringData:
  API_TOKEN: token
  OVERRIDE_ME: from-string-data
`))

	require.NoError(t, err)
	assert.Equal(t, "db-secret", manifest.Name)
	assert.Equal(t, "demo", manifest.Namespace)
	assert.Equal(t, "password", manifest.Data["DB_PASSWORD"])
	assert.Equal(t, "token", manifest.Data["API_TOKEN"])
	assert.Equal(t, "from-string-data", manifest.Data["OVERRIDE_ME"])
}

func TestDecodeK8sSecretManifestRejectsUnsupportedType(t *testing.T) {
	_, err := DecodeK8sSecretManifest([]byte(`
apiVersion: v1
kind: Secret
metadata:
  name: tls-secret
type: kubernetes.io/tls
`))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported secret type")
}

func TestDecodeK8sSecretManifestRejectsInvalidBase64(t *testing.T) {
	_, err := DecodeK8sSecretManifest([]byte(`
apiVersion: v1
kind: Secret
metadata:
  name: broken
data:
  DB_PASSWORD: '%%%invalid%%%'
`))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not valid base64")
}
