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

func isInitSubcommand(args []string) bool {
	return len(args) > 0 && args[0] == "init"
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

	// The rootless container init process starts as uid/gid 0 inside the user
	// namespace, which maps to an unprivileged host uid/gid. In that state it may
	// be unable to open the host-owned droplet audit log. Do not fail init just
	// because the audit logger cannot be opened; the parent create command still
	// records the create failure/success audit event.
	if err := logs.InitAuditLogger(); err != nil {
		if !isInitSubcommand(os.Args[1:]) {
			log.Fatalf("audit logger init failed: %v", err)
		}
		log.Printf("audit logger init skipped for init process: %v", err)
	} else {
		defer logs.AuditLogger.Close()
		logs.StartAuditLogTrimmer()
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
