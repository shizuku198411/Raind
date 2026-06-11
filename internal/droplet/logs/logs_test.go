package logs

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"raind/internal/droplet/spec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileLoggerWriteRecordAndClose(t *testing.T) {
	// == setup ==
	path := filepath.Join(t.TempDir(), "audit.log")
	logger, err := OpenFileLogger(path, 1024)
	require.NoError(t, err)

	// == exercise ==
	require.NoError(t, logger.WriteRecord(&Record{Event: "create", Runtime: "droplet", Result: "success"}))
	require.NoError(t, logger.Close())
	err = logger.WriteRecord(&Record{Event: "after-close"})

	// == assert ==
	require.Error(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	require.Len(t, lines, 1)
	var rec Record
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &rec))
	assert.Equal(t, "create", rec.Event)
}

func TestFileLoggerRejectsNilAndTooLargeRecord(t *testing.T) {
	// == setup ==
	path := filepath.Join(t.TempDir(), "audit.log")
	logger, err := OpenFileLogger(path, 20)
	require.NoError(t, err)
	defer logger.Close()

	// == assert ==
	require.Error(t, logger.WriteRecord(nil))
	require.Error(t, logger.WriteRecord(&Record{Event: strings.Repeat("x", 100)}))
}

func TestCountLinesAndTrimFileToLastNLines(t *testing.T) {
	// == setup ==
	path := filepath.Join(t.TempDir(), "audit.log")
	require.NoError(t, os.WriteFile(path, []byte("a\nb\nc\nd\n"), 0644))

	// == exercise ==
	count, err := CountLines(path)
	require.NoError(t, err)
	require.NoError(t, TrimFileToLastNLines(path, 2))

	// == assert ==
	assert.Equal(t, 4, count)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "c\nd\n", string(data))
}

func TestRecordAuditLogNilLoggerIsNoop(t *testing.T) {
	// == setup ==
	old := AuditLogger
	AuditLogger = nil
	t.Cleanup(func() { AuditLogger = old })

	// == exercise/assert ==
	require.NoError(t, RecordAuditLog(AuditRecord{ContainerId: "container-1", Event: "create", Result: "success"}))
	require.NoError(t, RecordHookAuditLog(AuditHookRecord{ContainerId: "container-1", Event: "hook", Result: "success"}))
}

func TestRecordAuditLogWritesSuccessAndFailureRecords(t *testing.T) {
	// == setup ==
	path := filepath.Join(t.TempDir(), "audit.log")
	logger, err := OpenFileLogger(path, 4096)
	require.NoError(t, err)
	old := AuditLogger
	AuditLogger = logger
	t.Cleanup(func() {
		_ = logger.Close()
		AuditLogger = old
	})
	containerSpec := spec.Spec{
		Process: spec.ProcessObject{
			Args: []string{"/bin/sh"},
			Capabilities: spec.CapabilityObject{
				Bounding: []string{"CAP_CHOWN"},
			},
		},
		LinuxSpec: spec.LinuxSpecObject{
			Namespaces: []spec.NamespaceObject{{Type: "mount"}},
			Seccomp:    &spec.SeccompObject{DefaultAction: "SCMP_ACT_ALLOW"},
		},
	}

	// == exercise ==
	require.NoError(t, RecordAuditLog(AuditRecord{
		ContainerId: "container-1",
		Event:       "create",
		Pid:         123,
		Spec:        &containerSpec,
		Result:      "success",
	}))
	require.NoError(t, RecordAuditLog(AuditRecord{
		ContainerId: "container-1",
		Event:       "delete",
		Stage:       "load_spec",
		Result:      "fail",
		Error:       errors.New("boom"),
	}))

	// == assert ==
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	require.Len(t, lines, 2)
	var success Record
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &success))
	assert.Equal(t, "create", success.Event)
	assert.Equal(t, "/bin/sh", success.Oci.ProcessArg0)
	assert.True(t, success.Namespaces["mount"])
	assert.Equal(t, "SCMP_ACT_ALLOW", success.Seccomp.DefaultAction)
	var failure Record
	require.NoError(t, json.Unmarshal([]byte(lines[1]), &failure))
	require.NotNil(t, failure.Error)
	assert.Equal(t, "load_spec", failure.Error.Stage)
	assert.Equal(t, "boom", failure.Error.Message)
}

func TestRecordHookAuditLogWritesHookResult(t *testing.T) {
	// == setup ==
	path := filepath.Join(t.TempDir(), "audit.log")
	logger, err := OpenFileLogger(path, 4096)
	require.NoError(t, err)
	old := AuditLogger
	AuditLogger = logger
	t.Cleanup(func() {
		_ = logger.Close()
		AuditLogger = old
	})

	// == exercise ==
	require.NoError(t, RecordHookAuditLog(AuditHookRecord{
		ContainerId: "container-1",
		Event:       "hook",
		Result:      "success",
		Hook: HookResult{
			Phase:    "poststart",
			Path:     "/bin/hook",
			ExitCode: 0,
		},
	}))

	// == assert ==
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var rec Record
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(string(data))), &rec))
	require.NotNil(t, rec.Hook)
	assert.Equal(t, "poststart", rec.Hook.Phase)
	assert.Equal(t, "/bin/hook", rec.Hook.Path)
}
