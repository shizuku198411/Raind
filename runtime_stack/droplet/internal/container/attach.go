package container

import (
	"droplet/internal/utils"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"golang.org/x/term"
)

func NewContainerAttach() *ContainerAttach {
	return &ContainerAttach{}
}

const (
	frameData   = 0x00
	frameResize = 0x01
)

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
			if werr := c.writeFrame(conn, frameData, buf[:n]); werr != nil {
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
	binary.BigEndian.PutUint16(payload[0:2], uint16(h)) // rows
	binary.BigEndian.PutUint16(payload[2:4], uint16(w)) // cols
	return c.writeFrame(conn, frameResize, payload)
}

func (c *ContainerAttach) writeFrame(w io.Writer, typ byte, payload []byte) error {
	h := make([]byte, 1+4)
	h[0] = typ
	binary.BigEndian.PutUint32(h[1:5], uint32(len(payload)))
	if _, err := w.Write(h); err != nil {
		return err
	}
	if len(payload) > 0 {
		_, err := w.Write(payload)
		return err
	}
	return nil
}
