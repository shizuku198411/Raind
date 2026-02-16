package logs

import (
	"raind/internal/core/logs"

	"github.com/urfave/cli/v2"
)

func CommandLogs() *cli.Command {
	return &cli.Command{
		Name:  "netflow",
		Usage: "view netflow logs",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:  "line",
				Usage: "max line number",
			},
			&cli.BoolFlag{
				Name:  "pager",
				Usage: "open log with pager",
			},
			&cli.BoolFlag{
				Name:  "json",
				Usage: "view json format",
			},
			&cli.StringFlag{
				Name:    "target",
				Aliases: []string{"t"},
				Usage:   "filter by  target contianer or address",
			},
		},
		Action: runNetflowLog,
	}
}

func runNetflowLog(ctx *cli.Context) error {
	// option
	opt_tailLine := ctx.Int("line")
	opt_pager := ctx.Bool("pager")
	opt_json := ctx.Bool("json")

	service := logs.NewServiceNetflowLog()
	if err := service.GetLoge(logs.ServiceNetflowModel{
		TailLine: opt_tailLine,
		Pager:    opt_pager,
		JsonView: opt_json,
	}); err != nil {
		return err
	}
	return nil
}
