package attachio

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/creack/pty"
)

const (
	FrameData   = 0x00
	FrameResize = 0x01
)

type Hub struct {
	ptmx *os.File

	mu      sync.Mutex
	conn    net.Conn // nil if detached
	console *os.File // console.log
	logger  *log.Logger
}

func NewHub(ptmx *os.File, console *os.File, logger *log.Logger) *Hub {
	return &Hub{ptmx: ptmx, console: console, logger: logger}
}

func (h *Hub) Attach(c net.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// single attach only: close previous
	if h.conn != nil {
		_ = h.conn.Close()
	}
	h.conn = c
}

func (h *Hub) detach(c net.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.conn == c {
		h.conn = nil
	}
}

func (h *Hub) StartPump() {
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

func AcceptLoop(ln net.Listener, h *Hub, logger *log.Logger) {
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

		h.Attach(conn)

		// conn -> ptmx (framed)
		go func(cc net.Conn) {
			_ = ReadFramesAndApply(cc, h.ptmx)
			h.detach(cc)
			_ = cc.Close()
			if logger != nil {
				logger.Printf("attach disconnected")
			}
		}(conn)
	}
}

func ReadFramesAndApply(r io.Reader, ptmx *os.File) error {
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
		case FrameData:
			if len(payload) > 0 {
				if _, err := ptmx.Write(payload); err != nil {
					return err
				}
			}
		case FrameResize:
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

func WriteFrame(w io.Writer, typ byte, payload []byte) error {
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

func PutWinsize(payload []byte, rows uint16, cols uint16) {
	binary.BigEndian.PutUint16(payload[0:2], rows)
	binary.BigEndian.PutUint16(payload[2:4], cols)
}
