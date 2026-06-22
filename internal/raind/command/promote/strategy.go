package promotecommand

import (
	"fmt"
	"os"
	"strings"

	"raind/internal/raind/core/promote"

	"github.com/urfave/cli/v2"
)

func CommandStrategy() *cli.Command {
	return &cli.Command{
		Name:  "strategy",
		Usage: "run a promote strategy workflow from raind-strategy.yaml",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "file",
				Aliases: []string{"f"},
				Usage:   "strategy yaml file",
				Value:   promote.DefaultStrategyFile,
			},
			&cli.StringFlag{
				Name:  "until",
				Usage: "stop after a stage: container, bottle-draft, bottle, resources-draft",
			},
			&cli.BoolFlag{
				Name:  "dry-run",
				Usage: "validate and print the strategy plan without executing it",
				Value: false,
			},
			&cli.StringFlag{
				Name:  "namespace",
				Usage: "namespace for generated resources; defaults to metadata.name",
			},
			&cli.StringFlag{
				Name:  "ingress-host",
				Usage: "generate an Ingress draft for the first TCP service port using this host",
			},
		},
		Action: runPromoteStrategy,
	}
}

func runPromoteStrategy(ctx *cli.Context) error {
	dryRun := ctx.Bool("dry-run")
	progressRenderer := newStrategyProgressRenderer(os.Stdout)
	logRenderer := newStrategyInternalLogRenderer(os.Stdout)
	result, err := promote.RunStrategyFile(ctx.String("file"), promote.StrategyOptions{
		File:        ctx.String("file"),
		Until:       ctx.String("until"),
		DryRun:      dryRun,
		Namespace:   ctx.String("namespace"),
		IngressHost: ctx.String("ingress-host"),
		ProgressStart: func(name string) {
			if !dryRun {
				fmt.Printf("Promote Strategy: %s\n", name)
			}
		},
		Progress: func(event promote.StrategyProgressEvent) {
			if !dryRun {
				logRenderer.Clear()
				progressRenderer.Handle(event)
			}
		},
		InternalLog: func(event promote.StrategyInternalLogEvent) {
			if !dryRun {
				logRenderer.Handle(event)
			}
		},
	})
	logRenderer.Clear()
	progressRenderer.ClearActive()
	if err != nil {
		return err
	}
	printStrategyResult(result, dryRun)
	return nil
}

type strategyInternalLogRenderer struct {
	out     *os.File
	lines   []string
	visible int
}

func newStrategyInternalLogRenderer(out *os.File) *strategyInternalLogRenderer {
	return &strategyInternalLogRenderer{out: out}
}

func (r *strategyInternalLogRenderer) Handle(event promote.StrategyInternalLogEvent) {
	if event.Done {
		r.Clear()
		return
	}
	line := strings.TrimSpace(event.Line)
	if line == "" {
		return
	}
	r.lines = append(r.lines, line)
	if len(r.lines) > 5 {
		r.lines = r.lines[len(r.lines)-5:]
	}
	r.redraw()
}

func (r *strategyInternalLogRenderer) Clear() {
	if r == nil || r.visible == 0 {
		return
	}
	for i := 0; i < r.visible; i++ {
		fmt.Fprint(r.out, "\033[F\033[2K")
	}
	r.visible = 0
	r.lines = nil
	_ = r.out.Sync()
}

func (r *strategyInternalLogRenderer) redraw() {
	if r == nil {
		return
	}
	for i := 0; i < r.visible; i++ {
		fmt.Fprint(r.out, "\033[F\033[2K")
	}
	for _, line := range r.lines {
		fmt.Fprintf(r.out, "\033[2;36m%s\033[0m\n", line)
	}
	r.visible = len(r.lines)
	_ = r.out.Sync()
}

type strategyProgressRenderer struct {
	out       *os.File
	active    bool
	activeKey string
}

func newStrategyProgressRenderer(out *os.File) *strategyProgressRenderer {
	return &strategyProgressRenderer{out: out}
}

func (r *strategyProgressRenderer) Handle(event promote.StrategyProgressEvent) {
	if r == nil {
		return
	}
	line := formatStrategyProgressLine(event)
	if !event.Done {
		r.ClearActive()
		fmt.Fprintf(r.out, "%s\n", line)
		r.active = true
		r.activeKey = strings.TrimSpace(event.Name)
		_ = r.out.Sync()
		return
	}

	if r.active && r.activeKey == strings.TrimSpace(event.Name) {
		fmt.Fprint(r.out, "\033[F\033[2K")
		r.active = false
		r.activeKey = ""
	}
	fmt.Fprintf(r.out, "%s\n", line)
	_ = r.out.Sync()
}

func (r *strategyProgressRenderer) ClearActive() {
	if r == nil || !r.active {
		return
	}
	fmt.Fprint(r.out, "\033[F\033[2K")
	r.active = false
	r.activeKey = ""
	_ = r.out.Sync()
}

func formatStrategyProgressLine(event promote.StrategyProgressEvent) string {
	stage, task := formatStrategyProgressName(event.Name)
	base := fmt.Sprintf("[%s] %s ...", stage, task)
	if !event.Done {
		return base
	}
	status := strings.TrimSpace(event.Status)
	if status == "" {
		status = "ok"
	}
	return base + " " + status
}

func formatStrategyProgressName(name string) (string, string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "strategy", "step"
	}
	stage, task, ok := strings.Cut(name, "/")
	if !ok {
		return "strategy", name
	}
	stage = strings.TrimSpace(stage)
	task = strings.TrimSpace(task)
	if stage == "" {
		stage = "strategy"
	}
	if task == "" {
		task = "step"
	}
	return stage, task
}

func printStrategyResult(result promote.StrategyRunResult, dryRun bool) {
	if dryRun {
		fmt.Printf("Promote Strategy: %s\n", result.Name)
		fmt.Println("Plan validated")
		return
	}
	if result.BottleOutput != "" {
		fmt.Printf("bottle draft: %s\n", result.BottleOutput)
	}
	if result.ComposeOutput != "" {
		fmt.Printf("compose draft: %s\n", result.ComposeOutput)
	}
	if result.ResourcesOutput != "" {
		fmt.Printf("resource drafts: %s\n", result.ResourcesOutput)
	}
}
