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
				printStrategyProgress(event)
			}
		},
	})
	if err != nil {
		return err
	}
	printStrategyResult(result, dryRun)
	return nil
}

func printStrategyProgress(event promote.StrategyProgressEvent) {
	status := strings.TrimSpace(event.Status)
	if status == "" {
		status = "ok"
	}
	total := event.Total
	if total <= 0 {
		total = event.Index
	}
	fmt.Printf("[%d/%d] %s ... %s\n", event.Index, total, event.Name, status)
	_ = os.Stdout.Sync()
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
	if result.ResourcesOutput != "" {
		fmt.Printf("resource drafts: %s\n", result.ResourcesOutput)
	}
}
