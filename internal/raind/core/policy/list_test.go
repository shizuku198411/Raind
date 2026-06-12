package policy

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrintPolicyListRendersEastWestHeadersAndHosts(t *testing.T) {
	out := captureStdout(t, func() {
		(&ServicePolicyList{}).printPolicyList("RAIND-EW", "enforce", []PolicyModel{{
			Id:          "p1",
			Status:      "applied",
			Source:      HostInfoModel{ContainerName: "web"},
			Destination: HostInfoModel{ContainerName: "db"},
			Protocol:    "tcp",
			DestPort:    5432,
			Comment:     "allow db",
		}}, true)
	})

	assert.Contains(t, out, "POLICY TYPE : East-West")
	assert.Contains(t, out, "DST CONTAINER")
	assert.Contains(t, out, "web")
	assert.Contains(t, out, "db")
	assert.Contains(t, out, "ALLOW")
}

func TestPrintPolicyListRendersNorthSouthAddress(t *testing.T) {
	out := captureStdout(t, func() {
		(&ServicePolicyList{}).printPolicyList("RAIND-NS-OBS", "observe_next_commit", []PolicyModel{{
			Id:          "p1",
			Status:      "before_commit",
			Source:      HostInfoModel{ContainerName: "web"},
			Destination: HostInfoModel{Address: "8.8.8.8"},
			Protocol:    "udp",
		}}, true)
	})

	assert.Contains(t, out, "POLICY TYPE : North-South")
	assert.Contains(t, out, "CURRENT MODE: observe (Next commit)")
	assert.Contains(t, out, "DST ADDR")
	assert.Contains(t, out, "8.8.8.8")
	assert.Contains(t, out, "DENY")
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()
	require.NoError(t, w.Close())
	b, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(b)
}
