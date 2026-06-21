package bottle

import (
	"fmt"
	"os"

	corebottle "raind/internal/raind/core/bottle"

	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

const (
	defaultBottleFile  = "bottle.yaml"
	defaultComposeFile = "compose.yaml"
)

var defaultBottleFiles = []string{defaultBottleFile, defaultComposeFile}

type bottleNameOnlySpec struct {
	Bottle struct {
		Name string `yaml:"name"`
	} `yaml:"bottle"`
}

func CommandUp() *cli.Command {
	return &cli.Command{
		Name:  "up",
		Usage: "create and start a bottle from yaml",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "file",
				Aliases: []string{"f"},
				Usage:   "yaml file path; defaults to bottle.yaml, then compose.yaml",
			},
		},
		Action: runUp,
	}
}

func CommandDown() *cli.Command {
	return &cli.Command{
		Name:  "down",
		Usage: "stop and remove a bottle described by yaml",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "file",
				Aliases: []string{"f"},
				Usage:   "yaml file path; defaults to bottle.yaml, then compose.yaml",
			},
		},
		Action: runDown,
	}
}

func runUp(ctx *cli.Context) error {
	filePath, err := resolveBottleFile(ctx.String("file"))
	if err != nil {
		return err
	}

	yamlBytes, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	createService := corebottle.NewServiceBottleCreate()
	respData, err := createService.Create(
		corebottle.ServiceBottleCreateModel{
			Yaml: yamlBytes,
		},
	)
	if err != nil {
		return fmt.Errorf("bottle create failed: %w", err)
	}

	fmt.Printf("bottle: %s created\n", respData.BottleName)

	startService := corebottle.NewServiceBottleStart()
	if err := startService.Start(
		corebottle.ServiceBottleStartModel{
			Target: respData.BottleName,
		},
	); err != nil {
		return fmt.Errorf("bottle start failed: %w", err)
	}

	return nil
}

func runDown(ctx *cli.Context) error {
	filePath, err := resolveBottleFile(ctx.String("file"))
	if err != nil {
		return err
	}

	name, err := readBottleNameFromFile(filePath)
	if err != nil {
		return err
	}

	stopService := corebottle.NewServiceBottleStop()
	if err := stopService.Stop(
		corebottle.ServiceBottleStopModel{
			Target: name,
		},
	); err != nil {
		return fmt.Errorf("bottle stop failed: %w", err)
	}

	deleteService := corebottle.NewServiceBottleDelete()
	if err := deleteService.Delete(
		corebottle.ServiceBottleDeleteModel{
			Target: name,
		},
	); err != nil {
		return fmt.Errorf("bottle remove failed: %w", err)
	}

	return nil
}

func resolveBottleFile(explicit string) (string, error) {
	if explicit != "" {
		if _, err := os.Stat(explicit); err != nil {
			return "", fmt.Errorf("bottle file %q is not readable: %w", explicit, err)
		}
		return explicit, nil
	}

	for _, candidate := range defaultBottleFiles {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("bottle file is required: use -f or create %s / %s", defaultBottleFile, defaultComposeFile)
}

func readBottleNameFromFile(filePath string) (string, error) {
	yamlBytes, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	var spec bottleNameOnlySpec
	if err := yaml.Unmarshal(yamlBytes, &spec); err != nil {
		return "", fmt.Errorf("parse bottle file failed: %w", err)
	}
	if spec.Bottle.Name == "" {
		return "", fmt.Errorf("bottle.name is required in %s", filePath)
	}

	return spec.Bottle.Name, nil
}
