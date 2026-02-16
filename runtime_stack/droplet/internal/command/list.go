package command

import (
	"droplet/internal/status"
	"encoding/json"
	"fmt"

	"github.com/urfave/cli/v2"
)

func commandList() *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "lists containers",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "format",
				Usage: "print format [default|json]",
			},
		},
		Action: runList,
	}
}

func runList(ctx *cli.Context) error {
	// format option
	formatOption := ctx.String("format")

	containerStatusHandler := status.NewStatusHandler()

	// read state.json
	containerStatusList, err := containerStatusHandler.ListContainers()
	if err != nil {
		return err
	}

	printList(containerStatusList, formatOption)

	return nil
}

func printList(list []status.StatusObject, format string) {

	if format == "json" {
		dataStr, err := json.Marshal(list)
		if err != nil {
			return
		}
		fmt.Print(string(dataStr))
	} else {
		fmt.Printf("%-15s %-15s %-8s %-s\n", "ID", "STATUS", "PID", "BUNDLE")
		for _, entry := range list {
			fmt.Printf("%-15s %-15s %-8d %-s\n", entry.Id, entry.Status, entry.Pid, entry.Bundle)
		}
	}
}
