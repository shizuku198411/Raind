package main

import (
	"log"
	"os"
	"raind/internal/droplet/command"
	"raind/internal/droplet/logs"
)

func isVersionArg(args []string) bool {
	for _, arg := range args {
		if arg == "--version" || arg == "-v" {
			return true
		}
	}
	return false
}

func main() {
	app := command.NewApp()

	// For version output, skip audit logger initialization.
	if isVersionArg(os.Args[1:]) {
		if err := app.Run(os.Args); err != nil {
			log.Fatal(err)
		}
		return
	}

	// init logger
	if err := logs.InitAuditLogger(); err != nil {
		log.Fatalf("audit logger init failed: %v", err)
	}
	defer logs.AuditLogger.Close()
	logs.StartAuditLogTrimmer()

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
