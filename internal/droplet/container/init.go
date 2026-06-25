package container

import (
	"fmt"
	"os"
	"path/filepath"
	"raind/internal/droplet/logs"
	"raind/internal/droplet/spec"
	"raind/internal/droplet/status"
	"raind/internal/droplet/utils"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/syndtr/gocapability/capability"
	"golang.org/x/sys/unix"
)

// NewContainerInit returns a ContainerInit wired with the default
// implementations of its dependencies (fifoReader and processReplacer).
// This is the standard entry point for executing the container init phase.
func NewContainerInit() *ContainerInit {
	return &ContainerInit{
		fifoReader:           newContainerFifoHandler(),
		specLoader:           newFileSpecLoader(),
		containerEnvPreparer: newRootContainerEnvPrepare(),
		syscallHandler:       utils.NewSyscallHandler(),
		appArmorHandler:      NewAppArmorManager(),
	}
}

// newRootContainerEnvPreparer returns the default environment preparer
// implementation for containers started as root on the host.
//
// This preparer performs setup steps that assume the runtime is executing
// with full privileges (e.g., user-namespace root switching, hostname
// configuration). A separate implementation can be provided for rootless
// execution environments.
func newRootContainerEnvPrepare() *rootContainerEnvPreparer {
	return &rootContainerEnvPreparer{
		syscallHandler: utils.NewSyscallHandler(),
		seccompHandler: NewSeccompManager(),
	}
}

// ContainerInit represents the runtime logic executed inside the
// container's init process.
//
// The init process waits for a start signal via FIFO and then
// replaces itself with the container entrypoint using execve-style
// semantics (syscall.Exec).
type ContainerInit struct {
	fifoReader           fifoReader
	specLoader           specLoader
	containerEnvPreparer containerEnvPreparer
	syscallHandler       utils.SyscallHandler
	appArmorHandler      AppArmorHandler
}

// Execute performs the init sequence for the container.
//
// The sequence is:
//
//  1. Wait for a start signal by reading from the FIFO path
//  2. Replace the current process image with the container entrypoint
//
// On success, this function does not return because the process image
// is replaced. Errors are returned only if the FIFO read fails or
// syscall.Exec cannot be invoked.
func (c *ContainerInit) Execute(opt InitOption) (err error) {
	// lock GO thread
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	var (
		spec  spec.Spec
		event = "init"
		stage string
	)

	// audit log
	defer func() {
		result := "success"
		if err != nil {
			result = "fail"
		}
		_ = logs.RecordAuditLog(logs.AuditRecord{
			ContainerId: opt.ContainerId,
			Event:       event,
			Stage:       stage,
			Spec:        &spec,
			Result:      result,
			Error:       err,
		})
	}()

	fifo := opt.Fifo
	entrypoint := opt.Entrypoint

	// 1. load config.json
	stage = "load_spec"
	spec, err = c.specSecureLoad(opt.ContainerId)
	if err != nil {
		return err
	}

	// 2. read fifo for waiting start signal
	stage = "read_fifo"
	err = c.fifoReader.readFifo(fifo)
	if err != nil {
		return err
	}

	// 3. prepare container environment
	stage = "prepare"
	err = c.containerEnvPreparer.prepare(opt.ContainerId, spec)
	if err != nil {
		return err
	}

	// 4. apply AppArmor Profile Onexec
	stage = "apply_apparmor"
	err = c.appArmorHandler.ApplyAAProfileOnExec(spec.LinuxSpec.AppArmorProfile)
	if err != nil {
		return err
	}

	// 5. replace process with the container entrypoint
	stage = "exec_entrypoint"
	// lookup entrypoint[0]'s abstract path
	arg0, err := c.lookEntrypointPath(entrypoint[0], spec.Process.Env)
	if err != nil {
		return err
	}
	entrypoint[0] = arg0
	// close all FD except 0,1,2
	c.closeAllExcept012()
	// execve
	err = c.syscallHandler.Exec(arg0, entrypoint, spec.Process.Env)
	if err != nil {
		return err
	}

	return nil
}

func (c *ContainerInit) closeAllExcept012() {
	ents, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return
	}
	for _, e := range ents {
		fd, err := strconv.Atoi(e.Name())
		if err != nil || fd < 3 {
			continue
		}
		_ = syscall.Close(fd)
	}
}

func (c *ContainerInit) specSecureLoad(containerId string) (spec.Spec, error) {
	fileHashPath := utils.ConfigFileHashPath(containerId)

	// 1. load hash string
	var specFileHash spec.SpecHash
	if err := utils.ReadJsonFile(
		fileHashPath,
		&specFileHash,
	); err != nil {
		return spec.Spec{}, err
	}

	// 2. calculate current config.json file hash
	currentHash, err := utils.Sha256File(utils.ConfigFilePath(containerId))
	if err != nil {
		return spec.Spec{}, err
	}

	// 3. assert
	if specFileHash.Sha256 != currentHash {
		return spec.Spec{}, fmt.Errorf("config.json hash validation failed: expect=%s, got=%s", specFileHash.Sha256, currentHash)
	}

	// 4. load config.json
	specFile, err := c.specLoader.loadFile(containerId)
	if err != nil {
		return spec.Spec{}, err
	}

	// 5. remove hash file
	if err := os.Remove(fileHashPath); err != nil {
		if !shouldIgnoreSpecHashRemoveError(specFile, err) {
			return spec.Spec{}, err
		}
	}

	return specFile, nil
}

func shouldIgnoreSpecHashRemoveError(containerSpec spec.Spec, err error) bool {
	return err != nil && os.IsPermission(err) && isRootlessSpec(containerSpec)
}

func (c *ContainerInit) lookEntrypointPath(arg0 string, env []string) (string, error) {
	// if arg0 has "/", it already abstract path
	if strings.Contains(arg0, "/") {
		return arg0, nil
	}
	// set PATH value
	var pathVal string
	for _, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			pathVal = strings.TrimPrefix(e, "PATH=")
			break
		}
	}
	if pathVal == "" {
		pathVal = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	}

	for _, dir := range strings.Split(pathVal, ":") {
		if dir == "" {
			dir = "."
		}
		cand := filepath.Join(dir, arg0)
		if err := unix.Access(cand, unix.X_OK); err == nil {
			return cand, nil
		}
	}
	return "", fmt.Errorf("%s: not found in PATH", arg0)
}

// containerEnvPreparer defines the behavior for preparing the container
// environment inside the init process.
//
// Implementations of this interface are responsible for performing
// container-local setup steps such as user namespace UID/GID switching,
// hostname configuration, filesystem setup, and other initialization logic
// that must occur before the container entrypoint is executed.
type containerEnvPreparer interface {
	prepare(containerId string, spec spec.Spec) error
}

// rootContainerEnvPreparer is the default envPreparer implementation used
// for privileged (root-executed) containers.
//
// It performs environment initialization tasks inside the init process,
// such as switching to UID/GID 0 within the user namespace and configuring
// the UTS namespace hostname. Additional setup steps (mounts, pivot_root,
// capability adjustments, etc.) may be added to this implementation as
// container initialization evolves.
type rootContainerEnvPreparer struct {
	syscallHandler utils.KernelSyscallHandler
	seccompHandler SeccompHandler
}

// prepare sets up the runtime environment for the root container process
// according to the provided OCI spec.
//
// The workflow is:
//  1. Switch to uid=0 (root) inside the user namespace
//  2. Set the hostname to the container ID from the spec
//  3. Set up the overlay filesystem based on rootfs and image annotations
//  4. Mount the configured filesystems
//  5. Mount standard device files under the new root
//  6. Create required symbolic links under the new root
//  7. Perform pivot_root into the container root filesystem
//  8. Configure Linux capabilities for the process
//
// If any step fails, the error is returned immediately and the remaining
// steps are not executed.
func (p *rootContainerEnvPreparer) prepare(containerId string, spec spec.Spec) (err error) {
	// 0. join existing namespaces (net/ipc/uts) if specified in spec
	prejoinedNamespaces := os.Getenv(raindNamespacesPrejoinedEnv) == "1"
	if !prejoinedNamespaces {
		err = joinExistingNamespaces(spec)
		if err != nil {
			return fmt.Errorf("join namespaces: %w", err)
		}
	}
	// 1. change uid=0(root) inside container
	err = p.switchToUserNamespaceRoot()
	if err != nil {
		return fmt.Errorf("switch to user namespace root: %w", err)
	}
	// 2. set hostname
	if !prejoinedNamespaces {
		err = p.setHostnameToContainerId(spec.Hostname)
		if err != nil {
			return fmt.Errorf("set hostname: %w", err)
		}
	}
	// 3. set env
	err = p.setEnv(spec.Process.Env)
	if err != nil {
		return fmt.Errorf("set env: %w", err)
	}
	// 4. make mount propagation private. Plain OCI rootfs bundles do not go
	// through setupOverlay, but pivot_root still requires propagation isolation.
	if err := p.makeMountsPrivate(); err != nil {
		return fmt.Errorf("make mounts private: %w", err)
	}
	// 5. setup overlay
	if err := p.setupOverlay(spec.Root.Path, spec.Annotations.Image); err != nil {
		return fmt.Errorf("setup overlay: %w", err)
	}
	// 6. mount filesystem
	ociBundleMode := isOCIBundleMode(containerId)
	err = p.mountFilesystem(containerId, spec.Root.Path, spec.Mounts, ociBundleMode)
	if err != nil {
		return fmt.Errorf("mount filesystem: %w", err)
	}
	// 7. mount standard device
	err = p.mountStdDevice(spec.Root.Path)
	if err != nil {
		return fmt.Errorf("mount std device: %w", err)
	}
	// 8. create OCI linux devices
	err = p.createLinuxDevices(spec.Root.Path, spec.LinuxSpec.Devices)
	if err != nil {
		return fmt.Errorf("create linux devices: %w", err)
	}
	// 9. create symbolic link
	err = p.createSymbolicLink(spec.Root.Path)
	if err != nil {
		return fmt.Errorf("create symbolic link: %w", err)
	}
	// 10. apply masked and readonly paths
	err = p.applyMaskedPaths(spec.Root.Path, spec.LinuxSpec.MaskedPaths)
	if err != nil {
		return fmt.Errorf("apply masked paths: %w", err)
	}
	err = p.applyReadonlyPaths(spec.Root.Path, spec.LinuxSpec.ReadonlyPaths)
	if err != nil {
		return fmt.Errorf("apply readonly paths: %w", err)
	}
	// 11. ensure rootfs is a mount point for pivot_root. Plain OCI rootfs
	// bundles do not go through the Raind overlay mount path, so bind-mount the
	// rootfs onto itself before pivoting.
	err = p.ensureRootfsMountpoint(spec.Root.Path)
	if err != nil {
		return fmt.Errorf("ensure rootfs mountpoint: %w", err)
	}
	// 12. pivot_root
	err = p.pivotRoot(spec.Root.Path)
	if err != nil {
		return fmt.Errorf("pivot root: %w", err)
	}
	// 13. apply readonly rootfs after pivot_root so the runtime can create the
	// temporary put_old directory first.
	if spec.Root.Readonly {
		err = p.makeCurrentRootReadonly()
		if err != nil {
			return fmt.Errorf("make rootfs readonly: %w", err)
		}
	}
	// 14. apply no_new_privileges before seccomp so the observed process state
	// matches process.noNewPrivileges from the OCI spec.
	if spec.Process.NoNewPrivileges {
		err = p.setNoNewPrivileges()
		if err != nil {
			return fmt.Errorf("set no_new_privileges: %w", err)
		}
	}
	// 15. install seccomp before dropping capabilities. Privileged runtimes can
	// load the filter without forcing no_new_privileges when the spec leaves it false.
	if spec.LinuxSpec.Seccomp != nil {
		err = p.seccompHandler.InstallDenyFilter(*spec.LinuxSpec.Seccomp)
		if err != nil {
			return fmt.Errorf("install seccomp: %w", err)
		}
	}
	// 16. set capability
	err = p.setCapability(spec.Process.Capabilities)
	if err != nil {
		return fmt.Errorf("set capability: %w", err)
	}
	// 17. set OOM score adjustment
	err = p.setOOMScoreAdj(spec.Process.OOMScoreAdj)
	if err != nil {
		return fmt.Errorf("set oom_score_adj: %w", err)
	}
	// 18. set rlimits
	err = p.setRlimits(spec.Process.Rlimits)
	if err != nil {
		return fmt.Errorf("set rlimits: %w", err)
	}
	// 19. set process user
	err = p.setProcessUser(spec.Process.User)
	if err != nil {
		return fmt.Errorf("set process user: %w", err)
	}
	// 20. change current dir
	err = p.syscallHandler.Chdir(spec.Process.Cwd)
	if err != nil {
		return fmt.Errorf("chdir: %w", err)
	}

	return nil
}

func (p *rootContainerEnvPreparer) makeMountsPrivate() error {
	return p.syscallHandler.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, "")
}

func (p *rootContainerEnvPreparer) ensureRootfsMountpoint(rootfs string) error {
	return p.syscallHandler.Mount(rootfs, rootfs, "", syscall.MS_BIND|syscall.MS_REC, "")
}

func (p *rootContainerEnvPreparer) makeCurrentRootReadonly() error {
	return p.syscallHandler.Mount("", "/", "", syscall.MS_BIND|syscall.MS_REMOUNT|syscall.MS_RDONLY|syscall.MS_REC, "")
}

func isOCIBundleMode(containerId string) bool {
	var state status.StatusObject
	if err := utils.ReadJsonFile(utils.ContainerStatePath(containerId), &state); err != nil {
		return false
	}
	return state.Bundle != "" && filepath.Clean(state.Bundle) != filepath.Clean(utils.ContainerDir(containerId))
}

func (p *rootContainerEnvPreparer) createLinuxDevices(rootfs string, devices []spec.DeviceObject) error {
	for _, device := range devices {
		target, err := securePath(rootfs, device.Path)
		if err != nil {
			return err
		}
		if err := p.syscallHandler.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		mode, err := deviceMode(device)
		if err != nil {
			return err
		}
		dev := 0
		if device.Type == "c" || device.Type == "u" || device.Type == "b" {
			dev = int(unix.Mkdev(uint32(device.Major), uint32(device.Minor)))
		}
		if err := p.syscallHandler.Mknod(target, mode, dev); err != nil {
			return err
		}
		if device.UID != nil || device.GID != nil {
			uid := -1
			gid := -1
			if device.UID != nil {
				uid = int(*device.UID)
			}
			if device.GID != nil {
				gid = int(*device.GID)
			}
			if err := p.syscallHandler.Chown(target, uid, gid); err != nil {
				return err
			}
		}
		if device.FileMode != nil {
			if err := p.syscallHandler.Chmod(target, os.FileMode(*device.FileMode)); err != nil {
				return err
			}
		}
	}
	return nil
}

func deviceMode(device spec.DeviceObject) (uint32, error) {
	perm := uint32(0666)
	if device.FileMode != nil {
		perm = *device.FileMode
	}
	switch device.Type {
	case "c", "u":
		return syscall.S_IFCHR | perm, nil
	case "b":
		return syscall.S_IFBLK | perm, nil
	case "p":
		return syscall.S_IFIFO | perm, nil
	default:
		return 0, fmt.Errorf("unsupported linux device type: %s", device.Type)
	}
}

func (p *rootContainerEnvPreparer) applyMaskedPaths(rootfs string, paths []string) error {
	for _, path := range paths {
		target, err := securePath(rootfs, path)
		if err != nil {
			return err
		}
		info, statErr := p.syscallHandler.Stat(target)
		if statErr == nil && info.IsDir() {
			if err := p.syscallHandler.Mount("tmpfs", target, "tmpfs", syscall.MS_NOSUID|syscall.MS_NODEV|syscall.MS_NOEXEC, ""); err != nil {
				return err
			}
			if err := p.syscallHandler.Mount("", target, "", syscall.MS_BIND|syscall.MS_REMOUNT|syscall.MS_RDONLY|syscall.MS_NOSUID|syscall.MS_NODEV|syscall.MS_NOEXEC, ""); err != nil {
				return err
			}
			continue
		}
		if statErr != nil && !p.syscallHandler.IsNotExist(statErr) {
			return statErr
		}
		if err := p.syscallHandler.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if statErr != nil {
			f, err := p.syscallHandler.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
			if err != nil {
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		}
		if err := p.syscallHandler.Mount("/dev/null", target, "", syscall.MS_BIND, ""); err != nil {
			return err
		}
		if err := p.syscallHandler.Mount("", target, "", syscall.MS_BIND|syscall.MS_REMOUNT|syscall.MS_RDONLY|syscall.MS_NOSUID|syscall.MS_NODEV|syscall.MS_NOEXEC, ""); err != nil {
			return err
		}
	}
	return nil
}

func (p *rootContainerEnvPreparer) applyReadonlyPaths(rootfs string, paths []string) error {
	for _, path := range paths {
		target, err := securePath(rootfs, path)
		if err != nil {
			return err
		}
		if _, err := p.syscallHandler.Stat(target); err != nil {
			if p.syscallHandler.IsNotExist(err) {
				continue
			}
			return err
		}
		if err := p.syscallHandler.Mount(target, target, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
			return err
		}
		if err := p.syscallHandler.Mount("", target, "", syscall.MS_BIND|syscall.MS_REMOUNT|syscall.MS_RDONLY|syscall.MS_REC, ""); err != nil {
			return err
		}
	}
	return nil
}

func (p *rootContainerEnvPreparer) setNoNewPrivileges() error {
	return unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0)
}

func (p *rootContainerEnvPreparer) setOOMScoreAdj(value *int) error {
	if value == nil {
		return nil
	}
	if *value < -1000 || *value > 1000 {
		return fmt.Errorf("oomScoreAdj must be between -1000 and 1000")
	}
	return p.syscallHandler.WriteFile("/proc/self/oom_score_adj", []byte(strconv.Itoa(*value)), 0644)
}

func (p *rootContainerEnvPreparer) setRlimits(rlimits []spec.RlimitObject) error {
	for _, item := range rlimits {
		resource, ok := rlimitResource(item.Type)
		if !ok {
			return fmt.Errorf("unsupported rlimit type: %s", item.Type)
		}
		limit := syscall.Rlimit{Cur: item.Soft, Max: item.Hard}
		if err := p.syscallHandler.Setrlimit(resource, &limit); err != nil {
			return err
		}
	}
	return nil
}

func rlimitResource(name string) (int, bool) {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "RLIMIT_AS":
		return unix.RLIMIT_AS, true
	case "RLIMIT_CORE":
		return unix.RLIMIT_CORE, true
	case "RLIMIT_CPU":
		return unix.RLIMIT_CPU, true
	case "RLIMIT_DATA":
		return unix.RLIMIT_DATA, true
	case "RLIMIT_FSIZE":
		return unix.RLIMIT_FSIZE, true
	case "RLIMIT_LOCKS":
		return unix.RLIMIT_LOCKS, true
	case "RLIMIT_MEMLOCK":
		return unix.RLIMIT_MEMLOCK, true
	case "RLIMIT_MSGQUEUE":
		return unix.RLIMIT_MSGQUEUE, true
	case "RLIMIT_NICE":
		return unix.RLIMIT_NICE, true
	case "RLIMIT_NOFILE":
		return unix.RLIMIT_NOFILE, true
	case "RLIMIT_NPROC":
		return unix.RLIMIT_NPROC, true
	case "RLIMIT_RSS":
		return unix.RLIMIT_RSS, true
	case "RLIMIT_RTPRIO":
		return unix.RLIMIT_RTPRIO, true
	case "RLIMIT_RTTIME":
		return unix.RLIMIT_RTTIME, true
	case "RLIMIT_SIGPENDING":
		return unix.RLIMIT_SIGPENDING, true
	case "RLIMIT_STACK":
		return unix.RLIMIT_STACK, true
	default:
		return 0, false
	}
}

func (p *rootContainerEnvPreparer) setProcessUser(user spec.UserObject) error {
	if len(user.AdditionalGids) > 0 {
		if err := p.syscallHandler.Setgroups(user.AdditionalGids); err != nil {
			return err
		}
	}
	if err := p.syscallHandler.Setresgid(user.GID, user.GID, user.GID); err != nil {
		return err
	}
	if err := p.syscallHandler.Setresuid(user.UID, user.UID, user.UID); err != nil {
		return err
	}
	if user.Umask != nil {
		syscall.Umask(*user.Umask)
	}
	return nil
}

// switchToUserNamespaceRoot switches the current process credentials to
// UID and GID 0 within the active user namespace.
//
// This ensures that subsequent privileged operations (such as mount,
// pivot_root, or hostname changes) execute with the required namespace-
// scoped capabilities, even when the process was not initially running as
// namespace-root.
func (p *rootContainerEnvPreparer) switchToUserNamespaceRoot() error {
	// switch root group (gid=0)
	if err := p.syscallHandler.Setresgid(0, 0, 0); err != nil {
		return err
	}
	// switch root user (uid=0)
	if err := p.syscallHandler.Setresuid(0, 0, 0); err != nil {
		return err
	}
	return nil
}

// setHostnameToContainerId configures the hostname for the process inside
// the UTS namespace.
//
// The hostname value is typically derived from the container ID or the
// OCI spec. An error is returned if the syscall fails or the namespace
// does not permit hostname updates.
func (p *rootContainerEnvPreparer) setHostnameToContainerId(hostname string) error {
	if err := p.syscallHandler.Sethostname([]byte(hostname)); err != nil {
		return err
	}
	return nil
}

func (p *rootContainerEnvPreparer) setEnv(envlist []string) error {
	for _, e := range envlist {
		envParts := strings.SplitN(e, "=", 2)
		k, v := envParts[0], envParts[1]
		if err := p.syscallHandler.Setenv(k, v); err != nil {
			return err
		}
	}
	return nil
}

// setupOverlay mounts the container root filesystem using overlayfs.
//
// imageAnnotation is a JSON string that is decoded into ImageConfigObject,
// which contains lower (image layers), upper, and work directories.
// The overlay filesystem is mounted at the given rootfs path.
func (p *rootContainerEnvPreparer) setupOverlay(rootfs string, imageAnnotation string) error {
	if strings.TrimSpace(imageAnnotation) == "" {
		return nil
	}

	// convert string to json
	var imageConfig spec.ImageConfigObject
	if err := utils.StringToJson(imageAnnotation, &imageConfig); err != nil {
		return err
	}

	// pivot_root requires mount propagation to be private in container mount
	// namespaces. Keep this before mounting the overlay so the whole namespace
	// is isolated from the parent mount tree.
	if err := p.syscallHandler.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("make root private failed: target=%q flags=0x%x: %w", "/", syscall.MS_PRIVATE|syscall.MS_REC, err)
	}

	// mount parameter
	mountSource := "overlay"
	mountTarget := rootfs
	mountFstype := imageConfig.RootfsType
	mountFlags := uintptr(0)
	// mount data contains following parameter
	// - lowerdir : container image layers
	// - upperdir : directory for storing differences with lowerdir
	// - workdir  : directory for working directory
	lowerDir := strings.Join(imageConfig.ImageLayer, ":")
	upperDir := imageConfig.UpperDir
	workDir := imageConfig.WorkDir

	// The runtime later bind-mounts managed files such as /etc/hosts over
	// the container rootfs. Some images do not contain these files in the
	// lower layers. Once the overlay is mounted, creating missing targets
	// under the merged root can fail for rootless containers when the merged
	// view is read-only from the namespace perspective. Seed the targets in
	// the overlay upperdir before mounting so the bind destinations already
	// exist in the merged rootfs.
	if err := p.ensureOverlayManagedFileTargets(upperDir); err != nil {
		return fmt.Errorf("prepare overlay managed file targets: %w", err)
	}

	mountData := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lowerDir, upperDir, workDir)

	// overlay
	if err := p.syscallHandler.Mount(mountSource, mountTarget, mountFstype, mountFlags, mountData); err != nil {
		return fmt.Errorf("mount overlay failed: source=%q target=%q fstype=%q flags=0x%x data=%q: %w", mountSource, mountTarget, mountFstype, mountFlags, mountData, err)
	}

	// re-mount for mount propagation
	if err := p.syscallHandler.Mount("", mountTarget, "", syscall.MS_PRIVATE|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("mount overlay propagation failed: target=%q flags=0x%x: %w", mountTarget, syscall.MS_PRIVATE|syscall.MS_REC, err)
	}

	return nil
}

func (p *rootContainerEnvPreparer) ensureOverlayManagedFileTargets(upperDir string) error {
	for _, destination := range []string{"/etc/resolv.conf", "/etc/hostname", "/etc/hosts"} {
		target, err := securePath(upperDir, destination)
		if err != nil {
			return err
		}

		if err := p.syscallHandler.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return fmt.Errorf("create managed target parent %q: %w", filepath.Dir(target), err)
		}

		if _, err := p.syscallHandler.Stat(target); err == nil {
			continue
		} else if !p.syscallHandler.IsNotExist(err) {
			return fmt.Errorf("stat managed target %q: %w", target, err)
		}

		f, err := p.syscallHandler.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("create managed target %q: %w", target, err)
		}
		if err := f.Close(); err != nil {
			return fmt.Errorf("close managed target %q: %w", target, err)
		}
	}
	return nil
}

// mountFilesystem mounts all filesystems required for the container runtime
// as well as user-specified bind mounts.
//
// The mountList contains entries such as /proc, /dev, /sys, cgroup, tmpfs,
// and arbitrary host paths. For bind mounts, this method prepares the
// destination path depending on whether the source is a file or a directory.
func (p *rootContainerEnvPreparer) mountFilesystem(containerId string, rootfs string, mountList []spec.MountObject, ociBundleMode bool) error {
	prerequiredMounts := baseMountsForMode(containerId, ociBundleMode)

	if !ociBundleMode {
		if os.Getenv(raindNamespacesPrejoinedEnv) == "1" {
			prerequiredMounts = filterRootlessPrejoinedMounts(prerequiredMounts)
		}
		prerequiredMounts = p.filterMissingManagedFileMounts(prerequiredMounts)
	}

	// user mounts
	for _, user_mount := range mountList {
		if err := validateUserMount(user_mount); err != nil {
			return err
		}
		prerequiredMounts = append(prerequiredMounts, spec.MountObject{
			Destination: user_mount.Destination,
			Type:        user_mount.Type,
			Source:      user_mount.Source,
			Options:     user_mount.Options,
		})
	}

	for _, mountConfig := range prerequiredMounts {
		var (
			mountFlags      uintptr
			mountData       string
			dataStrTmp      []string
			bindFlag        = false
			deviceMountFlag = false
		)
		if mountConfig.Options != nil {
			for _, option := range mountConfig.Options {
				switch option {
				case "nosuid":
					mountFlags |= syscall.MS_NOSUID
				case "noexec":
					mountFlags |= syscall.MS_NOEXEC
				case "nodev":
					mountFlags |= syscall.MS_NODEV
				case "dev":
					deviceMountFlag = true
				case "ro":
					mountFlags |= syscall.MS_RDONLY
				case "rw":
					// ignore
				case "bind":
					bindFlag = true
					mountFlags |= syscall.MS_BIND
				case "strictatime":
					mountFlags |= syscall.MS_STRICTATIME
				case "noatime":
					mountFlags |= syscall.MS_NOATIME
				case "relatime":
					mountFlags |= syscall.MS_RELATIME
				case "rbind":
					bindFlag = true
					mountFlags |= syscall.MS_BIND | syscall.MS_REC
				case "rprivate", "private", "z", "Z":
					// ignore
				default:
					dataStrTmp = append(dataStrTmp, option)
				}
			}
			mountData = strings.Join(dataStrTmp, ",")
		} else {
			mountFlags = uintptr(0)
		}
		// If type is explicitly bind, ensure bind flags are set even when options omit bind/rbind.
		if mountConfig.Type == "bind" && !bindFlag {
			bindFlag = true
			mountFlags |= syscall.MS_BIND
		}
		// If type is empty but source is an absolute path, treat as bind mount by default.
		if mountConfig.Type == "" && !bindFlag && strings.HasPrefix(mountConfig.Source, string(os.PathSeparator)) {
			bindFlag = true
			mountFlags |= syscall.MS_BIND
		}

		// validate destination path
		mountPath, err := securePath(rootfs, mountConfig.Destination)
		if err != nil {
			return err
		}

		// the process differs depending on whether the source to be mounted is a directory or a file.
		// if the source is a directory, the destination directory is checked for existence and created if it does not exist.
		// if the source is a file, the parent directory is created and an empty file is created.
		// this process is only bind mount
		if bindFlag {
			// validate source type
			// if source typ is symlink, then deny
			isLink, err := isSymlink(mountConfig.Source)
			if err != nil {
				return fmt.Errorf("lstat failed: %s: %w", mountConfig.Source, err)
			}
			if isLink {
				return fmt.Errorf("source:%s is symlink", mountConfig.Source)
			}

			// retrieve source info
			srcInfo, statErr := p.syscallHandler.Stat(mountConfig.Source)
			if statErr != nil {
				return statErr
			}

			if srcInfo.IsDir() { // source: directory
				// reject if any symlink exists under source directory tree
				//if err := rejectSymlinkInDirTreeFd(mountConfig.Source, WalkLimits{MaxDepth: 64, MaxEntries: 200_000}); err != nil {
				//	return fmt.Errorf("invalid mount source (symlink in tree): %s: %w", mountConfig.Source, err)
				//}
				// check if target directory is exists
				if _, err := p.syscallHandler.Stat(mountPath); p.syscallHandler.IsNotExist(err) {
					if err := p.syscallHandler.MkdirAll(mountPath, os.ModePerm); err != nil {
						return err
					}
				}
			} else { // source: file
				// create parent directory if not exists
				if err := p.syscallHandler.MkdirAll(filepath.Dir(mountPath), os.ModePerm); err != nil {
					return err
				}
				if _, err := p.syscallHandler.Stat(mountPath); p.syscallHandler.IsNotExist(err) {
					f, err := p.syscallHandler.OpenFile(mountPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
					if err != nil {
						return err
					}
					f.Close()
				}
			}
		} else {
			// check if target directory is exists
			if _, err := p.syscallHandler.Stat(mountPath); p.syscallHandler.IsNotExist(err) {
				if err := p.syscallHandler.MkdirAll(mountPath, os.ModePerm); err != nil {
					return err
				}
			}
		}

		// mount
		if err := secureMount(
			mountConfig.Source,
			mountPath,
			mountConfig.Type,
			mountFlags,
			mountData,
			deviceMountFlag,
		); err != nil {
			return fmt.Errorf("mount fs failed: source=%q target=%q fstype=%q flags=0x%x data=%q: %w", mountConfig.Source, mountPath, mountConfig.Type, mountFlags, mountData, err)
		}
	}

	return nil
}

func baseMountsForMode(containerId string, ociBundleMode bool) []spec.MountObject {
	if ociBundleMode {
		return nil
	}
	return defaultContainerMounts(containerId)
}

func defaultContainerMounts(containerId string) []spec.MountObject {
	// mount file system required for operation. required fs is the following
	//   /proc, /dev, /dev/pts, /sys, /sys/fs/cgroup, /dev/mqueue, /dev/shm
	// additionally, mount user-specified host directories
	return []spec.MountObject{
		{
			Destination: "/proc",
			Type:        "proc",
			Source:      "proc",
			Options: []string{
				"nosuid",
				"noexec",
				"nodev",
			},
		},
		{
			Destination: "/dev",
			Type:        "tmpfs",
			Source:      "tmpfs",
			Options: []string{
				"nosuid",
				"strictatime",
				"mode=755",
				"size=65536k",
			},
		},
		{
			Destination: "/dev/pts",
			Type:        "devpts",
			Source:      "devpts",
			Options: []string{
				"nosuid",
				"noexec",
				"newinstance",
				"ptmxmode=0666",
				"mode=0620",
				"gid=5",
			},
		},
		{
			Destination: "/sys",
			Type:        "sysfs",
			Source:      "sysfs",
			Options: []string{
				"nosuid",
				"noexec",
				"nodev",
				"ro",
			},
		},
		{
			Destination: "/tmp",
			Type:        "tmpfs",
			Source:      "tmpfs",
			Options: []string{
				"nosuid",
				"nodev",
				"mode=1777",
				"size=512m",
			},
		},
		{
			Destination: "/run",
			Type:        "tmpfs",
			Source:      "tmpfs",
			Options: []string{
				"nosuid",
				"nodev",
				"mode=755",
				"size=65536k",
			},
		},
		{
			Destination: "/proc/sys",
			Type:        "tmpfs",
			Source:      "tmpfs",
			Options: []string{
				"nosuid",
				"noexec",
				"nodev",
				"mode=0555",
				"size=0",
			},
		},
		{
			Destination: "/proc/sysrq-trigger",
			Type:        "bind",
			Source:      "/dev/null",
			Options: []string{
				"rbind",
				"ro",
			},
		},
		{
			Destination: "/sys/firmware",
			Type:        "tmpfs",
			Source:      "tmpfs",
			Options: []string{
				"nosuid",
				"noexec",
				"nodev",
				"mode=0555",
				"size=0",
			},
		},
		{
			Destination: "/sys/fs/bpf",
			Type:        "tmpfs",
			Source:      "tmpfs",
			Options: []string{
				"nosuid",
				"noexec",
				"nodev",
				"ro",
				"mode=0555",
				"size=0",
			},
		},
		{
			Destination: "/sys/fs/cgroup",
			Type:        "cgroup2",
			Source:      "cgroup",
			Options: []string{
				"nosuid",
				"nodev",
				"noexec",
				"ro",
			},
		},
		{
			Destination: "/dev/mqueue",
			Type:        "mqueue",
			Source:      "mqueue",
			Options: []string{
				"nosuid",
				"noexec",
				"nodev",
			},
		},
		{
			Destination: "/dev/shm",
			Type:        "tmpfs",
			Source:      "shm",
			Options: []string{
				"nosuid",
				"noexec",
				"nodev",
				"mode=1777",
				"size=67108864",
			},
		},
		{
			Destination: "/etc/resolv.conf",
			Type:        "bind",
			Source:      fmt.Sprintf("/etc/raind/container/%s/etc/resolv.conf", containerId),
			Options: []string{
				"rbind",
				"rprivate",
			},
		},
		{
			Destination: "/etc/hostname",
			Type:        "bind",
			Source:      fmt.Sprintf("/etc/raind/container/%s/etc/hostname", containerId),
			Options: []string{
				"rbind",
				"rprivate",
			},
		},
		{
			Destination: "/etc/hosts",
			Type:        "bind",
			Source:      fmt.Sprintf("/etc/raind/container/%s/etc/hosts", containerId),
			Options: []string{
				"rbind",
				"rprivate",
			},
		},
	}
}

func (p *rootContainerEnvPreparer) filterMissingManagedFileMounts(mounts []spec.MountObject) []spec.MountObject {
	filtered := make([]spec.MountObject, 0, len(mounts))
	for _, mountConfig := range mounts {
		if isManagedEtcMount(mountConfig) {
			if _, err := p.syscallHandler.Stat(mountConfig.Source); err != nil {
				continue
			}
		}
		filtered = append(filtered, mountConfig)
	}
	return filtered
}

func isManagedEtcMount(mountConfig spec.MountObject) bool {
	switch filepath.Clean(mountConfig.Destination) {
	case "/etc/resolv.conf", "/etc/hostname", "/etc/hosts":
		return strings.Contains(filepath.Clean(mountConfig.Source), string(filepath.Separator)+"etc"+string(filepath.Separator))
	default:
		return false
	}
}

func filterRootlessPrejoinedMounts(mounts []spec.MountObject) []spec.MountObject {
	filtered := make([]spec.MountObject, 0, len(mounts))
	for _, mountConfig := range mounts {
		destination := filepath.Clean(mountConfig.Destination)
		if destination == "/sys" || strings.HasPrefix(destination, "/sys/") || destination == "/dev/mqueue" {
			continue
		}
		filtered = append(filtered, mountConfig)
	}
	return filtered
}

// mountStdDevice bind-mounts standard device files into the container's /dev.
//
// The following devices are mounted from the host:
//   - /dev/random
//   - /dev/urandom
//   - /dev/null
//   - /dev/full
//   - /dev/zero
//   - /dev/tty
//
// If the destination file does not exist under rootfs, it is created first.
func (p *rootContainerEnvPreparer) mountStdDevice(rootfs string) error {
	devices := []string{
		"random",
		"urandom",
		"null",
		"zero",
		"full",
		"tty",
	}
	for _, device := range devices {
		destination := filepath.Join(rootfs, "dev", device)
		// check if the file exist
		if _, err := p.syscallHandler.Stat(destination); p.syscallHandler.IsNotExist(err) {
			// create
			if _, err := p.syscallHandler.Create(destination); err != nil {
				return err
			}
		}
		// mount
		if err := p.syscallHandler.Mount(
			"/dev/"+device,
			destination,
			"",
			syscall.MS_BIND,
			"",
		); err != nil {
			return fmt.Errorf("mount device failed: source=%q target=%q flags=0x%x: %w", "/dev/"+device, destination, syscall.MS_BIND, err)
		}
		// remount for setting read-only flag
		if err := p.syscallHandler.Mount(
			"",
			destination,
			"",
			syscall.MS_BIND|syscall.MS_REMOUNT|syscall.MS_RDONLY|syscall.MS_NOEXEC|syscall.MS_NOSUID,
			"",
		); err != nil {
			return fmt.Errorf("remount device failed: target=%q flags=0x%x: %w", destination, syscall.MS_BIND|syscall.MS_REMOUNT|syscall.MS_RDONLY|syscall.MS_NOEXEC|syscall.MS_NOSUID, err)
		}
	}
	return nil
}

// createSymbolicLink creates standard device-related symlinks under /dev
// inside the container rootfs.
//
// The following symlinks are created if they do not already exist:
//   - /dev/fd     -> /proc/self/fd
//   - /dev/stdin  -> /proc/self/fd/0
//   - /dev/stdout -> /proc/self/fd/1
//   - /dev/stderr -> /proc/self/fd/2
//   - /dev/ptmx   -> pts/ptmx
func (p *rootContainerEnvPreparer) createSymbolicLink(rootfs string) error {
	deviceDir := filepath.Join(rootfs, "dev")
	symlinks := []struct {
		link   string
		target string
	}{
		{filepath.Join(deviceDir, "fd"), "/proc/self/fd"},
		{filepath.Join(deviceDir, "stdin"), "/proc/self/fd/0"},
		{filepath.Join(deviceDir, "stdout"), "/proc/self/fd/1"},
		{filepath.Join(deviceDir, "stderr"), "/proc/self/fd/2"},
		{filepath.Join(deviceDir, "ptmx"), "pts/ptmx"},
	}

	for _, symlink := range symlinks {
		if _, err := p.syscallHandler.Lstat(symlink.link); err == nil {
			continue
		}
		if err := p.syscallHandler.Symlink(symlink.target, symlink.link); err != nil {
			return err
		}
	}

	return nil
}

// pivotRoot performs a pivot_root into the given rootfs and cleans up the old root.
//
// The sequence is:
//  1. create a put_old directory under the new root
//  2. call pivot_root(new_root, put_old)
//  3. chdir to "/"
//  4. unmount the old root at /put_old with MNT_DETACH
//  5. remove the /put_old directory
func (p *rootContainerEnvPreparer) pivotRoot(rootfs string) error {
	// oldroot directory
	putoldDir := filepath.Join(rootfs, "put_old")

	// 1. create put_old directory
	if err := p.syscallHandler.Mkdir(putoldDir, 0700); err != nil {
		return err
	}
	// 2. pivot_root
	if err := p.syscallHandler.PivotRoot(rootfs, putoldDir); err != nil {
		return err
	}
	// 3. change directory to root
	if err := p.syscallHandler.Chdir("/"); err != nil {
		return err
	}
	// 4. unmount put_old
	if err := p.syscallHandler.Unmount("/put_old", syscall.MNT_DETACH); err != nil {
		return err
	}
	// 5. remove put_old
	if err := p.syscallHandler.Rmdir("/put_old"); err != nil {
		return err
	}

	return nil
}

// setCapability configures Linux capabilities for the current (init) process
// according to the provided OCI capability configuration.
//
// The workflow is:
//  1. Create a capability set for PID 0 (the calling process)
//  2. Clear all capability sets (BOUNDING, PERMITTED, INHERITABLE, EFFECTIVE, AMBIENT)
//  3. Convert capability names from the spec to capability.Cap values
//  4. Populate each capability set from the corresponding field in capConfig
//  5. Apply the updated capability sets to the process
//
// If capability initialization or application fails, an error is returned.
func (p *rootContainerEnvPreparer) setCapability(capConfig spec.CapabilityObject) error {
	// set current process(init process) capability
	c, err := capability.NewPid2(0)
	if err != nil {
		return err
	}

	// clear all cap
	c.Clear(capability.BOUNDING | capability.PERMITTED | capability.INHERITABLE | capability.EFFECTIVE | capability.AMBIENT)

	// set bounding
	if len(capConfig.Bounding) > 0 {
		c.Set(capability.BOUNDING, toCaps(capConfig.Bounding)...)
	}
	// set permitted
	if len(capConfig.Permitted) > 0 {
		c.Set(capability.PERMITTED, toCaps(capConfig.Permitted)...)
	}
	// set inheritable
	if len(capConfig.Inheritable) > 0 {
		c.Set(capability.INHERITABLE, toCaps(capConfig.Inheritable)...)
	}
	// set effective
	if len(capConfig.Effective) > 0 {
		c.Set(capability.EFFECTIVE, toCaps(capConfig.Effective)...)
	}
	// set ambient
	if len(capConfig.Ambient) > 0 {
		c.Set(capability.AMBIENT, toCaps(capConfig.Ambient)...)
	}

	// apply
	if err := c.Apply(capability.BOUNDING | capability.PERMITTED | capability.INHERITABLE | capability.EFFECTIVE | capability.AMBIENT); err != nil {
		return fmt.Errorf("apply capability failed: %w", err)
	}

	return nil
}
