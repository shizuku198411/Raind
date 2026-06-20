package promotecommand

import (
	"fmt"
	"os"
	"raind/internal/raind/core/container"
	"raind/internal/raind/core/promote"
	"strings"

	"github.com/urfave/cli/v2"
)

func CommandContainer() *cli.Command {
	return &cli.Command{
		Name:      "container",
		Usage:     "promote a container to a bottle.yaml draft",
		ArgsUsage: "<id|name> [id|name ...]",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "to",
				Usage: "promotion target format",
				Value: "bottle",
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "output bottle.yaml path",
				Value:   "bottle.yaml",
			},
			&cli.StringFlag{
				Name:  "service-name",
				Usage: "service name in the generated bottle.yaml",
				Value: "app",
			},
			&cli.StringFlag{
				Name:  "bottle-name",
				Usage: "bottle name in the generated bottle.yaml",
			},
			&cli.BoolFlag{
				Name:  "include-image-env",
				Usage: "include common image-provided environment variables such as PATH",
				Value: false,
			},
			&cli.BoolFlag{
				Name:  "force",
				Usage: "overwrite output file if it already exists",
				Value: false,
			},
			&cli.BoolFlag{
				Name:  "stdout",
				Usage: "write generated bottle.yaml to stdout instead of a file",
				Value: false,
			},
		},
		Action: runPromoteContainer,
	}
}

func runPromoteContainer(ctx *cli.Context) error {
	opts, err := parseContainerOptions(ctx)
	if err != nil {
		return err
	}
	if len(opts.Targets) == 0 {
		return fmt.Errorf("container id or name is required")
	}
	if len(opts.Targets) > 1 && opts.ServiceNameSet {
		return fmt.Errorf("--service-name can only be used when promoting a single container")
	}
	if opts.To != "bottle" {
		return fmt.Errorf("unsupported promote target %q; only %q is supported", opts.To, "bottle")
	}

	inspectService := container.NewServiceContainerInspect()
	inspects := make([]container.ContainerInspectModel, 0, len(opts.Targets))
	for _, target := range opts.Targets {
		inspect, err := inspectService.Get(target)
		if err != nil {
			return err
		}
		inspects = append(inspects, inspect)
	}

	serviceName := opts.ServiceName
	if len(opts.Targets) > 1 && !opts.ServiceNameSet {
		serviceName = ""
	}
	draft, err := promote.BuildBottleDraftFromContainers(inspects, promote.ContainerToBottleOptions{
		BottleName:      opts.BottleName,
		ServiceName:     serviceName,
		IncludeImageEnv: opts.IncludeImageEnv,
	})
	if err != nil {
		return err
	}

	data, err := promote.RenderBottlefile(draft)
	if err != nil {
		return err
	}
	if opts.Stdout {
		_, err = os.Stdout.Write(data)
		return err
	}
	return promote.WriteOutput(opts.Output, data, opts.Force)
}

type containerOptions struct {
	Target          string
	Targets         []string
	To              string
	Output          string
	ServiceName     string
	ServiceNameSet  bool
	BottleName      string
	IncludeImageEnv bool
	Force           bool
	Stdout          bool
}

func parseContainerOptions(ctx *cli.Context) (containerOptions, error) {
	opts := containerOptions{
		Target:          ctx.Args().Get(0),
		Targets:         collectInitialTargets(ctx.Args().Slice()),
		To:              ctx.String("to"),
		Output:          ctx.String("output"),
		ServiceName:     ctx.String("service-name"),
		ServiceNameSet:  ctx.IsSet("service-name"),
		BottleName:      ctx.String("bottle-name"),
		IncludeImageEnv: ctx.Bool("include-image-env"),
		Force:           ctx.Bool("force"),
		Stdout:          ctx.Bool("stdout"),
	}
	args := ctx.Args().Slice()
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if i == 0 && opts.Target == arg {
			continue
		}
		switch arg {
		case "--to":
			value, next, err := requirePostArgValue(args, i, arg)
			if err != nil {
				return opts, err
			}
			opts.To = value
			i = next
		case "-o", "--output":
			value, next, err := requirePostArgValue(args, i, arg)
			if err != nil {
				return opts, err
			}
			opts.Output = value
			i = next
		case "--service-name":
			value, next, err := requirePostArgValue(args, i, arg)
			if err != nil {
				return opts, err
			}
			opts.ServiceName = value
			opts.ServiceNameSet = true
			i = next
		case "--bottle-name":
			value, next, err := requirePostArgValue(args, i, arg)
			if err != nil {
				return opts, err
			}
			opts.BottleName = value
			i = next
		case "--include-image-env":
			opts.IncludeImageEnv = true
		case "--force":
			opts.Force = true
		case "--stdout":
			opts.Stdout = true
		default:
			if value, ok := trimPostArgPrefix(arg, "--to="); ok {
				opts.To = value
				continue
			}
			if value, ok := trimPostArgPrefix(arg, "--output="); ok {
				opts.Output = value
				continue
			}
			if value, ok := trimPostArgPrefix(arg, "-o="); ok {
				opts.Output = value
				continue
			}
			if value, ok := trimPostArgPrefix(arg, "--service-name="); ok {
				opts.ServiceName = value
				opts.ServiceNameSet = true
				continue
			}
			if value, ok := trimPostArgPrefix(arg, "--bottle-name="); ok {
				opts.BottleName = value
				continue
			}
			if strings.HasPrefix(arg, "-") {
				return opts, fmt.Errorf("unexpected argument: %s", arg)
			}
			if arg != "" {
				opts.Targets = appendUnique(opts.Targets, arg)
			}
			continue
		}
	}
	return opts, nil
}

func requirePostArgValue(args []string, index int, name string) (string, int, error) {
	next := index + 1
	if next >= len(args) || args[next] == "" {
		return "", index, fmt.Errorf("%s requires a value", name)
	}
	return args[next], next, nil
}

func trimPostArgPrefix(arg, prefix string) (string, bool) {
	if len(arg) <= len(prefix) || arg[:len(prefix)] != prefix {
		return "", false
	}
	return arg[len(prefix):], true
}

func collectInitialTargets(args []string) []string {
	targets := []string{}
	for _, arg := range args {
		if arg == "" || strings.HasPrefix(arg, "-") {
			break
		}
		targets = appendUnique(targets, arg)
	}
	return targets
}

func appendUnique(items []string, value string) []string {
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}
