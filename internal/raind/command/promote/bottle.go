package promotecommand

import (
	"fmt"
	"strings"

	"raind/internal/raind/core/promote"

	"github.com/urfave/cli/v2"
)

func CommandBottle() *cli.Command {
	return &cli.Command{
		Name:      "bottle",
		Usage:     "promote a Bottlefile to Kubernetes-style resource manifests",
		ArgsUsage: "<Bottlefile>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "to",
				Usage: "promotion target format",
				Value: "resources",
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "output manifest directory",
				Value:   promote.DefaultResourcePromotionOutput,
			},
			&cli.StringFlag{
				Name:  "namespace",
				Usage: "namespace for generated resources; defaults to bottle.name",
			},
			&cli.StringFlag{
				Name:  "ingress-host",
				Usage: "generate an Ingress draft for the first TCP service port using this host",
			},
		},
		Action: runPromoteBottle,
	}
}

type promoteBottleCLIOptions struct {
	Path        string
	To          string
	Output      string
	Namespace   string
	IngressHost string
}

func runPromoteBottle(ctx *cli.Context) error {
	opts, err := parsePromoteBottleCLIOptions(ctx)
	if err != nil {
		return err
	}
	if opts.Path == "" {
		return fmt.Errorf("Bottlefile path is required")
	}
	if opts.To != "resources" {
		return fmt.Errorf("unsupported promote target %q; only %q is supported", opts.To, "resources")
	}

	draft, err := promote.BuildResourceDraftFromRunningBottleFile(opts.Path, promote.BottleToResourcesOptions{
		Namespace:   opts.Namespace,
		IngressHost: opts.IngressHost,
	})
	if err != nil {
		return err
	}
	files, err := promote.RenderResourceFiles(draft, promote.RenderResourcesOptions{
		IngressHost: opts.IngressHost,
	})
	if err != nil {
		return err
	}
	return promote.WriteResourceFiles(opts.Output, files, true)
}

func parsePromoteBottleCLIOptions(ctx *cli.Context) (promoteBottleCLIOptions, error) {
	opts := promoteBottleCLIOptions{
		Path:        ctx.Args().Get(0),
		To:          ctx.String("to"),
		Output:      ctx.String("output"),
		Namespace:   ctx.String("namespace"),
		IngressHost: ctx.String("ingress-host"),
	}

	// urfave/cli stops parsing command flags after the first positional argument.
	// Promote examples commonly put flags after the Bottlefile path, so accept the
	// same flags in the trailing argument slice as a compatibility layer:
	//
	//   raind promote bottle bottle.yaml --to resources -o manifests --ingress-host app.raind.local
	//
	args := ctx.Args().Slice()
	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--to":
			value, next, err := trailingFlagValue(args, i, arg)
			if err != nil {
				return opts, err
			}
			opts.To = value
			i = next
		case strings.HasPrefix(arg, "--to="):
			opts.To = strings.TrimPrefix(arg, "--to=")
		case arg == "-o" || arg == "--output":
			value, next, err := trailingFlagValue(args, i, arg)
			if err != nil {
				return opts, err
			}
			opts.Output = value
			i = next
		case strings.HasPrefix(arg, "--output="):
			opts.Output = strings.TrimPrefix(arg, "--output=")
		case arg == "--namespace":
			value, next, err := trailingFlagValue(args, i, arg)
			if err != nil {
				return opts, err
			}
			opts.Namespace = value
			i = next
		case strings.HasPrefix(arg, "--namespace="):
			opts.Namespace = strings.TrimPrefix(arg, "--namespace=")
		case arg == "--ingress-host":
			value, next, err := trailingFlagValue(args, i, arg)
			if err != nil {
				return opts, err
			}
			opts.IngressHost = value
			i = next
		case strings.HasPrefix(arg, "--ingress-host="):
			opts.IngressHost = strings.TrimPrefix(arg, "--ingress-host=")
		case strings.HasPrefix(arg, "-"):
			return opts, fmt.Errorf("unknown promote bottle option after Bottlefile path: %s", arg)
		default:
			return opts, fmt.Errorf("unexpected argument after Bottlefile path: %s", arg)
		}
	}

	return opts, nil
}

func trailingFlagValue(args []string, index int, flag string) (string, int, error) {
	next := index + 1
	if next >= len(args) || strings.HasPrefix(args[next], "-") {
		return "", index, fmt.Errorf("%s requires a value", flag)
	}
	return args[next], next, nil
}
