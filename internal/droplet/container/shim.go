package container

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"raind/internal/droplet/container/attachio"
	"raind/internal/droplet/logs"
	"raind/internal/droplet/spec"
	"raind/internal/droplet/status"
	"raind/internal/droplet/utils"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

func NewContainerShim() *ContainerShim {
	return &ContainerShim{
		specLoader:             newFileSpecLoader(),
		commandFactory:         &utils.ExecCommandFactory{},
		containerStatusManager: status.NewStatusHandler(),
	}
}

type ContainerShim struct {
	specLoader             specLoader
	commandFactory         utils.CommandFactory
	containerStatusManager status.ContainerStatusManager
}

type ShimExecuteOption struct {
	ContainerId   string
	Fifo          string
	Entrypoint    []string
	Tty           bool
	ConsoleSocket string
}

func (c *ContainerShim) Execute(opt ShimExecuteOption) (err error) {
	var (
		spec  spec.Spec
		event = "shim"
		stage string
		pid   int
	)

	containerId := opt.ContainerId

	// audit log
	defer func() {
		result := "success"
		if err != nil {
			result = "fail"
		}
		_ = logs.RecordAuditLog(logs.AuditRecord{
			ContainerId: containerId,
			Event:       event,
			Stage:       stage,
			Pid:         pid,
			Spec:        &spec,
			Result:      result,
			Error:       err,
		})
	}()

	// 1. load config.json
	stage = "load_spec"
	spec, err = c.specSecureLoad(containerId)
	if err != nil {
		return err
	}

	// open shim log
	stage = "open_log"
	shimLog, err := os.OpenFile(utils.ShimLogPath(containerId), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0640)
	if err != nil {
		return err
	}
	defer shimLog.Close()
	logger := log.New(shimLog, "shim: ", log.LstdFlags|log.Lmicroseconds)

	// 2. prepare init subcommand
	stage = "prepare_init_command"
	initArgs := append([]string{"init", containerId, opt.Fifo}, opt.Entrypoint...)
	cmdName := utils.SelfBinPath()
	cmdArgs := initArgs
	pidNamespacePath := buildNamespaceConfig(spec).pidPath
	usePrejoin := shouldPrejoinRootlessPathNamespaces(spec)
	if nsenterArgs := namespacePathNsenterArgs(spec); len(nsenterArgs) > 0 && !usePrejoin {
		cmdName = "nsenter"
		cmdArgs = append(nsenterArgs, "--", utils.SelfBinPath())
		cmdArgs = append(cmdArgs, initArgs...)
	}
	cmd := c.commandFactory.Command(cmdName, cmdArgs...)
	prejoinedNamespaces, unlockOSThread, err := prejoinRootlessPathNamespaces(spec)
	if unlockOSThread != nil {
		defer unlockOSThread()
	}
	if err != nil {
		logger.Printf("prejoin namespaces failed: %v", err)
		return err
	}
	if prejoinedNamespaces || len(namespacePathNsenterArgs(spec)) > 0 {
		cmd.SetEnv(append(os.Environ(), raindNamespacesPrejoinedEnv+"=1"))
	}

	// 3. configure stdio.
	// TTY mode keeps the existing pty/socket attach path.
	// Non-TTY mode still uses shim as a supervisor, but stdin is /dev/null and
	// stdout/stderr are written to init.log just like the previous direct init path.
	var (
		ptmx     *os.File
		tty      *os.File
		ln       net.Listener
		sockPath string
	)
	if opt.Tty {
		stage = "open_pty"
		ptmx, tty, err = pty.Open()
		if err != nil {
			return err
		}
		defer ptmx.Close()
		defer tty.Close()

		stage = "set_console_size"
		if err := applyConsoleSize(ptmx, spec.Process.ConsoleSize); err != nil {
			return err
		}

		if opt.ConsoleSocket != "" {
			stage = "send_console_socket"
			if err := sendConsoleFileDescriptor(opt.ConsoleSocket, ptmx); err != nil {
				return err
			}
		}

		stage = "listen_socket"
		sockPath = utils.SockPath(containerId)
		ln, err = net.Listen("unix", sockPath)
		if err != nil {
			return err
		}
		defer ln.Close()

		containerLog, err := os.OpenFile(utils.ContainerLogPath(containerId), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640)
		if err != nil {
			return err
		}
		defer containerLog.Close()

		cmd.SetStdin(tty)
		cmd.SetStdout(tty)
		cmd.SetStderr(tty)

		// Start accepting attach connections before init starts. The attach client
		// may connect immediately after create/start returns.
		h := attachio.NewHub(ptmx, containerLog, logger)
		h.StartPump()
		go attachio.AcceptLoop(ln, h, logger)
	} else {
		stage = "open_init_log"
		logPath := utils.ContainerLogPath(containerId)
		initLog, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640)
		if err != nil {
			return err
		}
		defer initLog.Close()
		if rootlessConfig, ok := rootlessConfigFromSpec(spec); ok {
			if err := prepareRootlessInitLog(logPath, rootlessConfig); err != nil {
				return err
			}
		}

		devNull, err := os.OpenFile(os.DevNull, os.O_RDONLY, 0)
		if err != nil {
			return err
		}
		defer devNull.Close()

		cmd.SetStdin(devNull)
		if isOCIBundleMode(containerId) {
			cmd.SetStdout(io.MultiWriter(initLog, os.Stdout))
			cmd.SetStderr(io.MultiWriter(initLog, os.Stderr))
		} else {
			cmd.SetStdout(initLog)
			cmd.SetStderr(initLog)
		}
	}

	// apply SysProcAttr
	procAttr := buildProcAttrForContainer(spec)
	sysProcAttr := buildSysProcAttr(procAttr)
	sysProcAttr.Setsid = true
	if opt.Tty {
		sysProcAttr.Setctty = true
		sysProcAttr.Ctty = 0
	}
	cmd.SetSysProcAttr(sysProcAttr)

	// 4. execute init subcommand
	stage = "exec_init"
	err = cmd.Start()
	if err != nil {
		logger.Printf("init start failed: %v", err)
		return err
	}
	initPid := cmd.Pid()
	if pidNamespacePath != "" {
		childPID, err := waitFirstChildPID(cmd.Pid(), 3*time.Second, 20*time.Millisecond)
		if err != nil {
			logger.Printf("wait pid namespace child failed: %v", err)
			return err
		}
		initPid = childPID
	}
	pid = initPid
	logger.Printf("init started pid=%d tty=%t", initPid, opt.Tty)

	// 5. create pidfile
	stage = "create_pid_file"
	err = c.writeInitPid(containerId, initPid)
	if err != nil {
		logger.Printf("writeInitPid failed: %v", err)
		return err
	}

	if opt.Tty {
		// The child process already inherited the slave side. Close the shim copy so
		// ptmx observes EOF once the container process exits.
		stage = "close_tty"
		_ = tty.Close()
	}

	// 6. wait init process
	stage = "wait_init"
	waitErr := cmd.Wait()
	logger.Printf("init exited: %v", waitErr)

	// 7. update state
	stage = "update_state"
	err = c.containerStatusManager.UpdateStatus(
		containerId,
		status.STOPPED,
		0,
		0,
	)
	if err != nil {
		return err
	}

	// 8. set exit code, reason and message
	stage = "update_exit_status"
	exitCode := c.InitExitCode(cmd)
	err = c.containerStatusManager.UpdateExitCode(containerId, exitCode)
	if err != nil {
		logger.Printf("update exit code failed: %v", err)
		return err
	}
	message := ""
	if waitErr != nil {
		message = waitErr.Error()
	}
	err = c.SetReasonAndMessage(containerId, exitCode, "", message)
	if err != nil {
		logger.Printf("update reason and message failed: %v", err)
		return err
	}

	if ln != nil {
		_ = ln.Close()
	}
	if sockPath != "" {
		_ = os.Remove(sockPath)
	}

	return waitErr
}

func waitFirstChildPID(parentPID int, timeout time.Duration, pollInterval time.Duration) (int, error) {
	if pollInterval <= 0 {
		pollInterval = 20 * time.Millisecond
	}
	deadline := time.Now().Add(timeout)
	for {
		if childPID, ok := firstChildPID(parentPID); ok {
			return childPID, nil
		}
		if childPID, ok := scanFirstChildPID(parentPID); ok {
			return childPID, nil
		}
		if time.Now().After(deadline) {
			return 0, fmt.Errorf("timed out waiting for child of pid %d", parentPID)
		}
		time.Sleep(pollInterval)
	}
}

func applyConsoleSize(ptmx *os.File, size *spec.ConsoleSizeObject) error {
	winsize, err := consoleWinsize(size)
	if err != nil {
		return err
	}
	if winsize == nil {
		return nil
	}
	if err := pty.Setsize(ptmx, winsize); err != nil {
		return fmt.Errorf("set console size: %w", err)
	}
	return nil
}

func consoleWinsize(size *spec.ConsoleSizeObject) (*pty.Winsize, error) {
	if size == nil {
		return nil, nil
	}
	if size.Height == 0 || size.Width == 0 {
		return nil, fmt.Errorf("process.consoleSize height and width must be positive")
	}
	if size.Height > spec.MaxConsoleSize || size.Width > spec.MaxConsoleSize {
		return nil, fmt.Errorf("process.consoleSize height and width must be <= %d", spec.MaxConsoleSize)
	}
	return &pty.Winsize{
		Rows: uint16(size.Height),
		Cols: uint16(size.Width),
	}, nil
}

func sendConsoleFileDescriptor(socketPath string, console *os.File) error {
	addr := net.UnixAddr{Name: socketPath, Net: "unix"}
	conn, err := net.DialUnix("unix", nil, &addr)
	if err != nil {
		return fmt.Errorf("dial console socket: %w", err)
	}
	defer conn.Close()

	rights := unix.UnixRights(int(console.Fd()))
	if _, _, err := conn.WriteMsgUnix([]byte{0}, rights, nil); err != nil {
		return fmt.Errorf("send console fd: %w", err)
	}
	return nil
}

const raindNamespacesPrejoinedEnv = "RAIND_NAMESPACES_PREJOINED"

func prejoinRootlessPathNamespaces(containerSpec spec.Spec) (bool, func(), error) {
	if !shouldPrejoinRootlessPathNamespaces(containerSpec) {
		return false, nil, nil
	}

	runtime.LockOSThread()
	unlock := runtime.UnlockOSThread
	if err := joinExistingNamespaces(containerSpec); err != nil {
		unlock()
		return false, nil, fmt.Errorf("prejoin namespaces: %w", err)
	}
	if err := allowRootlessSharedNetworkLowPorts(); err != nil {
		unlock()
		return false, nil, err
	}
	return true, unlock, nil
}

func shouldPrejoinRootlessPathNamespaces(containerSpec spec.Spec) bool {
	if !isRootlessSpec(containerSpec) {
		return false
	}
	return len(buildNamespaceJoinTargets(containerSpec)) > 0
}

func allowRootlessSharedNetworkLowPorts() error {
	const path = "/proc/sys/net/ipv4/ip_unprivileged_port_start"
	if err := os.WriteFile(path, []byte("0\n"), 0644); err != nil {
		return fmt.Errorf("allow rootless low ports in shared network namespace: %w", err)
	}
	return nil
}

func (c *ContainerShim) specSecureLoad(containerId string) (spec.Spec, error) {
	return verifyAndLoadSpec(containerId, c.specLoader)
}

// WriteInitPid atomically writes initPid to pidfile.
//
// Atomicity strategy:
//  1. create temp file in same dir
//  2. write content, fsync temp file
//  3. close
//  4. rename temp -> final (POSIX atomic in same filesystem)
//  5. fsync dir
func (c *ContainerShim) writeInitPid(containerId string, initPid int) error {
	if initPid <= 0 {
		return fmt.Errorf("invalid init pid: %d", initPid)
	}

	pidPath := utils.InitPidFilePath(containerId)
	dir := filepath.Dir(pidPath)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir container dir: %w", err)
	}

	// Create temp file in same directory for atomic rename.
	tmp, err := os.CreateTemp(dir, ".init.pid.*.tmp")
	if err != nil {
		return fmt.Errorf("create temp pidfile: %w", err)
	}

	tmpName := tmp.Name()
	// cleanup on failure
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}()

	content := []byte(strconv.Itoa(initPid) + "\n")

	if _, err := tmp.Write(content); err != nil {
		return fmt.Errorf("write temp pidfile: %w", err)
	}

	// Ensure file content is flushed to disk.
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync temp pidfile: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp pidfile: %w", err)
	}

	// Atomic replace.
	if err := os.Rename(tmpName, pidPath); err != nil {
		return fmt.Errorf("rename pidfile: %w", err)
	}

	// Best-effort fsync directory for crash consistency.
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}

	return nil
}

func (c *ContainerShim) InitExitCode(proc utils.CommandExecutor) int {
	if proc == nil {
		return -1
	}

	if code := proc.ExitCode(); code >= 0 {
		return code
	}

	ws, ok := proc.Sys().(syscall.WaitStatus)
	if !ok {
		return -1
	}
	if ws.Signaled() {
		return 128 + int(ws.Signal())
	}

	return -1
}

func (c *ContainerShim) SetReasonAndMessage(containerId string, exitCode int, reason, message string) error {
	if exitCode == 0 {
		if err := c.containerStatusManager.UpdateReasonAndMessage(
			containerId, "Completed", "exit code: "+strconv.Itoa(exitCode),
		); err != nil {
			return err
		}
		return nil
	} else if exitCode != 0 {
		if err := c.containerStatusManager.UpdateReasonAndMessage(
			containerId, "Error", message,
		); err != nil {
			return err
		}
		return nil
	}
	return nil
}
