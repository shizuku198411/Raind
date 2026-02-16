package container

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	httpclient "raind/internal/core/client"
	"raind/internal/utils"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/term"
)

func NewServiceContainerExec() *ServiceContainerExec {
	return &ServiceContainerExec{}
}

type ServiceContainerExec struct{}

func (c *ServiceContainerExec) Exec(param ServiceExecModel) error {
	// request body
	requestBody, err := json.Marshal(
		ExecRequestModel{
			Command: param.Command,
			Tty:     param.Tty,
		},
	)
	if err != nil {
		return err
	}

	httpClient := httpclient.NewHttpClient()
	if httpClient == nil {
		return fmt.Errorf("sudo required")
	}
	httpClient.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/v1/containers/%s/actions/exec", param.ContainerId),
		requestBody,
	)

	resp, err := httpClient.Client.Do(httpClient.Request)
	if err != nil {
		return fmt.Errorf("Cannot connect to the Raind daemon. Is the raind daemon running?")
	}
	defer resp.Body.Close()

	var respModel ExecResponseModel

	if !httpClient.IsStatusOk(resp) {
		decodeErr := json.NewDecoder(resp.Body).Decode(&respModel)
		if decodeErr != nil {
			return fmt.Errorf("decode response: %w", decodeErr)
		}
		return fmt.Errorf("unexpected status: %s: %s", resp.Status, respModel.Message)
	}

	// attach if -t provided
	if param.Tty {
		if err := c.attach(param.ContainerId); err != nil {
			return err
		}
	}

	return nil
}

// Attach connects to Condenser websocket endpoint and attaches to container TTY.
func (c *ServiceContainerExec) attach(containerId string) error {
	wsURL := fmt.Sprintf("wss://localhost:7755/v1/containers/%s/exec/attach", containerId)
	u, err := url.Parse(wsURL)
	if err != nil {
		return fmt.Errorf("parse ws url: %w", err)
	}

	// Dial websocket
	httpClient := httpclient.NewHttpClient()
	if httpClient == nil {
		return fmt.Errorf("sudo required")
	}
	dialer, err := httpClient.NewMTLSDialer(
		utils.PublicCertPath,
		utils.ClientCertPath,
		utils.ClientKeyPath,
	)
	if err != nil {
		return err
	}
	ws, _, err := dialer.Dial(u.String(), http.Header{})
	if err != nil {
		return fmt.Errorf("Cannot connect to the Raind daemon. Is the raind daemon running?")
	}
	defer ws.Close()

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
