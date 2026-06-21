package promotecommand

import (
	"fmt"
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
	result, err := promote.RunStrategyFile(ctx.String("file"), promote.StrategyOptions{
		File:        ctx.String("file"),
		Until:       ctx.String("until"),
		DryRun:      ctx.Bool("dry-run"),
		Namespace:   ctx.String("namespace"),
		IngressHost: ctx.String("ingress-host"),
	})
	if err != nil {
		return err
	}
	printStrategyResult(result, ctx.Bool("dry-run"))
	return nil
}

func printStrategyResult(result promote.StrategyRunResult, dryRun bool) {
	if dryRun {
		fmt.Printf("Promote Strategy: %s\n", result.Name)
		fmt.Println("Plan validated")
		return
	}
	fmt.Printf("Promote Strategy: %s\n", result.Name)
	for i, step := range result.Steps {
		status := strings.TrimSpace(step.Status)
		if status == "" {
			status = "ok"
		}
		fmt.Printf("[%d/%d] %s ... %s\n", i+1, len(result.Steps), step.Name, status)
	}
	if result.BottleOutput != "" {
		fmt.Printf("bottle draft: %s\n", result.BottleOutput)
	}
	if result.ResourcesOutput != "" {
		fmt.Printf("resource drafts: %s\n", result.ResourcesOutput)
	}
}
