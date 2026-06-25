package spec

import "encoding/json"

type RootObject struct {
	Path     string `json:"path"`
	Readonly bool   `json:"readonly,omitempty"`
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

type UserObject struct {
	UID            int   `json:"uid"`
	GID            int   `json:"gid"`
	Umask          *int  `json:"umask,omitempty"`
	AdditionalGids []int `json:"additionalGids,omitempty"`
}

type RlimitObject struct {
	Type string `json:"type"`
	Hard uint64 `json:"hard"`
	Soft uint64 `json:"soft"`
}

type ConsoleSizeObject struct {
	Height uint `json:"height"`
	Width  uint `json:"width"`
}

type ProcessObject struct {
	Cwd             string             `json:"cwd"`
	Env             []string           `json:"env"`
	Args            []string           `json:"args"`
	Terminal        bool               `json:"terminal,omitempty"`
	ConsoleSize     *ConsoleSizeObject `json:"consoleSize,omitempty"`
	OOMScoreAdj     *int               `json:"oomScoreAdj,omitempty"`
	Capabilities    CapabilityObject   `json:"capabilities"`
	User            UserObject         `json:"user,omitempty"`
	Rlimits         []RlimitObject     `json:"rlimits,omitempty"`
	NoNewPrivileges bool               `json:"noNewPrivileges,omitempty"`
}

type MemoryObject struct {
	Limit int `json:"limit"`
}

type CpuObject struct {
	Period int `json:"period"`
	Quota  int `json:"quota"`
}

type PidsObject struct {
	Limit int `json:"limit"`
}

type DeviceCgroupObject struct {
	Allow  bool   `json:"allow"`
	Type   string `json:"type,omitempty"`
	Major  *int64 `json:"major,omitempty"`
	Minor  *int64 `json:"minor,omitempty"`
	Access string `json:"access,omitempty"`
}

type ResourceObject struct {
	Memory  MemoryObject         `json:"memory"`
	Cpu     CpuObject            `json:"cpu"`
	Pids    PidsObject           `json:"pids,omitempty"`
	Devices []DeviceCgroupObject `json:"devices,omitempty"`
}

type DeviceObject struct {
	Path     string  `json:"path"`
	Type     string  `json:"type"`
	Major    int64   `json:"major"`
	Minor    int64   `json:"minor"`
	FileMode *uint32 `json:"fileMode,omitempty"`
	UID      *uint32 `json:"uid,omitempty"`
	GID      *uint32 `json:"gid,omitempty"`
}

type NamespaceObject struct {
	Type string `json:"type"`
	Path string `json:"path,omitempty"`
}

type IDMappingObject struct {
	ContainerID int `json:"containerID"`
	HostID      int `json:"hostID"`
	Size        int `json:"size"`
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
	Resources         ResourceObject    `json:"resources"`
	Namespaces        []NamespaceObject `json:"namespaces"`
	Devices           []DeviceObject    `json:"devices,omitempty"`
	UIDMappings       []IDMappingObject `json:"uidMappings,omitempty"`
	GIDMappings       []IDMappingObject `json:"gidMappings,omitempty"`
	MaskedPaths       []string          `json:"maskedPaths,omitempty"`
	ReadonlyPaths     []string          `json:"readonlyPaths,omitempty"`
	RootfsPropagation string            `json:"rootfsPropagation,omitempty"`
	Seccomp           *SeccompObject    `json:"seccomp,omitempty"`
	AppArmorProfile   string            `json:"apparmorProfile,omitempty"`
}

type AnnotationObject struct {
	Version  string            `json:"io.raind.runtime.annotation.version"`
	Net      string            `json:"io.raind.net.config"`
	Image    string            `json:"io.raind.image.config"`
	Rootless string            `json:"io.raind.rootless,omitempty"`
	Extra    map[string]string `json:"-"`
}

func (a AnnotationObject) MarshalJSON() ([]byte, error) {
	items := map[string]string{}
	for k, v := range a.Extra {
		items[k] = v
	}
	if a.Version != "" {
		items["io.raind.runtime.annotation.version"] = a.Version
	}
	if a.Net != "" {
		items["io.raind.net.config"] = a.Net
	}
	if a.Image != "" {
		items["io.raind.image.config"] = a.Image
	}
	if a.Rootless != "" {
		items["io.raind.rootless"] = a.Rootless
	}
	return json.Marshal(items)
}

func (a *AnnotationObject) UnmarshalJSON(data []byte) error {
	var items map[string]string
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}
	a.Version = items["io.raind.runtime.annotation.version"]
	a.Net = items["io.raind.net.config"]
	a.Image = items["io.raind.image.config"]
	a.Rootless = items["io.raind.rootless"]
	delete(items, "io.raind.runtime.annotation.version")
	delete(items, "io.raind.net.config")
	delete(items, "io.raind.image.config")
	delete(items, "io.raind.rootless")
	if len(items) > 0 {
		a.Extra = items
	} else {
		a.Extra = nil
	}
	return nil
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

const (
	RootlessModeShiftedRoot = "shifted-root"
	RootlessModeLoginRoot   = "login-root"
)

// Annotation: io.raind.rootless
type RootlessConfigObject struct {
	Enabled     bool   `json:"enabled"`
	Mode        string `json:"mode,omitempty"`
	HostRootUID int    `json:"hostRootUID,omitempty"`
	HostRootGID int    `json:"hostRootGID,omitempty"`
}
