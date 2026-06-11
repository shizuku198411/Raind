package spec

type RootObject struct {
	Path string `json:"path"`
}

type MountObject struct {
	Destination string   `json:"destination"`
	Type        string   `json:"type"`
	Source      string   `json:"source"`
	Options     []string `json:"options"`
}

type CapabilityObject struct {
	Bounding    []string `json:"bounding"`
	Permitted   []string `json:"permitted"`
	Inheritable []string `json:"inheritable"`
	Effective   []string `json:"effective"`
	Ambient     []string `json:"ambient"`
}

type ProcessObject struct {
	Cwd          string           `json:"cwd"`
	Env          []string         `json:"env"`
	Args         []string         `json:"args"`
	Capabilities CapabilityObject `json:"capabilities"`
}

type MemoryObject struct {
	Limit int `json:"limit"`
}

type CpuObject struct {
	Period int `json:"period"`
	Quota  int `json:"quota"`
}

type ResourceObject struct {
	Memory MemoryObject `json:"memory"`
	Cpu    CpuObject    `json:"cpu"`
}

type NamespaceObject struct {
	Type string `json:"type"`
	Path string `json:"path,omitempty"`
}

type SeccompArgObject struct {
	Index    uint    `json:"index"`
	Value    uint64  `json:"value"`
	Op       string  `json:"op"`
	ValueTwo *uint64 `json:"valueTwo,omitempty"`
}

type SeccompFilterObject struct {
	Architectures []string `json:"architectures,omitempty"`
	Caps          []string `json:"caps,omitempty"`
	MinKernel     string   `json:"minkernel,omitempty"`
}

type SeccompSyscallObject struct {
	Names    []string             `json:"names"`
	Action   string               `json:"action"`
	ErrnoRet *uint32              `json:"errnoRet,omitempty"`
	Args     []SeccompArgObject   `json:"args,omitempty"`
	Comment  string               `json:"comment,omitempty"`
	Include  *SeccompFilterObject `json:"includes,omitempty"`
	Excludes *SeccompFilterObject `json:"excludes,omitempty"`
}

type SeccompObject struct {
	DefaultAction   string                 `json:"defaultAction"`
	DefaultErrnoRet *uint32                `json:"defaultErrnoRet,omitempty"`
	Architectures   []string               `json:"architectures,omitempty"`
	Flags           []string               `json:"flags,omitempty"`
	Syscalls        []SeccompSyscallObject `json:"syscalls,omitempty"`
}

type LinuxSpecObject struct {
	Resources       ResourceObject    `json:"resources"`
	Namespaces      []NamespaceObject `json:"namespaces"`
	Seccomp         *SeccompObject    `json:"seccomp,omitempty"`
	AppArmorProfile string            `json:"apparmorProfile,omitempty"`
}

type AnnotationObject struct {
	Version string `json:"io.raind.runtime.annotation.version"`
	Net     string `json:"io.raind.net.config"`
	Image   string `json:"io.raind.image.config"`
}

type HookObject struct {
	Path    string   `json:"path"`
	Args    []string `json:"args,omitempty"`
	Env     []string `json:"env,omitempty"`
	Timeout *int     `json:"timeout,omitempty"`
}

type HookLifecycleObject struct {
	Prestart        []HookObject `json:"prestart,omitempty"` // DEPRECATED
	CreateRuntime   []HookObject `json:"createRuntime,omitempty"`
	CreateContainer []HookObject `json:"createContainer,omitempty"`
	StartContainer  []HookObject `json:"startContainer,omitempty"`
	Poststart       []HookObject `json:"poststart,omitempty"`
	StopContainer   []HookObject `json:"stopContainer,omitempty"`
	Poststop        []HookObject `json:"poststop,omitempty"`
}

type Spec struct {
	OciVersion  string              `json:"ociVersion"`
	Root        RootObject          `json:"root"`
	Mounts      []MountObject       `json:"mounts"`
	Process     ProcessObject       `json:"process"`
	Hostname    string              `json:"hostname"`
	LinuxSpec   LinuxSpecObject     `json:"linux"`
	Hooks       HookLifecycleObject `json:"hooks,omitempty"`
	Annotations AnnotationObject    `json:"annotations"`
}

// Annotation: io.raind.net.config
type IPv4Object struct {
	Address string `json:"address"`
	Gateway string `json:"gateway"`
}

type DnsObject struct {
	Servers []string `json:"servers"`
}

type InterfaceObject struct {
	Name string     `json:"name"`
	IPv4 IPv4Object `json:"ipv4"`
	Dns  DnsObject  `json:"dns"`
}

type NetConfigObject struct {
	HostInterface   string          `json:"hostInterface"`
	BridgeInterface string          `json:"bridgeInterface"`
	Interface       InterfaceObject `json:"interface"`
}

// Annotation: io.raind.image.config
type ImageConfigObject struct {
	RootfsType string   `json:"rootfsType"`
	ImageLayer []string `json:"imageLayer"`
	UpperDir   string   `json:"upperDir"`
	WorkDir    string   `json:"workDir"`
}

type SpecHash struct {
	Sha256 string `json:"sha256"`
}
