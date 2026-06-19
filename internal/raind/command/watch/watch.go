package watchcommand

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/urfave/cli/v2"
)

func Flag() cli.Flag {
	return &cli.BoolFlag{
		Name:    "wait",
		Aliases: []string{"w", "watch"},
		Usage:   "repeat list output every second until interrupted",
	}
}

func Enabled(ctx *cli.Context) bool {
	for _, c := range ctx.Lineage() {
		if c.Bool("wait") {
			return true
		}
	}
	for _, arg := range ctx.Args().Slice() {
		if arg == "-w" || arg == "--wait" || arg == "--watch" {
			return true
		}
	}
	return false
}

func Run(wait bool, fn func() error) error {
	if !wait {
		return fn()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	previousLines := 0
	for {
		output, err := captureStdout(fn)
		if err != nil {
			return err
		}

		if previousLines > 0 {
			fmt.Fprintf(os.Stdout, "\r\033[%dA\r\033[J", previousLines)
		}
		fmt.Fprint(os.Stdout, output)
		previousLines = lineCount(output)

		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			fmt.Fprintln(os.Stdout)
			return nil
		case <-timer.C:
		}
	}
}

func captureStdout(fn func() error) (string, error) {
	reader, writer, err := os.Pipe()
	if err != nil {
		return "", err
	}

	readDone := make(chan struct {
		output []byte
		err    error
	}, 1)
	go func() {
		output, err := io.ReadAll(reader)
		readDone <- struct {
			output []byte
			err    error
		}{output: output, err: err}
	}()

	oldStdout := os.Stdout
	os.Stdout = writer

	runErr := fn()
	closeErr := writer.Close()
	os.Stdout = oldStdout

	result := <-readDone
	reader.Close()

	if runErr != nil {
		return string(result.output), runErr
	}
	if closeErr != nil {
		return string(result.output), closeErr
	}
	if result.err != nil {
		return string(result.output), result.err
	}
	return string(result.output), nil
}

func lineCount(output string) int {
	trimmed := strings.TrimSuffix(output, "\n")
	if trimmed == "" {
		return 1
	}
	return strings.Count(trimmed, "\n") + 1
}
