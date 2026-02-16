package policy

import (
	"fmt"
	"raind/internal/core/policy"

	"github.com/urfave/cli/v2"
)

func CommandCommit() *cli.Command {
	return &cli.Command{
		Name:   "commit",
		Usage:  "commit policy",
		Action: runCommit,
	}
}

func runCommit(ctx *cli.Context) error {
	// user confirm
	fmt.Print("This operation will affect the container network.\nAre you sure you want to commit? (y/n): ")
	var confirm string
	fmt.Scan(&confirm)
	if confirm == "y" || confirm == "Y" || confirm == "yes" || confirm == "Yes" || confirm == "YES" {
		service := policy.NewServicePolicyCommit()
		err := service.Commit()
		if err != nil {
			return err
		}
	} else {
		fmt.Println("commit canceled")
	}

	return nil
}
