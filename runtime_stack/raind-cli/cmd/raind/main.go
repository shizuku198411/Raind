package main

import (
	"log"
	"os"
	"raind/internal/command"
)

func main() {
	app := command.NewApp()

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
