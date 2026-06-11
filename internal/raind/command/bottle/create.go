package bottle

import (
	"fmt"
	"os"
	"raind/internal/raind/core/bottle"

	"github.com/urfave/cli/v2"
)

func CommandCreate() *cli.Command {
	return &cli.Command{
		Name:      "create",
		Usage:     "create a bottle from yaml",
		ArgsUsage: "",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "file",
				Aliases:  []string{"f"},
				Usage:    "yaml file path",
				Required: true,
			},
		},
		Action: runCreate,
	}
}

func runCreate(ctx *cli.Context) error {
	filePath := ctx.String("file")
	if filePath == "" {
		return fmt.Errorf("file path is required")
	}

	yamlBytes, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	service := bottle.NewServiceBottleCreate()
	respData, err := service.Create(
		bottle.ServiceBottleCreateModel{
			Yaml: yamlBytes,
		},
	)
	if err != nil {
		return fmt.Errorf("bottle create failed: %w", err)
	}

	fmt.Printf("bottle: %s created\n", respData.BottleName)

	return nil
}
