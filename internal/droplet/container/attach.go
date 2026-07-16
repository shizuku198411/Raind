package container

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"raind/internal/droplet/container/attachio"
	"raind/internal/droplet/utils"
	"sync"
	"syscall"

	"golang.org/x/term"
)

func NewContainerAttach() *ContainerAttach {
	return &ContainerAttach{}
}

type ContainerAttach struct{}

func (c *ContainerAttach) Execute(opt AttachOption) error {
	sockPath := utils.SockPath(opt.ContainerId)
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		return fmt.Errorf("dial console soclet: %w", err)
	}
	defer conn.Close()

	// TTY: raw mode
	isTTY := term.IsTerminal(int(os.Stdin.Fd()))
	if isTTY {
		oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			return fmt.Errorf("make raw: %w", err)
		}
		defer func() { _ = term.Restore(int(os.Stdin.Fd()), oldState) }()
	}

	// start resize watcher
	stopResize := make(chan struct{})
	if isTTY {
		_ = c.sendResize(conn)
		go c.watchWinch(conn, stopResize)
		defer close(stopResize)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	wg.Add(2)

	// socket -> stdout (raw data is sent from shim)
	go func() {
		defer wg.Done()
		_, e := io.Copy(os.Stdout, conn)
		errCh <- e
	}()

	// stdin -> socket (send frame data)
	go func() {
		defer wg.Done()
		e := c.pumpStdinFramed(conn, os.Stdin)
		errCh <- e
	}()

	e := <-errCh
	_ = conn.Close()
	wg.Wait()

	if e == io.EOF {
		return nil
	}
	return e
}

func (c *ContainerAttach) pumpStdinFramed(conn net.Conn, r io.Reader) error {
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			if werr := c.writeFrame(conn, attachio.FrameData, buf[:n]); werr != nil {
				return werr
			}
		}
		if err != nil {
			return err
		}
	}
}

func (c *ContainerAttach) watchWinch(conn net.Conn, stop <-chan struct{}) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	defer signal.Stop(ch)

	for {
		select {
		case <-stop:
			return
		case <-ch:
			_ = c.sendResize(conn)
		}
	}
}

func (c *ContainerAttach) sendResize(conn net.Conn) error {
	fd := int(os.Stdout.Fd())
	if !term.IsTerminal(fd) {
		return nil
	}
	w, h, err := term.GetSize(fd) // (cols, rows)
	if err != nil {
		return err
	}
	payload := make([]byte, 4)
	attachio.PutWinsize(payload, uint16(h), uint16(w))
	return c.writeFrame(conn, attachio.FrameResize, payload)
}

func (c *ContainerAttach) writeFrame(w io.Writer, typ byte, payload []byte) error {
	return attachio.WriteFrame(w, typ, payload)
}
