package logs

import (
	"droplet/internal/spec"
	"droplet/internal/utils"
	"log"
	"time"
)

type AuditRecord struct {
	ContainerId string
	Event       string
	Stage       string
	Pid         int
	Command     *[]string
	Signals     *[]string
	Spec        *spec.Spec
	Result      string
	Error       error
}

func RecordAuditLog(auditRecord AuditRecord) error {
	rec := &Record{
		TS:          time.Now(),
		LogVersion:  "0.1.0",
		Event:       auditRecord.Event,
		Runtime:     "droplet",
		RuntimeVer:  "0.1.0",
		ContainerId: auditRecord.ContainerId,

		Bundle:     utils.ContainerDir(auditRecord.ContainerId),
		ConfigPath: utils.ConfigFilePath(auditRecord.ContainerId),
		StatePath:  utils.ContainerStatePath(auditRecord.ContainerId),
		Pid:        auditRecord.Pid,

		Result: auditRecord.Result,
	}

	// oci
	configHash, _ := utils.Sha256File(rec.ConfigPath)
	rec.Oci = &OciInfo{
		ConfigSHA256: configHash,
	}

	if auditRecord.Command != nil {
		rec.ExecCommand = *auditRecord.Command
	}

	if auditRecord.Signals != nil {
		rec.Signals = *auditRecord.Signals
	}

	if auditRecord.Spec != nil {
		rec.Oci.ProcessArg0 = auditRecord.Spec.Process.Args[0]
		rec.Namespaces = mapNamespace(auditRecord.Spec.LinuxSpec)
		rec.Capabilities = &CapsInfo{
			Bounding:    auditRecord.Spec.Process.Capabilities.Bounding,
			Effective:   auditRecord.Spec.Process.Capabilities.Effective,
			Permitted:   auditRecord.Spec.Process.Capabilities.Permitted,
			Inheritable: auditRecord.Spec.Process.Capabilities.Inheritable,
			Ambient:     auditRecord.Spec.Process.Capabilities.Ambient,
		}
		rec.Seccomp = &SeccompInfo{
			DefaultAction: auditRecord.Spec.LinuxSpec.Seccomp.DefaultAction,
		}
		rec.LSM = &LsmInfo{
			AppArmor: &AppArmorInfo{
				Profile: auditRecord.Spec.LinuxSpec.AppArmorProfile,
			},
		}
	}

	// error
	if auditRecord.Result != "success" {
		rec.Error = &ErrInfo{
			Stage:   auditRecord.Stage,
			Message: auditRecord.Error.Error(),
		}
	}

	if err := AuditLogger.WriteRecord(rec); err != nil {
		log.Printf("audit log write failed: %v", err)
	}

	return nil
}

type AuditHookRecord struct {
	ContainerId string
	Event       string
	Hook        HookResult
	Result      string
}

func RecordHookAuditLog(auditHookRecord AuditHookRecord) error {
	rec := &Record{
		TS:          time.Now(),
		LogVersion:  "0.1.0",
		Event:       auditHookRecord.Event,
		Runtime:     "droplet",
		RuntimeVer:  "0.1.0",
		ContainerId: auditHookRecord.ContainerId,

		Hook: &auditHookRecord.Hook,

		Result: auditHookRecord.Result,
	}

	if err := AuditLogger.WriteRecord(rec); err != nil {
		log.Printf("audit log write failed: %v", err)
	}

	return nil
}

func mapNamespace(linuxObject spec.LinuxSpecObject) map[string]bool {
	mapNs := map[string]bool{
		"mount":   false,
		"network": false,
		"uts":     false,
		"pid":     false,
		"ipc":     false,
		"user":    false,
		"cgroup":  false,
	}

	for _, ns := range linuxObject.Namespaces {
		if _, ok := mapNs[ns.Type]; ok {
			mapNs[ns.Type] = true
		}
	}
	return mapNs
}
