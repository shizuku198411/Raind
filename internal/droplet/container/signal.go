package container

import (
	"fmt"
	"strconv"
	"strings"
	"syscall"
)

var signalMap = map[string]syscall.Signal{
	"TERM": syscall.SIGTERM,
	"KILL": syscall.SIGKILL,
	"INT":  syscall.SIGINT,
	"HUP":  syscall.SIGHUP,
	"QUIT": syscall.SIGQUIT,
	"USR1": syscall.SIGUSR1,
	"USR2": syscall.SIGUSR2,
	"STOP": syscall.SIGSTOP,
	"CONT": syscall.SIGCONT,
}

func parseSignal(input string) (string, syscall.Signal, error) {
	name := strings.TrimSpace(input)
	if name == "" {
		name = "TERM"
	}
	if n, err := strconv.Atoi(name); err == nil {
		if n < 0 {
			return "", 0, fmt.Errorf("invalid signal: %s", input)
		}
		return strconv.Itoa(n), syscall.Signal(n), nil
	}

	name = strings.ToUpper(name)
	name = strings.TrimPrefix(name, "SIG")
	sig, ok := signalMap[name]
	if !ok {
		return "", 0, fmt.Errorf("unsupported signal: %s", input)
	}
	return name, sig, nil
}
