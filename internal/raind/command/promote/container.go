package promotecommand

import (
	"fmt"
	"os"
	"raind/internal/raind/core/container"
	"raind/internal/raind/core/promote"

	"github.com/urfave/cli/v2"
)

func CommandContainer() *cli.Command {
	return &cli.Command{
		Name:      "container",
		Usage:     "promote a container to a Dripfile draft",
		ArgsUsage: "<id|name>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "to",
				Usage: "promotion target format",
				Value: "bottle",
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "output Dripfile path",
				Value:   "Dripfile",
			},
			&cli.StringFlag{
				Name:  "service-name",
				Usage: "service name in the generated Dripfile",
				Value: "app",
			},
			&cli.StringFlag{
				Name:  "bottle-name",
				Usage: "bottle name in the generated Dripfile",
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
				Usage: "write generated Dripfile to stdout instead of a file",
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
	if opts.Target == "" {
		return fmt.Errorf("container id or name is required")
	}
	if opts.To != "bottle" {
		return fmt.Errorf("unsupported promote target %q; only %q is supported", opts.To, "bottle")
	}

	inspectService := container.NewServiceContainerInspect()
	inspect, err := inspectService.Get(opts.Target)
	if err != nil {
		return err
	}

	draft, err := promote.BuildBottleDraftFromContainer(inspect, promote.ContainerToBottleOptions{
		BottleName:      opts.BottleName,
		ServiceName:     opts.ServiceName,
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
	To              string
	Output          string
	ServiceName     string
	BottleName      string
	IncludeImageEnv bool
	Force           bool
	Stdout          bool
}

func parseContainerOptions(ctx *cli.Context) (containerOptions, error) {
	opts := containerOptions{
		Target:          ctx.Args().Get(0),
		To:              ctx.String("to"),
		Output:          ctx.String("output"),
		ServiceName:     ctx.String("service-name"),
		BottleName:      ctx.String("bottle-name"),
		IncludeImageEnv: ctx.Bool("include-image-env"),
		Force:           ctx.Bool("force"),
		Stdout:          ctx.Bool("stdout"),
	}
	args := ctx.Args().Slice()
	if len(args) <= 1 {
		return opts, nil
	}
	for i := 1; i < len(args); i++ {
		arg := args[i]
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
				continue
			}
			if value, ok := trimPostArgPrefix(arg, "--bottle-name="); ok {
				opts.BottleName = value
				continue
			}
			return opts, fmt.Errorf("unexpected argument: %s", arg)
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
