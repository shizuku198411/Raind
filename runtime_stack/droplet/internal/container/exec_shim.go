package container

import (
	"droplet/internal/logs"
	"droplet/internal/spec"
	"droplet/internal/utils"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"slices"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
)

func NewContainerExecShim() *ContainerExecShim {
	return &ContainerExecShim{
		specLoader:     newFileSpecLoader(),
		commandFactory: &utils.ExecCommandFactory{},
	}
}

type ContainerExecShim struct {
	specLoader     specLoader
	commandFactory utils.CommandFactory
}

func (c *ContainerExecShim) Execute(containerId string, containerPid string, entrypoint []string) (err error) {
	var (
		spec  spec.Spec
		event = "exec_shim"
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

	// open log
	stage = "open_log"
	shimLog, err := os.OpenFile(utils.ExecShimLogPath(containerId), os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		return err
	}
	defer shimLog.Close()
	logger := log.New(shimLog, "exec_shim: ", log.LstdFlags|log.Lmicroseconds)

	// 1. remove old file
	stage = "remove_old_socket"
	sockPath := utils.ExecSockPath(containerId)
	err = os.Remove(sockPath)
	if err != nil && !os.IsNotExist(err) {
		logger.Printf("sock path remove failed: %v", err)
		return err
	}

	// 2. pty
	stage = "open_pty"
	ptmx, tty, err := pty.Open()
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(sockPath) }()

	// 3. console socket listen
	stage = "listen_socket"
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		logger.Printf("unix socket listen failed: %v", err)
		return err
	}

	// 4. prepare nsenter command
	nsenterCommand := []string{"nsenter", "-t", containerPid, "--all"}
	commandStr := slices.Concat(nsenterCommand, entrypoint)
	cmd := c.commandFactory.Command(commandStr[0], commandStr[1:]...)
	// set stdio to tty
	cmd.SetStdin(tty)
	cmd.SetStdout(tty)
	cmd.SetStderr(tty)
	cmd.SetSysProcAttr(&syscall.SysProcAttr{
		Setsid:  true,
		Setctty: true,
		Ctty:    0,
	})

	// 5. execute nsenter command
	stage = "exec_command"
	err = cmd.Start()
	if err != nil {
		logger.Printf("nsenter failed: %v", err)
		return err
	}
	nsenterPid := cmd.Pid()
	pid = nsenterPid
	logger.Printf("nsenter started pid=%d", nsenterPid)

	// 6. close tty
	_ = tty.Close()

	// 7. accept and proxy
	consoleLog, err := os.OpenFile(utils.ExecConsoleLogPath(containerId), os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		return err
	}
	defer consoleLog.Close()
	h := newExecHub(ptmx, consoleLog, logger)
	h.startPump()
	go c.acceptLoop(ln, h, logger)

	// 8. wait init process
	//err = cmd.Wait()
	waitErr := cmd.Wait()
	logger.Printf("nsenter exited: %v", waitErr)

	_ = ln.Close()
	_ = os.Remove(sockPath)

	return waitErr
}

func (c *ContainerExecShim) readFramesAndApply(r io.Reader, ptmx *os.File) error {
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

func (c *ContainerExecShim) acceptLoop(ln net.Listener, h *execHub, logger *log.Logger) {
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

type execHub struct {
	ptmx *os.File

	mu      sync.Mutex
	conn    net.Conn // nil if detached
	console *os.File // console.log
	logger  *log.Logger
}

func newExecHub(ptmx *os.File, console *os.File, logger *log.Logger) *execHub {
	return &execHub{ptmx: ptmx, console: console, logger: logger}
}

func (h *execHub) attach(c net.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// single attach only: close previous
	if h.conn != nil {
		_ = h.conn.Close()
	}
	h.conn = c
}

func (h *execHub) detach(c net.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.conn == c {
		h.conn = nil
	}
}

func (h *execHub) startPump() {
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
