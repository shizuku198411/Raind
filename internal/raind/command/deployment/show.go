package deploymentcommand

import (
	"fmt"
	"raind/internal/raind/core/deployment"

	"github.com/urfave/cli/v2"
)

func CommandShow() *cli.Command {
	return &cli.Command{
		Name:      "show",
		Usage:     "show a deployment detail",
		Aliases:   []string{"describe"},
		ArgsUsage: "<deployment-id>",
		Action:    runShow,
	}
}

func runShow(ctx *cli.Context) error {
	id := ctx.Args().Get(0)
	if id == "" {
		return fmt.Errorf("deployment id is required")
	}

	service := deployment.NewServiceDeploymentDetail()
	return service.Detail(id)
}
