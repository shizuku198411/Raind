package container

import (
	"syscall"

	ns "raind/internal/droplet/container/namespace"
	"raind/internal/droplet/spec"
)

type procAttr struct {
	cloneFlags    uintptr
	uidMap        []syscall.SysProcIDMap
	gidMap        []syscall.SysProcIDMap
	setGroupsFlag bool
	credential    *syscall.Credential
}

type namespaceConfig struct {
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

type namespaceJoinTarget struct {
	name   string
	path   string
	nstype int
}

func buildSysProcAttr(procAttr procAttr) *syscall.SysProcAttr {
	return ns.BuildSysProcAttr(procAttr.toNamespace())
}

func buildProcAttrForRootContainer(nsConfig namespaceConfig) procAttr {
	return procAttrFromNamespace(ns.BuildProcAttrForRootContainer(nsConfig.toNamespace()))
}

func buildProcAttrForContainer(containerSpec spec.Spec) procAttr {
	return procAttrFromNamespace(ns.BuildProcAttrForContainer(containerSpec))
}

func buildNamespaceConfig(spec spec.Spec) namespaceConfig {
	return namespaceConfigFromNamespace(ns.BuildConfig(spec))
}

func buildCloneFlags(nsConfig namespaceConfig) uintptr {
	return ns.BuildCloneFlags(nsConfig.toNamespace())
}

func buildNamespaceJoinTargets(spec spec.Spec) []namespaceJoinTarget {
	targets := ns.BuildJoinTargets(spec)
	out := make([]namespaceJoinTarget, 0, len(targets))
	for _, target := range targets {
		out = append(out, namespaceJoinTarget{
			name:   target.Name(),
			path:   target.Path(),
			nstype: target.NSType(),
		})
	}
	return out
}

func userNamespacePath(spec spec.Spec) string {
	return ns.UserNamespacePath(spec)
}

func namespacePathNsenterArgs(containerSpec spec.Spec) []string {
	return ns.PathNsenterArgs(containerSpec)
}

func resolvePIDNamespacePath(path string) string {
	return ns.ResolvePIDPath(path)
}

func pidFromProcNamespacePath(path string) (int, bool) {
	return ns.PIDFromProcPath(path)
}

func firstChildPID(parentPID int) (int, bool) {
	return ns.FirstChildPID(parentPID)
}

func scanFirstChildPID(parentPID int) (int, bool) {
	return ns.ScanFirstChildPID(parentPID)
}

func procStatPPID(stat string) int {
	return ns.ProcStatPPID(stat)
}

func joinExistingNamespaces(spec spec.Spec) error {
	return ns.JoinExisting(spec)
}

func buildRootUserNamespaceIDMap(nsConfig namespaceConfig) (uidMap, gidMap []syscall.SysProcIDMap) {
	return ns.BuildRootUserNamespaceIDMap(nsConfig.toNamespace())
}

func buildSpecUserNamespaceIDMap(nsConfig namespaceConfig, uidMappings []spec.IDMappingObject, gidMappings []spec.IDMappingObject) (uidMap, gidMap []syscall.SysProcIDMap) {
	return ns.BuildSpecUserNamespaceIDMap(nsConfig.toNamespace(), uidMappings, gidMappings)
}

func buildRootlessUserNamespaceIDMap(nsConfig namespaceConfig, rootlessConfig spec.RootlessConfigObject) (uidMap, gidMap []syscall.SysProcIDMap) {
	return ns.BuildRootlessUserNamespaceIDMap(nsConfig.toNamespace(), rootlessConfig)
}

func (p procAttr) toNamespace() ns.ProcAttr {
	return ns.NewProcAttr(p.cloneFlags, p.uidMap, p.gidMap, p.setGroupsFlag, p.credential)
}

func procAttrFromNamespace(p ns.ProcAttr) procAttr {
	return procAttr{
		cloneFlags:    p.CloneFlags(),
		uidMap:        p.UIDMap(),
		gidMap:        p.GIDMap(),
		setGroupsFlag: p.SetGroupsFlag(),
		credential:    p.Credential(),
	}
}

func (c namespaceConfig) toNamespace() ns.Config {
	return ns.NewConfig(
		c.mount,
		c.network,
		c.uts,
		c.pid,
		c.ipc,
		c.user,
		c.cgroup,
		c.time,
		c.mountPath,
		c.networkPath,
		c.utsPath,
		c.pidPath,
		c.ipcPath,
		c.userPath,
		c.cgroupPath,
		c.timePath,
	)
}

func namespaceConfigFromNamespace(c ns.Config) namespaceConfig {
	return namespaceConfig{
		mount:       c.Mount(),
		network:     c.Network(),
		uts:         c.UTS(),
		pid:         c.PID(),
		ipc:         c.IPC(),
		user:        c.User(),
		cgroup:      c.Cgroup(),
		time:        c.Time(),
		mountPath:   c.MountPath(),
		networkPath: c.NetworkPath(),
		utsPath:     c.UTSPath(),
		pidPath:     c.PIDPath(),
		ipcPath:     c.IPCPath(),
		userPath:    c.UserPath(),
		cgroupPath:  c.CgroupPath(),
		timePath:    c.TimePath(),
	}
}
