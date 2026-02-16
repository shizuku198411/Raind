package websocket

import (
	"condenser/internal/api/http/logger"
	"condenser/internal/store/csm"
	"condenser/internal/utils"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

func NewRequestHandler() *Handler {
	return &Handler{
		Resolver: StaticResolver{
			ContainerRoot: utils.ContainerRootDir,
			SockName:      "tty.sock",
		},
		Upgrader:   websocket.Upgrader{},
		csmHandler: csm.NewCsmManager(csm.NewCsmStore(utils.CsmStorePath)),
	}
}

func NewExecRequestHandler() *Handler {
	return &Handler{
		Resolver: StaticResolver{
			ContainerRoot: utils.ContainerRootDir,
			SockName:      "exec_tty.sock",
		},
		csmHandler: csm.NewCsmManager(csm.NewCsmStore(utils.CsmStorePath)),
	}
}

type SockResolver interface {
	// containerId -> path to droplet shim's unix socket (tty.sock)
	ConsoleSockPath(containerId string) (string, error)
}

type StaticResolver struct {
	ContainerRoot string // e.g. "/etc/raind/container"
	SockName      string // e.g. "tty.sock"
}

func (r StaticResolver) ConsoleSockPath(containerId string) (string, error) {
	name := r.SockName
	if name == "" {
		name = "tty.sock"
	}
	return filepath.Join(r.ContainerRoot, containerId, name), nil
}

type Handler struct {
	Resolver   SockResolver
	Upgrader   websocket.Upgrader
	csmHandler csm.CsmHandler
}

// ServeHTTP handles GET /containers/{id}/attach (WebSocket)
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	target := chi.URLParam(r, "containerId")
	if target == "" {
		http.Error(w, "missing container id", http.StatusBadRequest)
		return
	}
	containerId, err := h.csmHandler.ResolveContainerId(target)
	if err != nil {
		http.Error(w, fmt.Sprintf("container: %s not found", containerId), http.StatusBadRequest)
		return
	}

	containerInfo, err := h.csmHandler.GetContainerById(containerId)
	if r.URL.Path == "/v1/containers/"+target+"/attach" && !containerInfo.Tty {
		http.Error(
			w,
			fmt.Sprintf(
				"container: %s has tty mode disabled.\nif attach operation needed, use -t option when create container",
				target,
			),
			http.StatusBadRequest,
		)
		return
	}

	// set log: target
	log_containerId, log_containerName, _ := h.csmHandler.GetContainerIdAndName(containerId)
	logger.SetTarget(r.Context(), logger.Target{
		ContainerId:   log_containerId,
		ContainerName: log_containerName,
	})

	up := h.Upgrader
	if up.CheckOrigin == nil {
		up.CheckOrigin = func(r *http.Request) bool { return true }
	}

	ws, err := up.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("panic in exec-attach: container=%s: %v", containerId, rec)
		}
		_ = ws.Close()
	}()

	sockPath, err := h.Resolver.ConsoleSockPath(containerId)
	if err != nil {
		_ = ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("resolve sock path failed: %v", err)))
		return
	}
	log.Printf("target socket: %s", sockPath)

	// --- Dial retry (exec-shim/attach race mitigation) ---
	unixConn, err := dialUnixWithRetry(r.Context(), sockPath, 2*time.Second, 50*time.Millisecond)
	if err != nil {
		log.Printf("dial unix sock failed (after retry): sock=%s err=%v", sockPath, err)

		// Prefer WS close control rather than plain text, so client treats it as normal closure.
		_ = ws.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseTryAgainLater, "target not ready"),
			time.Now().Add(1*time.Second),
		)
		return
	}
	defer unixConn.Close()
	// --- end retry ---

	wsr := newWSBinaryStreamReader(ws) // WS(binary messages) -> stream reader
	wsw := newWSBinaryStreamWriter(ws) // stream writer -> WS(binary messages)

	errCh := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)

	// WS -> unix (client->ptmx framed bytes)
	go func() {
		defer wg.Done()
		_, e := io.Copy(unixConn, wsr)
		errCh <- e
	}()

	// unix -> WS (ptmx raw bytes)
	go func() {
		defer wg.Done()
		_, e := io.Copy(wsw, unixConn)
		errCh <- e
	}()

	// close both session
	e := <-errCh
	log.Printf("exec-attach stream end: %v", e)
	_ = unixConn.Close()

	// send CloseMessage
	_ = ws.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, "stream closed"),
		time.Now().Add(1*time.Second),
	)

	_ = ws.Close()
	wg.Wait()

	_ = e
}

// dialUnixWithRetry tries to connect to a unix domain socket until it succeeds
// or the deadline expires. This mitigates the race where the socket path exists
// but the server process hasn't started listening yet.
func dialUnixWithRetry(ctx context.Context, sockPath string, timeout time.Duration, interval time.Duration) (net.Conn, error) {
	deadline := time.Now().Add(timeout)

	var lastErr error
	for {
		conn, err := net.Dial("unix", sockPath)
		if err == nil {
			return conn, nil
		}
		lastErr = err

		// Only retry for "not ready yet" conditions (common for unix sockets).
		// If it's some other error, bubble up quickly.
		// Note: net.Dial wraps errors; we conservatively retry on ECONNREFUSED/ENOENT.
		if !isRetryableUnixDialErr(err) {
			return nil, err
		}

		if time.Now().After(deadline) {
			return nil, lastErr
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
	}
}

func isRetryableUnixDialErr(err error) bool {
	// Common patterns:
	// - connect: connection refused (server not listening yet / stale socket)
	// - no such file or directory (path not created yet)
	if errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ENOENT) {
		return true
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		// opErr.Err may be *os.SyscallError or syscall.Errno
		if errors.Is(opErr.Err, syscall.ECONNREFUSED) || errors.Is(opErr.Err, syscall.ENOENT) {
			return true
		}
		var sysErr *os.SyscallError
		if errors.As(opErr.Err, &sysErr) {
			if errors.Is(sysErr.Err, syscall.ECONNREFUSED) || errors.Is(sysErr.Err, syscall.ENOENT) {
				return true
			}
		}
	}

	return false
}

// --- WebSocket stream adapters ---
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
			continue
		}
		r.cur = rd
	}
}

// wsBinaryStreamWriter writes stream bytes as binary WS messages.
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
