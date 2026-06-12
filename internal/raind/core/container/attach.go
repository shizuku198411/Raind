package container

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	httpclient "raind/internal/raind/core/client"
	"raind/internal/raind/utils"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/term"
)

const (
	frameData   = 0x00
	frameResize = 0x01
)

func NewServiceContainerAttach() *ServiceContainerAttach {
	return &ServiceContainerAttach{}
}

type ServiceContainerAttach struct{}

// Attach connects to Condenser websocket endpoint and attaches to container TTY.
func (c *ServiceContainerAttach) Attach(containerId string) error {
	wsURL := fmt.Sprintf("wss://localhost:7755/v1/containers/%s/attach", containerId)
	u, err := url.Parse(wsURL)
	if err != nil {
		return fmt.Errorf("parse ws url: %w", err)
	}

	// Dial websocket
	httpClient, err := httpclient.NewHttpClient()
	if err != nil {
		return err
	}
	certPaths := utils.ResolveClientCertPaths()

	dialer, err := httpClient.NewMTLSDialer(
		certPaths.CA,
		certPaths.Cert,
		certPaths.Key,
	)
	if err != nil {
		return err
	}
	ws, resp, err := dialer.Dial(u.String(), http.Header{})
	if err != nil {
		if resp != nil {
			defer resp.Body.Close()
			b, _ := io.ReadAll(resp.Body)
			return errors.New(string(b))
		}
		return fmt.Errorf("Cannot connect to the Raind daemon. Is the raind daemon running?")
	}
	defer ws.Close()

	fmt.Println("attached. detach with Ctrl-P Ctrl-Q")

	// TTY raw mode
	isTTY := term.IsTerminal(int(os.Stdin.Fd()))
	if isTTY {
		old, err := term.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			return fmt.Errorf("make raw: %w", err)
		}
		defer func() { _ = term.Restore(int(os.Stdin.Fd()), old) }()
	}

	// WS reader/writer adapters
	wsr := newWSBinaryStreamReader(ws)
	wsw := newWSBinaryStreamWriter(ws)

	// Start resize sender (initial + SIGWINCH)
	stopResize := make(chan struct{})
	if isTTY {
		_ = sendResizeFrame(wsw)
		go watchWinchAndResize(wsw, stopResize)
		defer close(stopResize)
	}

	errCh := make(chan error, 2)
	detachCh := make(chan struct{})
	var wg sync.WaitGroup

	// Droplet output (raw) -> stdout
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, e := io.Copy(os.Stdout, wsr)
		errCh <- e
	}()

	// stdin -> Droplet input (framed) -> websocket(binary) with detach keys
	wg.Add(1)
	go func() {
		defer wg.Done()
		e := pumpStdinWithDetach(wsw, os.Stdin, detachCh)
		if e != nil {
			errCh <- e
		}
	}()

	// Either direction ends OR detach => close WS and return
	select {
	case e := <-errCh:
		_ = ws.Close()
		wg.Wait()
		return normalizeAttachErr(e)

	case <-detachCh:
		// detach
		_ = ws.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseGoingAway, "detached"),
			time.Now().Add(1*time.Second),
		)
		_ = ws.Close()
		wg.Wait()
		return nil
	}
}

// pumpStdinAsFrames reads from stdin and writes data frames to w.
func pumpStdinAsFrames(w io.Writer, r io.Reader) error {
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			if werr := writeFrame(w, frameData, buf[:n]); werr != nil {
				return werr
			}
		}
		if err != nil {
			return err
		}
	}
}

// sendResizeFrame writes a resize frame via stream writer (wsw).
// This avoids concurrent websocket writes (resize vs stdin frames).
func sendResizeFrame(w io.Writer) error {
	fd := int(os.Stdout.Fd())
	if !term.IsTerminal(fd) {
		return nil
	}
	cols, rows, err := term.GetSize(fd) // (width, height) = (cols, rows)
	if err != nil {
		return err
	}

	payload := make([]byte, 4)
	binary.BigEndian.PutUint16(payload[0:2], uint16(rows))
	binary.BigEndian.PutUint16(payload[2:4], uint16(cols))

	return writeFrame(w, frameResize, payload)
}

func watchWinchAndResize(w io.Writer, stop <-chan struct{}) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	defer signal.Stop(ch)

	for {
		select {
		case <-stop:
			return
		case <-ch:
			_ = sendResizeFrame(w)
		}
	}
}

// writeFrame writes one frame to stream writer w.
// frame: [type:1][len:4][payload:len]
func writeFrame(w io.Writer, typ byte, payload []byte) error {
	_, err := w.Write(buildFrame(typ, payload))
	return err
}

func buildFrame(typ byte, payload []byte) []byte {
	h := make([]byte, 1+4+len(payload))
	h[0] = typ
	binary.BigEndian.PutUint32(h[1:5], uint32(len(payload)))
	copy(h[5:], payload)
	return h
}

// --- WebSocket stream adapters ---
//
// newWSBinaryStreamReader concatenates WS binary messages into one stream.
// This is required because shim reads framed bytes as a continuous stream.
type wsBinaryStreamReader struct {
	ws  *websocket.Conn
	cur io.Reader
}

func newWSBinaryStreamReader(ws *websocket.Conn) *wsBinaryStreamReader {
	return &wsBinaryStreamReader{ws: ws}
}

func (r *wsBinaryStreamReader) Read(p []byte) (int, error) {
	for {
		if r.cur != nil {
			n, err := r.cur.Read(p)
			if err == io.EOF {
				r.cur = nil
				continue
			}
			return n, err
		}
		mt, rd, err := r.ws.NextReader()
		if err != nil {
			return 0, err
		}
		if mt != websocket.BinaryMessage {
			// ignore non-binary
			continue
		}
		r.cur = rd
	}
}

// newWSBinaryStreamWriter turns Write(p) into WS binary messages.
// It chunks writes and serializes concurrent writes via a mutex.
type wsBinaryStreamWriter struct {
	ws *websocket.Conn
	mu sync.Mutex
}

func newWSBinaryStreamWriter(ws *websocket.Conn) *wsBinaryStreamWriter {
	return &wsBinaryStreamWriter{ws: ws}
}

func (w *wsBinaryStreamWriter) Write(p []byte) (int, error) {
	const chunk = 32 * 1024
	total := 0

	w.mu.Lock()
	defer w.mu.Unlock()

	for len(p) > 0 {
		n := len(p)
		if n > chunk {
			n = chunk
		}

		wr, err := w.ws.NextWriter(websocket.BinaryMessage)
		if err != nil {
			return total, err
		}
		if _, err := wr.Write(p[:n]); err != nil {
			_ = wr.Close()
			return total, err
		}
		if err := wr.Close(); err != nil {
			return total, err
		}

		total += n
		p = p[n:]
	}
	return total, nil
}

func pumpStdinWithDetach(
	w io.Writer,
	r *os.File,
	detachCh chan<- struct{},
) error {
	buf := make([]byte, 1)
	var lastCtrlP bool

	for {
		n, err := r.Read(buf)
		if n > 0 {
			b := buf[0]

			// Ctrl-P (0x10)
			if b == 0x10 {
				lastCtrlP = true
				continue
			}

			// Ctrl-Q (0x11) after Ctrl-P → detach
			if lastCtrlP && b == 0x11 {
				close(detachCh)
				return nil
			}

			lastCtrlP = false

			if err := writeFrame(w, frameData, buf[:1]); err != nil {
				return err
			}
		}

		if err != nil {
			return err
		}
	}
}

func normalizeAttachErr(e error) error {
	// EOF is normal when user closes stdin / server closes
	if e == io.EOF || e == nil {
		return nil
	}
	if websocket.IsCloseError(e, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
		return nil
	}
	return e
}
