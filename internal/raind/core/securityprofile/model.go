package securityprofile

type ProfileType string

const (
	ProfileTypeBuiltIn ProfileType = "built-in"
	ProfileTypeCustom  ProfileType = "custom"
)

type ProfileSummary struct {
	Name              string      `json:"name"`
	Type              ProfileType `json:"type"`
	CapabilitiesCount int         `json:"capabilitiesCount"`
	SeccompEnabled    bool        `json:"seccompEnabled"`
	AppArmorProfile   string      `json:"apparmorProfile,omitempty"`
}

type SecurityProfile struct {
	Name            string            `json:"name" yaml:"name"`
	Type            ProfileType       `json:"type" yaml:"type"`
	Extends         string            `json:"extends,omitempty" yaml:"extends,omitempty"`
	AddCap          []string          `json:"addCap,omitempty" yaml:"addCap,omitempty"`
	DropCap         []string          `json:"dropCap,omitempty" yaml:"dropCap,omitempty"`
	Capabilities    CapabilityProfile `json:"capabilities" yaml:"capabilities"`
	Seccomp         *SeccompObject    `json:"seccomp,omitempty" yaml:"seccomp,omitempty"`
	AppArmorProfile string            `json:"apparmorProfile,omitempty" yaml:"apparmorProfile,omitempty"`
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

type ListResponseData struct {
	Profiles []ProfileSummary `json:"profiles"`
}

type ShowResponseData struct {
	Profile SecurityProfile `json:"profile"`
}

type RegisterResponseData struct {
	Profile SecurityProfile `json:"profile"`
}

type DeleteResponseData struct {
	Name string `json:"name"`
}

type ListResponseModel struct {
	Status  string           `json:"status"`
	Message string           `json:"message"`
	Data    ListResponseData `json:"data"`
}

type ShowResponseModel struct {
	Status  string           `json:"status"`
	Message string           `json:"message"`
	Data    ShowResponseData `json:"data"`
}

type RegisterResponseModel struct {
	Status  string               `json:"status"`
	Message string               `json:"message"`
	Data    RegisterResponseData `json:"data"`
}

type DeleteResponseModel struct {
	Status  string             `json:"status"`
	Message string             `json:"message"`
	Data    DeleteResponseData `json:"data"`
}
