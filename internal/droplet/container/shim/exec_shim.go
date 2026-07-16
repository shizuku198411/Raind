package shim

import (
	"log"
	"net"
	"os"
	"slices"
	"syscall"

	"raind/internal/droplet/container/attachio"
	"raind/internal/droplet/container/audit"
	"raind/internal/droplet/spec"
	"raind/internal/droplet/utils"

	"github.com/creack/pty"
)

type ExecShim struct {
	LoadSpec            func(containerId string) (spec.Spec, error)
	CommandFactory      utils.CommandFactory
	ResolveEntrypoint   func(containerPid string, containerSpec spec.Spec, entrypoint []string) ([]string, error)
	BuildNsenterCommand func(containerPid string, containerSpec spec.Spec) []string
}

func (c *ExecShim) Execute(containerId string, containerPid string, entrypoint []string) (err error) {
	var specFile spec.Spec

	auditLog := audit.New(containerId, "exec_shim")
	auditLog.SetSpec(&specFile)
	defer auditLog.Record(&err)

	auditLog.Stage("open_log")
	shimLog, err := os.OpenFile(utils.ExecShimLogPath(containerId), os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		return err
	}
	defer shimLog.Close()
	logger := log.New(shimLog, "exec_shim: ", log.LstdFlags|log.Lmicroseconds)

	auditLog.Stage("remove_old_socket")
	sockPath := utils.ExecSockPath(containerId)
	err = os.Remove(sockPath)
	if err != nil && !os.IsNotExist(err) {
		logger.Printf("sock path remove failed: %v", err)
		return err
	}

	auditLog.Stage("open_pty")
	ptmx, tty, err := pty.Open()
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(sockPath) }()

	auditLog.Stage("listen_socket")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		logger.Printf("unix socket listen failed: %v", err)
		return err
	}

	auditLog.Stage("load_spec")
	specFile, err = c.LoadSpec(containerId)
	if err != nil {
		logger.Printf("load spec failed: %v", err)
		return err
	}

	resolvedEntrypoint, err := c.ResolveEntrypoint(containerPid, specFile, entrypoint)
	if err != nil {
		logger.Printf("resolve entrypoint failed: %v", err)
		return err
	}

	nsenterCommand := c.BuildNsenterCommand(containerPid, specFile)
	commandStr := slices.Concat(nsenterCommand, resolvedEntrypoint)
	cmd := c.CommandFactory.Command(commandStr[0], commandStr[1:]...)
	cmd.SetEnv(specFile.Process.Env)
	cmd.SetStdin(tty)
	cmd.SetStdout(tty)
	cmd.SetStderr(tty)
	cmd.SetSysProcAttr(&syscall.SysProcAttr{
		Setsid:  true,
		Setctty: true,
		Ctty:    0,
	})

	auditLog.Stage("exec_command")
	err = cmd.Start()
	if err != nil {
		logger.Printf("nsenter failed: %v", err)
		return err
	}
	nsenterPid := cmd.Pid()
	auditLog.SetPid(nsenterPid)
	logger.Printf("nsenter started pid=%d", nsenterPid)

	_ = tty.Close()

	containerLog, err := os.OpenFile(utils.ContainerLogPath(containerId), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640)
	if err != nil {
		return err
	}
	defer containerLog.Close()
	h := attachio.NewHub(ptmx, containerLog, logger)
	h.StartPump()
	go attachio.AcceptLoop(ln, h, logger)

	waitErr := cmd.Wait()
	logger.Printf("nsenter exited: %v", waitErr)

	_ = ln.Close()
	_ = os.Remove(sockPath)

	return waitErr
}
