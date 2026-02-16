package main

import (
	"droplet/internal/command"
	"droplet/internal/logs"
	"log"
	"os"
)

func main() {
	// init logger
	if err := logs.InitAuditLogger(); err != nil {
		log.Fatalf("audit logger init failed: %v", err)
	}
	defer logs.AuditLogger.Close()
	logs.StartAuditLogTrimmer()

	app := command.NewApp()

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
