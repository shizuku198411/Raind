package container

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"raind/internal/droplet/logs"
	"raind/internal/droplet/spec"
	"raind/internal/droplet/status"
	"raind/internal/droplet/utils"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
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

func (c *ContainerShim) Execute(containerId string, fifo string, entrypoint []string) (err error) {
	var (
		spec  spec.Spec
		event = "shim"
		stage string
		pid   int
	)

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

	// 2. pty
	stage = "open_pty"
	ptmx, tty, err := pty.Open()
	if err != nil {
		return err
	}

	// 3. console socket listen
	stage = "listen_socket"
	sockPath := utils.SockPath(containerId)
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		return err
	}

	// open log
	stage = "open_log"
	shimLog, err := os.OpenFile(utils.ShimLogPath(containerId), os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		return err
	}
	defer shimLog.Close()
	logger := log.New(shimLog, "shim: ", log.LstdFlags|log.Lmicroseconds)
	consoleLog, err := os.OpenFile(utils.ConsoleLogPath(containerId), os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		return err
	}
	defer consoleLog.Close()

	// 4. prepare init subcommand
	stage = "prepare_init_command"
	initArgs := append([]string{"init", containerId, fifo}, entrypoint...)
	cmd := c.commandFactory.Command(utils.SelfBinPath(), initArgs...)

	// set stdio to tty
	cmd.SetStdin(tty)
	cmd.SetStdout(tty)
	cmd.SetStderr(tty)

	// apply SysProcAttr
	procAttr := buildProcAttrForContainer(spec)
	sysProcAttr := buildSysProcAttr(procAttr)
	sysProcAttr.Setsid = true
	sysProcAttr.Setctty = true
	sysProcAttr.Ctty = 0
	cmd.SetSysProcAttr(sysProcAttr)

	// 5. execute init subcommand
	stage = "exec_init"
	err = cmd.Start()
	if err != nil {
		logger.Printf("init start failed: %v", err)
		return err
	}
	initPid := cmd.Pid()
	pid = initPid
	logger.Printf("init started pid=%d", initPid)

	// 6. create pidfile
	stage = "create_pid_file"
	err = c.writeInitPid(containerId, initPid)
	if err != nil {
		logger.Printf("writeInitPid failed: %v", err)
		return err
	}

	// 6. close tty
	stage = "close_tty"
	_ = tty.Close()

	// 7. accept and proxy
	stage = "data_accept"
	h := newHub(ptmx, consoleLog, logger)
	h.startPump()
	go c.acceptLoop(ln, h, logger)

	// 8. wait init process
	//err = cmd.Wait()
	stage = "wait_init"
	waitErr := cmd.Wait()
	logger.Printf("init exited: %v", waitErr)

	// 9. update state
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

	// 10. set exit code, reason and message
	stage = "update exit status"
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

	_ = ln.Close()
	_ = os.Remove(sockPath)

	return waitErr
}

func (c *ContainerShim) specSecureLoad(containerId string) (spec.Spec, error) {
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

	return specFile, nil
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

func (c *ContainerShim) readFramesAndApply(r io.Reader, ptmx *os.File) error {
	h := make([]byte, 1+4)
	for {
		if _, err := io.ReadFull(r, h); err != nil {
			return err
		}
		typ := h[0]
		n := binary.BigEndian.Uint32(h[1:5])

		// safety limit (e.g. 8MB)
		if n > 8*1024*1024 {
			return fmt.Errorf("frame too large: %d", n)
		}

		payload := make([]byte, n)
		if n > 0 {
			if _, err := io.ReadFull(r, payload); err != nil {
				return err
			}
		}

		switch typ {
		case frameData:
			if len(payload) > 0 {
				if _, err := ptmx.Write(payload); err != nil {
					return err
				}
			}
		case frameResize:
			if len(payload) != 4 {
				continue
			}
			rows := binary.BigEndian.Uint16(payload[0:2])
			cols := binary.BigEndian.Uint16(payload[2:4])
			_ = pty.Setsize(ptmx, &pty.Winsize{Rows: rows, Cols: cols})
		default:
			// unknown frame -> ignore
		}
	}
}

func (c *ContainerShim) acceptLoop(ln net.Listener, h *hub, logger *log.Logger) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			if logger != nil {
				logger.Printf("accept error: %v", err)
			}
			time.Sleep(50 * time.Millisecond)
			continue
		}
		if logger != nil {
			logger.Printf("attach connected")
		}

		h.attach(conn)

		// conn -> ptmx (framed)
		go func(cc net.Conn) {
			_ = c.readFramesAndApply(cc, h.ptmx)
			h.detach(cc)
			_ = cc.Close()
			if logger != nil {
				logger.Printf("attach disconnected")
			}
		}(conn)
	}
}

type hub struct {
	ptmx *os.File

	mu      sync.Mutex
	conn    net.Conn // nil if detached
	console *os.File // console.log
	logger  *log.Logger
}

func newHub(ptmx *os.File, console *os.File, logger *log.Logger) *hub {
	return &hub{ptmx: ptmx, console: console, logger: logger}
}

func (h *hub) attach(c net.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// single attach only: close previous
	if h.conn != nil {
		_ = h.conn.Close()
	}
	h.conn = c
}

func (h *hub) detach(c net.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.conn == c {
		h.conn = nil
	}
}

func (h *hub) startPump() {
	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := h.ptmx.Read(buf)
			if n > 0 {
				if h.console != nil {
					_, _ = h.console.Write(buf[:n])
				}

				h.mu.Lock()
				c := h.conn
				h.mu.Unlock()
				if c != nil {
					_, _ = c.Write(buf[:n])
				}
			}
			if err != nil {
				if h.logger != nil {
					h.logger.Printf("ptmx read end: %v", err)
				}
				return
			}
		}
	}()
}
