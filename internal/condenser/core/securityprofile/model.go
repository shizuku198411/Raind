package securityprofile

type ProfileType string

const (
	ProfileTypeBuiltIn ProfileType = "built-in"
	ProfileTypeCustom  ProfileType = "custom"

	ProfileDefault    = "default"
	ProfileDev        = "dev"
	ProfileDeploy     = "deploy"
	ProfileRestricted = "restricted"
	ProfilePrivileged = "privileged"
	ProfileUnconfined = "unconfined"
)

type SecurityProfile struct {
	Name            string            `json:"name" yaml:"name"`
	Type            ProfileType       `json:"type" yaml:"type"`
	Extends         string            `json:"extends,omitempty" yaml:"extends,omitempty"`
	AddCap          []string          `json:"addCap,omitempty" yaml:"addCap,omitempty"`
	DropCap         []string          `json:"dropCap,omitempty" yaml:"dropCap,omitempty"`
	Capabilities    CapabilityProfile `json:"capabilities" yaml:"capabilities"`
	Seccomp         *SeccompObject    `json:"seccomp,omitempty" yaml:"seccomp,omitempty"`
	AppArmorProfile string            `json:"apparmorProfile,omitempty" yaml:"apparmorProfile,omitempty"`
	NoNewPrivileges bool              `json:"noNewPrivileges,omitempty" yaml:"noNewPrivileges,omitempty"`
}

type CapabilityProfile struct {
	Base []string `json:"base" yaml:"base"`
}

type CustomProfileManifest struct {
	APIVersion string                `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	Kind       string                `json:"kind,omitempty" yaml:"kind,omitempty"`
	Metadata   CustomProfileMetadata `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Spec       CustomProfileSpec     `json:"spec,omitempty" yaml:"spec,omitempty"`
	Name       string                `json:"name,omitempty" yaml:"name,omitempty"`
	Extends    string                `json:"extends,omitempty" yaml:"extends,omitempty"`
	AddCap     []string              `json:"addCap,omitempty" yaml:"add-cap,omitempty"`
	DropCap    []string              `json:"dropCap,omitempty" yaml:"drop-cap,omitempty"`
}

type CustomProfileMetadata struct {
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
}

type CustomProfileSpec struct {
	Extends string   `json:"extends,omitempty" yaml:"extends,omitempty"`
	AddCap  []string `json:"addCap,omitempty" yaml:"add-cap,omitempty"`
	DropCap []string `json:"dropCap,omitempty" yaml:"drop-cap,omitempty"`
}

func (m CustomProfileManifest) ProfileName() string {
	if m.Metadata.Name != "" {
		return m.Metadata.Name
	}
	return m.Name
}

func (m CustomProfileManifest) ProfileExtends() string {
	if m.Spec.Extends != "" {
		return m.Spec.Extends
	}
	return m.Extends
}

func (m CustomProfileManifest) ProfileAddCap() []string {
	if len(m.Spec.AddCap) > 0 {
		return m.Spec.AddCap
	}
	return m.AddCap
}

func (m CustomProfileManifest) ProfileDropCap() []string {
	if len(m.Spec.DropCap) > 0 {
		return m.Spec.DropCap
	}
	return m.DropCap
}

type SeccompArgObject struct {
	Index    uint    `json:"index" yaml:"index"`
	Value    uint64  `json:"value" yaml:"value"`
	Op       string  `json:"op" yaml:"op"`
	ValueTwo *uint64 `json:"valueTwo,omitempty" yaml:"valueTwo,omitempty"`
}

type SeccompFilterObject struct {
	Architectures []string `json:"architectures,omitempty" yaml:"architectures,omitempty"`
	Caps          []string `json:"caps,omitempty" yaml:"caps,omitempty"`
	MinKernel     string   `json:"minkernel,omitempty" yaml:"minkernel,omitempty"`
}

type SeccompSyscallObject struct {
	Names    []string             `json:"names" yaml:"names"`
	Action   string               `json:"action" yaml:"action"`
	ErrnoRet *uint32              `json:"errnoRet,omitempty" yaml:"errnoRet,omitempty"`
	Args     []SeccompArgObject   `json:"args,omitempty" yaml:"args,omitempty"`
	Comment  string               `json:"comment,omitempty" yaml:"comment,omitempty"`
	Include  *SeccompFilterObject `json:"includes,omitempty" yaml:"includes,omitempty"`
	Excludes *SeccompFilterObject `json:"excludes,omitempty" yaml:"excludes,omitempty"`
}

type SeccompObject struct {
	DefaultAction   string                 `json:"defaultAction" yaml:"defaultAction"`
	DefaultErrnoRet *uint32                `json:"defaultErrnoRet,omitempty" yaml:"defaultErrnoRet,omitempty"`
	Architectures   []string               `json:"architectures,omitempty" yaml:"architectures,omitempty"`
	Flags           []string               `json:"flags,omitempty" yaml:"flags,omitempty"`
	Syscalls        []SeccompSyscallObject `json:"syscalls,omitempty" yaml:"syscalls,omitempty"`
}

type ProfileSummary struct {
	Name              string      `json:"name"`
	Type              ProfileType `json:"type"`
	CapabilitiesCount int         `json:"capabilitiesCount"`
	SeccompEnabled    bool        `json:"seccompEnabled"`
	AppArmorProfile   string      `json:"apparmorProfile,omitempty"`
	NoNewPrivileges   bool        `json:"noNewPrivileges,omitempty"`
}
