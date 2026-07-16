package container

import (
	"raind/internal/droplet/container/signals"
	"syscall"
)

var signalMap = signals.Map

func parseSignal(input string) (string, syscall.Signal, error) {
	return signals.Parse(input)
}
