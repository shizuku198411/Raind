package main

import (
	"log"
	"os"
	"raind/internal/droplet/command"
	"raind/internal/droplet/logs"
	"strconv"
	"strings"
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

func isNonInitialUserNamespace(uidMap string) bool {
	for _, line := range strings.Split(uidMap, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		containerID, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil || containerID != 0 {
			continue
		}
		hostID, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return false
		}
		return hostID != 0
	}
	return false
}

func userNamespaceDiffersFromInit(selfUIDMap string, initUIDMap string) bool {
	selfUIDMap = strings.TrimSpace(selfUIDMap)
	initUIDMap = strings.TrimSpace(initUIDMap)
	return selfUIDMap != "" && initUIDMap != "" && selfUIDMap != initUIDMap
}

func currentUserNamespaceDiffersFromInit() bool {
	selfUIDMap, err := os.ReadFile("/proc/self/uid_map")
	if err != nil {
		return false
	}
	initUIDMap, err := os.ReadFile("/proc/1/uid_map")
	if err != nil {
		return false
	}
	return userNamespaceDiffersFromInit(string(selfUIDMap), string(initUIDMap))
}

func shouldSkipAuditLoggerInit(args []string, err error) bool {
	return shouldSkipAuditLoggerInitWithNamespace(args, err, currentUserNamespaceDiffersFromInit)
}

func shouldSkipAuditLoggerInitWithNamespace(args []string, err error, inNestedUserNS func() bool) bool {
	if isInitSubcommand(args) {
		return true
	}
	return os.IsPermission(err) && inNestedUserNS()
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

	// Rootless runtime processes may run as uid/gid 0 inside a non-initial user
	// namespace, which maps to an unprivileged host uid/gid. In that state they
	// can be unable to open the host-owned droplet audit log. Do not fail those
	// child-side runtime paths just because the audit logger cannot be opened.
	if err := logs.InitAuditLogger(); err != nil {
		if !shouldSkipAuditLoggerInit(os.Args[1:], err) {
			log.Fatalf("audit logger init failed: %v", err)
		}
		log.Printf("audit logger init skipped for rootless runtime process: %v", err)
	} else {
		defer logs.AuditLogger.Close()
		logs.StartAuditLogTrimmer()
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
