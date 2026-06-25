package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"raind/internal/droplet/command"
	"raind/internal/droplet/logs"
	"strconv"
	"strings"
	"time"
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
			writeOCIRuntimeErrorLog(os.Args[1:], err)
			log.Print(err)
			os.Exit(1)
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
		writeOCIRuntimeErrorLog(os.Args[1:], err)
		log.Print(err)
		os.Exit(1)
	}
}

func writeOCIRuntimeErrorLog(args []string, err error) {
	logPath := flagValue(args, "--log")
	if logPath == "" {
		return
	}
	if mkErr := os.MkdirAll(filepath.Dir(logPath), 0755); mkErr != nil {
		return
	}
	f, openErr := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if openErr != nil {
		return
	}
	defer f.Close()
	record := map[string]string{
		"level": "error",
		"msg":   err.Error(),
		"time":  time.Now().Format(time.RFC3339Nano),
	}
	_ = json.NewEncoder(f).Encode(record)
}

func flagValue(args []string, name string) string {
	prefix := name + "="
	for i, arg := range args {
		if arg == name && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(arg, prefix) {
			return strings.TrimPrefix(arg, prefix)
		}
	}
	return ""
}
