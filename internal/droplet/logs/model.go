package logs

import "time"

type Record struct {
	TS          time.Time `json:"ts"`
	LogVersion  string    `json:"log_version"`
	Event       string    `json:"event"` // create/start/kill/delete
	Runtime     string    `json:"runtime"`
	RuntimeVer  string    `json:"runtime_version"`
	ContainerId string    `json:"container_id,omitempty"`
	Bundle      string    `json:"bundle,omitempty"`
	ConfigPath  string    `json:"config_path,omitempty"`
	StatePath   string    `json:"state_path,omitempty"`

	ExecCommand []string `json:"exec_command,omitempty"`

	Oci     *OciInfo `json:"oci,omitempty"`
	Pid     int      `json:"pid,omitempty"`
	Signals []string `json:"signals,omitempty"`

	Namespaces   map[string]bool `json:"namespaces,omitempty"`
	Capabilities *CapsInfo       `json:"capabilities,omitempty"`
	Seccomp      *SeccompInfo    `json:"seccomp,omitempty"`
	LSM          *LsmInfo        `json:"lsm,omitempty"`
	Hook         *HookResult     `json:"hook,omitempty"`

	Result string   `json:"result,omitempty"`
	Error  *ErrInfo `json:"error,omitempty"`
}

type OciInfo struct {
	ConfigSHA256  string `json:"config_sha256,omitempty"`
	SeccompSHA256 string `json:"seccomp_sha256,omitempty"`
	ProcessArg0   string `json:"process_arg0,omitempty"`
}

type UserNsInfo struct {
	Enabled bool   `json:"enabled,omitempty"`
	UIDMap  string `json:"uid_map,omitempty"`
	GIDMap  string `json:"gid_map,omitempty"`
}

type CapsInfo struct {
	Bounding    []string `json:"bounding,omitempty"`
	Effective   []string `json:"effective,omitempty"`
	Permitted   []string `json:"permitted,omitempty"`
	Inheritable []string `json:"inheritable,omitempty"`
	Ambient     []string `json:"ambient,omitempty"`
}

type SeccompInfo struct {
	DefaultAction string `json:"default_action,omitempty"`
}

type LsmInfo struct {
	AppArmor *AppArmorInfo `json:"apparmor,omitempty"`
	SELinux  *SeLinuxInfo  `json:"selinux,omitempty"`
}

type AppArmorInfo struct {
	Profile string `json:"profile,omitempty"`
}

type SeLinuxInfo struct {
	Enabled bool   `json:"enabled"`
	Label   string `json:"label,omitempty"`
}

type HookResult struct {
	Phase      string `json:"phase,omitempty"`
	Path       string `json:"path,omitempty"`
	ArgsSHA256 string `json:"args_sha256,omitempty"`
	EnvSHA256  string `json:"env_sha256,omitempty"`

	StdinSHA256 string `json:"stdin_sha256,omitempty"`
	StdinBytes  int    `json:"stdin_bytes,omitempty"`

	ExitCode   int    `json:"exit_code,omitempty"`
	DurationMS int64  `json:"duration_ms,omitempty"`
	StderrTail string `json:"stderr_tail,omitempty"`
}

type ErrInfo struct {
	Stage   string `json:"stage,omitempty"`
	Errno   string `json:"errno,omitempty"`
	Message string `json:"message,omitempty"`
}
