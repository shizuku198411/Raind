package namespace

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
	"raind/internal/droplet/container/rootless"
	"raind/internal/droplet/spec"
)

// procAttr represents the low-level process attributes that will be applied
// when starting the container init process.
//
// At present it contains only the cloneFlags derived from the selected
// namespaces, but the struct exists to allow future extension (for example,
// UID/GID mappings, capability settings, or seccomp configuration) without
// changing the function signatures that depend on it.
type ProcAttr struct {
	cloneFlags    uintptr
	uidMap        []syscall.SysProcIDMap
	gidMap        []syscall.SysProcIDMap
	setGroupsFlag bool
	credential    *syscall.Credential
}

func NewProcAttr(cloneFlags uintptr, uidMap []syscall.SysProcIDMap, gidMap []syscall.SysProcIDMap, setGroupsFlag bool, credential *syscall.Credential) ProcAttr {
	return ProcAttr{
		cloneFlags:    cloneFlags,
		uidMap:        uidMap,
		gidMap:        gidMap,
		setGroupsFlag: setGroupsFlag,
		credential:    credential,
	}
}

func (p ProcAttr) CloneFlags() uintptr {
	return p.cloneFlags
}

func (p ProcAttr) UIDMap() []syscall.SysProcIDMap {
	return p.uidMap
}

func (p ProcAttr) GIDMap() []syscall.SysProcIDMap {
	return p.gidMap
}

func (p ProcAttr) SetGroupsFlag() bool {
	return p.setGroupsFlag
}

func (p ProcAttr) Credential() *syscall.Credential {
	return p.credential
}

// buildSysProcAttr converts the given procAttr into a syscall.SysProcAttr,
// which can be assigned to exec.Cmd.SysProcAttr when launching the init
// process.
//
// The returned SysProcAttr currently sets only the Cloneflags field, but
// additional process attributes (such as UID/GID mappings for user
// namespaces) may be added here in the future as the runtime evolves.
func BuildSysProcAttr(procAttr ProcAttr) *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Cloneflags:                 procAttr.cloneFlags,
		UidMappings:                procAttr.uidMap,
		GidMappings:                procAttr.gidMap,
		GidMappingsEnableSetgroups: procAttr.setGroupsFlag,
		Credential:                 procAttr.credential,
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
func BuildProcAttrForRootContainer(nsConfig Config) ProcAttr {
	cloneFlags := BuildCloneFlags(nsConfig)
	uidMap, gidMap := BuildRootUserNamespaceIDMap(nsConfig)
	setGroupsFlag := nsConfig.user

	return ProcAttr{
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
func BuildProcAttrForContainer(containerSpec spec.Spec) ProcAttr {
	nsConfig := BuildConfig(containerSpec)
	cloneFlags := BuildCloneFlags(nsConfig)
	uidMap, gidMap := BuildRootUserNamespaceIDMap(nsConfig)
	setGroupsFlag := nsConfig.user
	var credential *syscall.Credential

	if specUIDMap, specGIDMap := BuildSpecUserNamespaceIDMap(nsConfig, containerSpec.LinuxSpec.UIDMappings, containerSpec.LinuxSpec.GIDMappings); len(specUIDMap) > 0 || len(specGIDMap) > 0 {
		uidMap, gidMap = specUIDMap, specGIDMap
	} else if rootlessConfig, ok := rootless.ConfigFromSpec(containerSpec); ok {
		uidMap, gidMap = BuildRootlessUserNamespaceIDMap(nsConfig, rootlessConfig)
		// Rootless containers still need setgroups inside the child user
		// namespace for images such as nginx that drop workers to a non-root
		// user through initgroups(3). The mapped GID range keeps those groups
		// constrained to unprivileged host IDs.
		setGroupsFlag = nsConfig.user

		// Start the init process as uid/gid 0 inside the newly-created user namespace.
		// With the rootless map below, that namespace root maps to an unprivileged
		// host uid/gid such as 10000:10000. Without this, the child may start with
		// an unmapped overflow id and later fail to switch to namespace root with EPERM.
		if nsConfig.user {
			credential = &syscall.Credential{
				Uid:         0,
				Gid:         0,
				NoSetGroups: true,
			}
		}
	}

	return ProcAttr{
		cloneFlags:    cloneFlags,
		uidMap:        uidMap,
		gidMap:        gidMap,
		setGroupsFlag: setGroupsFlag,
		credential:    credential,
	}
}

// namespaceConfig represents the set of Linux namespaces that should be
// created for the container's init process.
//
// Each field corresponds to an OCI runtime-spec namespace type.
// A value of true indicates that the namespace should be created
// (i.e., the associated CLONE_NEW* flag will be applied).
type Config struct {
	mount   bool
	network bool
	uts     bool
	pid     bool
	ipc     bool
	user    bool
	cgroup  bool
	time    bool

	mountPath   string
	networkPath string
	utsPath     string
	pidPath     string
	ipcPath     string
	userPath    string
	cgroupPath  string
	timePath    string
}

func NewConfig(mount bool, network bool, uts bool, pid bool, ipc bool, user bool, cgroup bool, time bool, mountPath string, networkPath string, utsPath string, pidPath string, ipcPath string, userPath string, cgroupPath string, timePath string) Config {
	return Config{
		mount:       mount,
		network:     network,
		uts:         uts,
		pid:         pid,
		ipc:         ipc,
		user:        user,
		cgroup:      cgroup,
		time:        time,
		mountPath:   mountPath,
		networkPath: networkPath,
		utsPath:     utsPath,
		pidPath:     pidPath,
		ipcPath:     ipcPath,
		userPath:    userPath,
		cgroupPath:  cgroupPath,
		timePath:    timePath,
	}
}

func (c Config) Mount() bool         { return c.mount }
func (c Config) Network() bool       { return c.network }
func (c Config) UTS() bool           { return c.uts }
func (c Config) PID() bool           { return c.pid }
func (c Config) IPC() bool           { return c.ipc }
func (c Config) User() bool          { return c.user }
func (c Config) Cgroup() bool        { return c.cgroup }
func (c Config) Time() bool          { return c.time }
func (c Config) MountPath() string   { return c.mountPath }
func (c Config) NetworkPath() string { return c.networkPath }
func (c Config) UTSPath() string     { return c.utsPath }
func (c Config) PIDPath() string     { return c.pidPath }
func (c Config) IPCPath() string     { return c.ipcPath }
func (c Config) UserPath() string    { return c.userPath }
func (c Config) CgroupPath() string  { return c.cgroupPath }
func (c Config) TimePath() string    { return c.timePath }

// buildNamespaceConfig constructs a namespaceConfig from the namespaces
// defined in the OCI runtime-spec.
//
// The function inspects spec.LinuxSpec.Namespaces and marks each namespace
// as enabled in the returned namespaceConfig. If a namespace type is not
// present in the spec, the corresponding field remains false.
//
// This function does not perform any system calls; it simply derives the
// configuration that will later be used to construct SysProcAttr.
func BuildConfig(spec spec.Spec) Config {
	var nsConfig Config
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
		case "time":
			if ns.Path != "" {
				nsConfig.timePath = ns.Path
				nsConfig.time = false
				break
			}
			if nsConfig.timePath == "" {
				// Accept OCI time namespaces from Docker/containerd for compatibility.
				// Droplet does not create a separate time namespace yet.
				nsConfig.time = true
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
func BuildCloneFlags(nsConfig Config) uintptr {
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

type JoinTarget struct {
	name   string
	path   string
	nstype int
}

func (t JoinTarget) Name() string {
	return t.name
}

func (t JoinTarget) Path() string {
	return t.path
}

func (t JoinTarget) NSType() int {
	return t.nstype
}

// buildNamespaceJoinTargets returns the list of namespaces that should be joined
// via setns based on explicit Path entries in the spec. Only namespaces with
// Path set are included. Order is fixed to avoid surprises.
func BuildJoinTargets(spec spec.Spec) []JoinTarget {
	nsConfig := BuildConfig(spec)
	var targets []JoinTarget

	if nsConfig.mountPath != "" {
		targets = append(targets, JoinTarget{name: "mount", path: nsConfig.mountPath, nstype: unix.CLONE_NEWNS})
	}
	if nsConfig.cgroupPath != "" {
		targets = append(targets, JoinTarget{name: "cgroup", path: nsConfig.cgroupPath, nstype: unix.CLONE_NEWCGROUP})
	}
	if nsConfig.networkPath != "" {
		targets = append(targets, JoinTarget{name: "network", path: nsConfig.networkPath, nstype: unix.CLONE_NEWNET})
	}
	if nsConfig.ipcPath != "" {
		targets = append(targets, JoinTarget{name: "ipc", path: nsConfig.ipcPath, nstype: unix.CLONE_NEWIPC})
	}
	if nsConfig.utsPath != "" {
		targets = append(targets, JoinTarget{name: "uts", path: nsConfig.utsPath, nstype: unix.CLONE_NEWUTS})
	}

	return targets
}

func UserNamespacePath(spec spec.Spec) string {
	return BuildConfig(spec).userPath
}

func PathNsenterArgs(containerSpec spec.Spec) []string {
	nsConfig := BuildConfig(containerSpec)
	args := []string{}
	if nsConfig.cgroupPath != "" {
		args = append(args, "--cgroup="+nsConfig.cgroupPath)
	}
	if nsConfig.ipcPath != "" {
		args = append(args, "--ipc="+nsConfig.ipcPath)
	}
	if nsConfig.mountPath != "" {
		args = append(args, "--mount="+nsConfig.mountPath)
	}
	if nsConfig.networkPath != "" {
		args = append(args, "--net="+nsConfig.networkPath)
	}
	if nsConfig.pidPath != "" {
		args = append(args, "--pid="+ResolvePIDPath(nsConfig.pidPath))
	}
	if nsConfig.utsPath != "" {
		args = append(args, "--uts="+nsConfig.utsPath)
	}
	if nsConfig.userPath != "" {
		args = append(args, "--user="+nsConfig.userPath, "--setuid=0", "--setgid=0")
	}
	return args
}

func ResolvePIDPath(path string) string {
	if filepath.Base(path) != "pid_for_children" {
		return path
	}

	parentPID, ok := PIDFromProcPath(path)
	if !ok {
		return path
	}
	if childPID, ok := FirstChildPID(parentPID); ok {
		return fmt.Sprintf("/proc/%d/ns/pid", childPID)
	}
	if childPID, ok := ScanFirstChildPID(parentPID); ok {
		return fmt.Sprintf("/proc/%d/ns/pid", childPID)
	}
	return path
}

func PIDFromProcPath(path string) (int, bool) {
	clean := filepath.Clean(path)
	parts := strings.Split(clean, string(os.PathSeparator))
	if len(parts) != 5 || parts[1] != "proc" || parts[3] != "ns" {
		return 0, false
	}
	pid, err := strconv.Atoi(parts[2])
	if err != nil || pid <= 0 {
		return 0, false
	}
	return pid, true
}

func FirstChildPID(parentPID int) (int, bool) {
	childrenPath := fmt.Sprintf("/proc/%d/task/%d/children", parentPID, parentPID)
	data, err := os.ReadFile(childrenPath)
	if err != nil {
		return 0, false
	}
	for _, field := range strings.Fields(string(data)) {
		pid, err := strconv.Atoi(field)
		if err == nil && pid > 0 {
			return pid, true
		}
	}
	return 0, false
}

func ScanFirstChildPID(parentPID int) (int, bool) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0, false
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}
		stat, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "stat"))
		if err != nil {
			continue
		}
		if ProcStatPPID(string(stat)) == parentPID {
			return pid, true
		}
	}
	return 0, false
}

func ProcStatPPID(stat string) int {
	closeParen := strings.LastIndex(stat, ")")
	if closeParen < 0 || closeParen+2 >= len(stat) {
		return 0
	}
	fields := strings.Fields(stat[closeParen+2:])
	if len(fields) < 2 {
		return 0
	}
	ppid, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0
	}
	return ppid
}

// joinExistingNamespaces applies setns for non-user namespaces that specify
// Path in the OCI spec. User namespace joining is handled before exec through
// nsenter so the Go runtime does not call setns(CLONE_NEWUSER) after it has
// started multiple threads.
func JoinExisting(spec spec.Spec) error {
	targets := BuildJoinTargets(spec)
	for _, t := range targets {
		f, err := os.Open(t.path)
		if err != nil {
			return fmt.Errorf("open %s namespace: %w", t.name, err)
		}
		if err := unix.Setns(int(f.Fd()), t.nstype); err != nil {
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
func BuildRootUserNamespaceIDMap(nsConfig Config) (uidMap, gidMap []syscall.SysProcIDMap) {
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

func BuildSpecUserNamespaceIDMap(nsConfig Config, uidMappings []spec.IDMappingObject, gidMappings []spec.IDMappingObject) (uidMap, gidMap []syscall.SysProcIDMap) {
	if !nsConfig.user {
		return nil, nil
	}
	for _, mapping := range uidMappings {
		uidMap = append(uidMap, syscall.SysProcIDMap{
			ContainerID: mapping.ContainerID,
			HostID:      mapping.HostID,
			Size:        mapping.Size,
		})
	}
	for _, mapping := range gidMappings {
		gidMap = append(gidMap, syscall.SysProcIDMap{
			ContainerID: mapping.ContainerID,
			HostID:      mapping.HostID,
			Size:        mapping.Size,
		})
	}
	return uidMap, gidMap
}

func BuildRootlessUserNamespaceIDMap(nsConfig Config, rootlessConfig spec.RootlessConfigObject) (uidMap, gidMap []syscall.SysProcIDMap) {
	if !nsConfig.user {
		return nil, nil
	}

	policy := rootless.IDMapPolicyFromConfig(rootlessConfig)
	if policy.Mode() == spec.RootlessModeLoginRoot {
		uidMap = []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: policy.RootUID(), Size: 1},
			{ContainerID: 1, HostID: policy.UIDBase(), Size: policy.MapSize() - 1},
		}
		gidMap = []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: policy.RootGID(), Size: 1},
			{ContainerID: 1, HostID: policy.GIDBase(), Size: policy.MapSize() - 1},
		}
		return uidMap, gidMap
	}

	uidMap = []syscall.SysProcIDMap{
		{ContainerID: 0, HostID: policy.UIDBase(), Size: policy.MapSize()},
	}
	gidMap = []syscall.SysProcIDMap{
		{ContainerID: 0, HostID: policy.GIDBase(), Size: policy.MapSize()},
	}

	return uidMap, gidMap
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
