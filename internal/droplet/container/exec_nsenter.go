package container

import (
	"fmt"
	"path/filepath"
	"raind/internal/droplet/spec"
	"strings"

	"golang.org/x/sys/unix"
)

func buildExecNsenterCommand(containerPid string, containerSpec spec.Spec) []string {
	// Container init performs pivot_root inside the container mount namespace.
	// nsenter joins namespaces but keeps the caller's root/cwd unless instructed
	// otherwise, so exec must also adopt the target process root and the OCI cwd.
	// Use explicit namespace flags instead of --all so rootless user-namespace
	// credential switching can be ordered clearly and so future namespace types do
	// not alter exec behavior unexpectedly.
	args := []string{"nsenter", "-t", containerPid}
	if isRootlessSpec(containerSpec) {
		args = append(args, "-U", "--setuid", "0", "--setgid", "0")
	}
	args = append(args,
		"-m",     // mount namespace: container rootfs and mounts
		"-u",     // UTS namespace: hostname/domain
		"-i",     // IPC namespace
		"-n",     // network namespace
		"-p",     // PID namespace; nsenter forks so the command is inside it
		"-C",     // cgroup namespace
		"--root", // switch to the target process root
		"--wd="+execWorkingDir(containerSpec),
		"--",
	)
	return args
}

func execWorkingDir(containerSpec spec.Spec) string {
	if containerSpec.Process.Cwd == "" {
		return "/"
	}
	return containerSpec.Process.Cwd
}

func resolveExecEntrypoint(containerPid string, containerSpec spec.Spec, entrypoint []string) ([]string, error) {
	if len(entrypoint) == 0 {
		return nil, fmt.Errorf("exec command required")
	}

	resolved := append([]string(nil), entrypoint...)
	arg0 := resolved[0]
	if strings.Contains(arg0, "/") {
		return resolved, nil
	}

	pathVal := execPathEnv(containerSpec.Process.Env)
	root := fmt.Sprintf("/proc/%s/root", containerPid)
	for _, dir := range strings.Split(pathVal, ":") {
		if dir == "" {
			dir = "."
		}
		containerPath := filepath.Join(dir, arg0)
		hostPath := filepath.Join(root, strings.TrimPrefix(containerPath, "/"))
		if err := unix.Access(hostPath, unix.X_OK); err == nil {
			resolved[0] = containerPath
			return resolved, nil
		}
	}

	return nil, fmt.Errorf("%s: not found in container PATH %q", arg0, pathVal)
}

func execPathEnv(env []string) string {
	for _, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			return strings.TrimPrefix(e, "PATH=")
		}
	}
	return "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
}
