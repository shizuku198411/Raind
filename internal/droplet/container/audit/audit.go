package audit

import (
	"raind/internal/droplet/logs"
	"raind/internal/droplet/spec"
)

type Recorder struct {
	record logs.AuditRecord
}

func New(containerId string, event string) *Recorder {
	return &Recorder{
		record: logs.AuditRecord{
			ContainerId: containerId,
			Event:       event,
		},
	}
}

func (r *Recorder) Stage(stage string) {
	r.record.Stage = stage
}

func (r *Recorder) SetSpec(containerSpec *spec.Spec) {
	r.record.Spec = containerSpec
}

func (r *Recorder) SetPid(pid int) {
	r.record.Pid = pid
}

func (r *Recorder) SetCommand(command *[]string) {
	r.record.Command = command
}

func (r *Recorder) SetSignals(signals *[]string) {
	r.record.Signals = signals
}

func (r *Recorder) Record(errp *error) {
	result := "success"
	if errp != nil && *errp != nil {
		result = "fail"
		r.record.Error = *errp
	}
	r.record.Result = result
	_ = logs.RecordAuditLog(r.record)
}
