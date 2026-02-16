package servicecommand

import (
	"fmt"
	"raind/internal/core/service"

	"github.com/urfave/cli/v2"
)

func CommandCreate() *cli.Command {
	return &cli.Command{
		Name:  "create",
		Usage: "create a service from yaml",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "file",
				Aliases:  []string{"f"},
				Usage:    "service definition yaml file",
				Required: true,
			},
		},
		Action: runCreate,
	}
}

func runCreate(ctx *cli.Context) error {
	yamlPath := ctx.String("file")
	if yamlPath == "" {
		return fmt.Errorf("missing required flag: -f, --file (yaml file)")
	}

	svc := service.NewServiceServiceCreate()
	serviceId, err := svc.Create(
		service.ServiceServiceCreateModel{
			FilePath: yamlPath,
		},
	)
	if err != nil {
		return err
	}

	if serviceId == "" {
		fmt.Printf("service created\n")
		return nil
	}
	fmt.Printf("service: %s created\n", serviceId)
	return nil
}
