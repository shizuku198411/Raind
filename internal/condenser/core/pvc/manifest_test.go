package pvc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseStorageQuantity(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want uint64
	}{
		{name: "bytes", in: "1024", want: 1024},
		{name: "decimal K", in: "1K", want: 1000},
		{name: "decimal M", in: "1M", want: 1000 * 1000},
		{name: "decimal G", in: "1G", want: 1000 * 1000 * 1000},
		{name: "binary Ki", in: "1Ki", want: 1024},
		{name: "binary Mi", in: "1Mi", want: 1024 * 1024},
		{name: "binary Gi", in: "1Gi", want: 1024 * 1024 * 1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseStorageQuantity(tt.in)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseStorageQuantityRejectsInvalidValues(t *testing.T) {
	for _, value := range []string{"", "0", "-1Gi", "1.5Gi", "1Pi", "abc"} {
		t.Run(value, func(t *testing.T) {
			_, err := ParseStorageQuantity(value)
			require.Error(t, err)
		})
	}
}

func TestDecodeK8sPVCManifest(t *testing.T) {
	got, err := DecodeK8sPVCManifest([]byte(`
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: db-data
  namespace: demo
  annotations:
    raind.dev/reclaimPolicy: Delete
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
`))
	require.NoError(t, err)
	assert.Equal(t, "db-data", got.Name)
	assert.Equal(t, "demo", got.Namespace)
	assert.Equal(t, "Delete", got.ReclaimPolicy)
	assert.Equal(t, uint64(1024*1024*1024), got.RequestedBytes)
	assert.Equal(t, "Filesystem", got.VolumeMode)
}
