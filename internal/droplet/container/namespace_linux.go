package container

import (
	"fmt"
	"os"
	"strconv"
	"syscall"

	"golang.org/x/sys/unix"
	"raind/internal/droplet/spec"
	"raind/internal/droplet/utils"
)

// procAttr represents the low-level process attributes that will be applied
// when starting the container init process.
//
// At present it contains only the cloneFlags derived from the selected
// namespaces, but the struct exists to allow future extension (for example,
// UID/GID mappings, capability settings, or seccomp configuration) without
// changing the function signatures that depend on it.
type procAttr struct {
	cloneFlags    uintptr
	uidMap        []syscall.SysProcIDMap
	gidMap        []syscall.SysProcIDMap
	setGroupsFlag bool
}

// buildSysProcAttr converts the given procAttr into a syscall.SysProcAttr,
// which can be assigned to exec.Cmd.SysProcAttr when launching the init
// process.
//
// The returned SysProcAttr currently sets only the Cloneflags field, but
// additional process attributes (such as UID/GID mappings for user
// namespaces) may be added here in the future as the runtime evolves.
func buildSysProcAttr(procAttr procAttr) *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Cloneflags:                 procAttr.cloneFlags,
		UidMappings:                procAttr.uidMap,
		GidMappings:                procAttr.gidMap,
		GidMappingsEnableSetgroups: procAttr.setGroupsFlag,
	}
}

// buildProcAttrForRootContainer builds a procAttr for a root-executed
// container, including user namespaces if requested in nsConfig.
//
// For now this configures a simple identity mapping (0 -> 0, size 65535)
// when the user namespace is enabled. Other namespaces are expressed as
// clone flags only. This function can be extended to support additional
// mappings (e.g. non-root users, rootless mode, or spec-based mappings)
// without changing the SysProcAttr construction logic.
func buildProcAttrForRootContainer(nsConfig namespaceConfig) procAttr {
	cloneFlags := buildCloneFlags(nsConfig)
	uidMap, gidMap := buildRootUserNamespaceIDMap(nsConfig)
	setGroupsFlag := nsConfig.user

	return procAttr{
		cloneFlags:    cloneFlags,
		uidMap:        uidMap,
		gidMap:        gidMap,
		setGroupsFlag: setGroupsFlag,
	}
}

// buildProcAttrForContainer builds process attributes from the OCI spec.
//
// Rootless containers are expressed as a Raind annotation.
// When the annotation is enabled, the container still gets UID/GID 0 inside
// its user namespace, but that root ID is mapped to unprivileged host UID/GID range.
func buildProcAttrForContainer(containerSpec spec.Spec) procAttr {
	nsConfig := buildNamespaceConfig(containerSpec)
	cloneFlags := buildCloneFlags(nsConfig)
	uidMap, gidMap := buildRootUserNamespaceIDMap(nsConfig)
	setGroupsFlag := nsConfig.user
	if isRootlessSpec(containerSpec) {
		uidMap, gidMap = buildRootlessUserNamespaceIDMap(nsConfig)
		// Rootless mappings should not allow setgroups in the child user namespace.
		// Leaving this false makes Go write "deny" before gid_map.
		setGroupsFlag = false
	}

	return procAttr{
		cloneFlags:    cloneFlags,
		uidMap:        uidMap,
		gidMap:        gidMap,
		setGroupsFlag: setGroupsFlag,
	}
}

// namespaceConfig represents the set of Linux namespaces that should be
// created for the container's init process.
//
// Each field corresponds to an OCI runtime-spec namespace type.
// A value of true indicates that the namespace should be created
// (i.e., the associated CLONE_NEW* flag will be applied).
type namespaceConfig struct {
	mount   bool
	network bool
	uts     bool
	pid     bool
	ipc     bool
	user    bool
	cgroup  bool

	mountPath   string
	networkPath string
	utsPath     string
	pidPath     string
	ipcPath     string
	userPath    string
	cgroupPath  string
}

// buildNamespaceConfig constructs a namespaceConfig from the namespaces
// defined in the OCI runtime-spec.
//
// The function inspects spec.LinuxSpec.Namespaces and marks each namespace
// as enabled in the returned namespaceConfig. If a namespace type is not
// present in the spec, the corresponding field remains false.
//
// This function does not perform any system calls; it simply derives the
// configuration that will later be used to construct SysProcAttr.
func buildNamespaceConfig(spec spec.Spec) namespaceConfig {
	var nsConfig namespaceConfig
	for _, ns := range spec.LinuxSpec.Namespaces {
		switch ns.Type {
		case "mount":
			if ns.Path != "" {
				nsConfig.mountPath = ns.Path
				nsConfig.mount = false
				break
			}
			if nsConfig.mountPath == "" {
				nsConfig.mount = true
			}
		case "network":
			if ns.Path != "" {
				nsConfig.networkPath = ns.Path
				nsConfig.network = false
				break
			}
			if nsConfig.networkPath == "" {
				nsConfig.network = true
			}
		case "uts":
			if ns.Path != "" {
				nsConfig.utsPath = ns.Path
				nsConfig.uts = false
				break
			}
			if nsConfig.utsPath == "" {
				nsConfig.uts = true
			}
		case "pid":
			if ns.Path != "" {
				nsConfig.pidPath = ns.Path
				nsConfig.pid = false
				break
			}
			if nsConfig.pidPath == "" {
				nsConfig.pid = true
			}
		case "ipc":
			if ns.Path != "" {
				nsConfig.ipcPath = ns.Path
				nsConfig.ipc = false
				break
			}
			if nsConfig.ipcPath == "" {
				nsConfig.ipc = true
			}
		case "user":
			if ns.Path != "" {
				nsConfig.userPath = ns.Path
				nsConfig.user = false
				break
			}
			if nsConfig.userPath == "" {
				nsConfig.user = true
			}
		case "cgroup":
			if ns.Path != "" {
				nsConfig.cgroupPath = ns.Path
				nsConfig.cgroup = false
				break
			}
			if nsConfig.cgroupPath == "" {
				nsConfig.cgroup = true
			}
		}
	}
	return nsConfig
}

// buildCloneFlags constructs the Linux namespace clone flags from the given
// namespaceConfig and returns the bitwise OR of the corresponding CLONE_NEW*
//
// Each enabled namespace in nsConfig results in the associated clone flag
// being added to the returned value. The resulting flags value is intended
// to be used as syscall.SysProcAttr.Cloneflags when spawning the container
// init process.
//
// This function does not perform any system calls; it only derives the flag
// mask based on the requested namespaces.
func buildCloneFlags(nsConfig namespaceConfig) uintptr {
	var flags uintptr

	if nsConfig.mount {
		flags |= syscall.CLONE_NEWNS
	}
	if nsConfig.network {
		flags |= syscall.CLONE_NEWNET
	}
	if nsConfig.uts {
		flags |= syscall.CLONE_NEWUTS
	}
	if nsConfig.pid {
		flags |= syscall.CLONE_NEWPID
	}
	if nsConfig.ipc {
		flags |= syscall.CLONE_NEWIPC
	}
	if nsConfig.user {
		flags |= syscall.CLONE_NEWUSER
	}
	if nsConfig.cgroup {
		flags |= syscall.CLONE_NEWCGROUP
	}

	return flags
}

type namespaceJoinTarget struct {
	name string
	path string
}

// buildNamespaceJoinTargets returns the list of namespaces that should be joined
// via setns based on explicit Path entries in the spec. Only namespaces with
// Path set are included. Order is fixed to avoid surprises.
func buildNamespaceJoinTargets(spec spec.Spec) []namespaceJoinTarget {
	nsConfig := buildNamespaceConfig(spec)
	var targets []namespaceJoinTarget

	if nsConfig.networkPath != "" {
		targets = append(targets, namespaceJoinTarget{name: "network", path: nsConfig.networkPath})
	}
	if nsConfig.ipcPath != "" {
		targets = append(targets, namespaceJoinTarget{name: "ipc", path: nsConfig.ipcPath})
	}
	if nsConfig.utsPath != "" {
		targets = append(targets, namespaceJoinTarget{name: "uts", path: nsConfig.utsPath})
	}

	return targets
}

// joinExistingNamespaces applies setns for any namespaces that specify Path in
// the OCI spec. User namespaces are intentionally ignored here; they must be
// handled by the higher-level runtime to avoid setns EINVAL in Go.
func joinExistingNamespaces(spec spec.Spec) error {
	targets := buildNamespaceJoinTargets(spec)
	for _, t := range targets {
		if t.name == "user" {
			continue
		}
		f, err := os.Open(t.path)
		if err != nil {
			return fmt.Errorf("open %s namespace: %w", t.name, err)
		}
		nstype := 0
		if t.name == "user" {
			nstype = unix.CLONE_NEWUSER
		}
		if err := unix.Setns(int(f.Fd()), nstype); err != nil {
			_ = f.Close()
			return fmt.Errorf("setns %s: %w", t.name, err)
		}
		_ = f.Close()
	}
	return nil
}

// buildRootUserNamespaceIDMaps returns UID/GID ID maps suitable for a
// root-executed container when the user namespace is enabled.
//
// When nsConfig.user is true, this function creates an identity mapping
// from container UID/GID 0..(size-1) to host UID/GID 0..(size-1).
// When the user namespace is disabled, it returns nil maps.
//
// This function is the main extension point for future mapping policies:
// for example, supporting rootless containers, using /etc/subuid/subgid,
// or honoring OCI spec.Process.User fields.
func buildRootUserNamespaceIDMap(nsConfig namespaceConfig) (uidMap, gidMap []syscall.SysProcIDMap) {
	if !nsConfig.user {
		return nil, nil
	}

	const idMapSize = 65535

	uidMap = []syscall.SysProcIDMap{
		{
			ContainerID: 0,
			HostID:      0,
			Size:        idMapSize,
		},
	}
	gidMap = []syscall.SysProcIDMap{
		{
			ContainerID: 0,
			HostID:      0,
			Size:        idMapSize,
		},
	}

	return uidMap, gidMap
}

func buildRootlessUserNamespaceIDMap(nsConfig namespaceConfig) (uidMap, gidMap []syscall.SysProcIDMap) {
	if !nsConfig.user {
		return nil, nil
	}

	uidBase := envInt("RAIND_ROOTLESS_UID_BASE", 100000)
	gidBase := envInt("RAIND_ROOTLESS_GID_BASE", 100000)
	mapSize := envInt("RAIND_ROOTLESS_ID_MAP_SIZE", 65536)

	uidMap = []syscall.SysProcIDMap{
		{
			ContainerID: 0,
			HostID:      uidBase,
			Size:        mapSize,
		},
	}
	gidMap = []syscall.SysProcIDMap{
		{
			ContainerID: 0,
			HostID:      gidBase,
			Size:        mapSize,
		},
	}

	return uidMap, gidMap
}

func isRootlessSpec(containerSpec spec.Spec) bool {
	if containerSpec.Annotations.Rootless == "" {
		return false
	}
	var rootless spec.RootlessConfigObject
	if err := utils.StringToJson(containerSpec.Annotations.Rootless, &rootless); err != nil {
		return false
	}
	return rootless.Enabled
}

func envInt(name string, fallback int) int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
