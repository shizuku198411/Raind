package image

import (
	"archive/tar"
	"condenser/internal/runtime"
	"condenser/internal/runtime/droplet"
	"condenser/internal/store/csm"
	"condenser/internal/store/ipam"
	"condenser/internal/utils"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"al.essio.dev/pkg/shellescape"
)

type buildState struct {
	imageRepo  string
	imageRef   string
	rootfsPath string

	env        []string
	workdir    string
	cmd        []string
	entrypoint []string
	runScript  []string
}

type buildInstruction struct {
	op   string
	args string
}

// == service: build image ==
func (s *ImageService) Build(buildParameter ServiceBuildModel) (string, error) {
	if buildParameter.Image == "" {
		return "", errors.New("image tag is required")
	}
	if buildParameter.ContextDir == "" {
		return "", errors.New("context dir is required")
	}
	if buildParameter.DripfilePath == "" {
		return "", errors.New("dripfile path is required")
	}
	bridge := buildParameter.Network
	if bridge == "" {
		bridge = "raind0"
	}

	// parse and validate dripfile
	instructions, err := parseDripfile(buildParameter.DripfilePath)
	if err != nil {
		return "", err
	}

	state := buildState{
		workdir: "/",
	}
	defer func() {
		if state.rootfsPath != "" {
			_ = os.RemoveAll(state.rootfsPath)
		}
	}()

	for _, ins := range instructions {
		switch ins.op {
		case "FROM":
			if state.imageRepo != "" {
				return "", errors.New("multi-stage build is not supported")
			}
			if err := s.applyFrom(&state, ins.args); err != nil {
				return "", err
			}
		case "WORKDIR":
			if err := s.applyWorkdir(&state, ins.args); err != nil {
				return "", err
			}
		case "ENV":
			if err := s.applyEnv(&state, ins.args); err != nil {
				return "", err
			}
		case "COPY", "ADD":
			if err := s.applyCopy(&state, buildParameter.ContextDir, ins.args); err != nil {
				return "", err
			}
		case "RUN":
			if err := s.applyRun(&state, ins.args); err != nil {
				return "", err
			}
		case "CMD":
			if err := s.applyCmd(&state, ins.args); err != nil {
				return "", err
			}
		case "ENTRYPOINT":
			if err := s.applyEntrypoint(&state, ins.args); err != nil {
				return "", err
			}
		default:
			return "", fmt.Errorf("unsupported instruction: %s", ins.op)
		}
	}

	if state.rootfsPath == "" {
		return "", errors.New("missing FROM instruction")
	}

	if len(state.runScript) > 0 {
		if err := s.runCommandInContainer(&state, bridge, state.runScript); err != nil {
			return "", err
		}
	}

	imageRepo, imageRef, err := s.parseImageRef(buildParameter.Image)
	if err != nil {
		return "", err
	}

	if err := s.storeBuiltImage(imageRepo, imageRef, state); err != nil {
		return "", err
	}
	return imageRepo + ":" + imageRef, nil
}

func (s *ImageService) applyFrom(state *buildState, image string) error {
	image = strings.TrimSpace(strings.Fields(image)[0])
	if image == "" {
		return errors.New("FROM requires image")
	}

	imageRepo, imageRef, err := s.parseImageRef(image)
	if err != nil {
		return err
	}

	if !s.ilmHandler.IsImageExist(imageRepo, imageRef) {
		if err := s.Pull(ServicePullModel{Image: image}); err != nil {
			return err
		}
	}

	configPath, err := s.ilmHandler.GetConfigPath(imageRepo, imageRef)
	if err != nil {
		return err
	}
	imageConfig, err := s.GetImageConfig(configPath)
	if err != nil {
		return err
	}

	baseRootfs, err := s.ilmHandler.GetRootfsPath(imageRepo, imageRef)
	if err != nil {
		return err
	}

	tmpRootfs, err := os.MkdirTemp("", "raind-build-rootfs-")
	if err != nil {
		return err
	}
	if err := copyDir(baseRootfs, tmpRootfs); err != nil {
		return err
	}

	state.imageRepo = imageRepo
	state.imageRef = imageRef
	state.rootfsPath = tmpRootfs
	state.env = cloneSlice(imageConfig.Config.Env)
	state.workdir = imageConfig.Config.WorkingDir
	if state.workdir == "" {
		state.workdir = "/"
	}
	state.cmd = cloneSlice(imageConfig.Config.Cmd)
	state.entrypoint = cloneSlice(imageConfig.Config.Entrypoint)
	return nil
}

func (s *ImageService) applyWorkdir(state *buildState, arg string) error {
	dir := strings.TrimSpace(arg)
	if dir == "" {
		return errors.New("WORKDIR requires path")
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(state.workdir, dir)
	}
	state.workdir = filepath.Clean(dir)
	target := filepath.Join(state.rootfsPath, strings.TrimPrefix(state.workdir, "/"))
	return os.MkdirAll(target, 0o755)
}

func (s *ImageService) applyEnv(state *buildState, arg string) error {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return errors.New("ENV requires key=value")
	}
	parts := strings.Fields(arg)
	for i := 0; i < len(parts); i++ {
		if strings.Contains(parts[i], "=") {
			kv := strings.SplitN(parts[i], "=", 2)
			state.env = setEnvVar(state.env, kv[0], kv[1])
			continue
		}
		if i+1 >= len(parts) {
			return errors.New("ENV requires key value")
		}
		state.env = setEnvVar(state.env, parts[i], parts[i+1])
		i++
	}
	return nil
}

func (s *ImageService) applyCopy(state *buildState, contextDir string, arg string) error {
	parts := strings.Fields(arg)
	if len(parts) < 2 {
		return errors.New("COPY/ADD requires src and dest")
	}
	if len(parts) > 2 {
		return errors.New("COPY/ADD multiple sources not supported")
	}
	src := parts[0]
	dst := parts[1]
	if src == "" || dst == "" {
		return errors.New("COPY/ADD requires src and dest")
	}
	srcPath, err := safeJoin(contextDir, src)
	if err != nil {
		return err
	}
	dstAbs := dst
	if !filepath.IsAbs(dstAbs) {
		dstAbs = filepath.Join(state.workdir, dstAbs)
	}
	dstPath := filepath.Join(state.rootfsPath, strings.TrimPrefix(filepath.Clean(dstAbs), "/"))
	if strings.HasSuffix(dst, "/") {
		if err := os.MkdirAll(dstPath, 0o755); err != nil {
			return err
		}
	}

	info, err := os.Lstat(srcPath)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return copyDir(srcPath, dstPath)
	}
	if dstInfo, err := os.Lstat(dstPath); err == nil && dstInfo.IsDir() {
		dstPath = filepath.Join(dstPath, filepath.Base(srcPath))
	}
	return copyFile(srcPath, dstPath, info.Mode())
}

func (s *ImageService) applyRun(state *buildState, arg string) error {
	runLine, err := runLineFromArg(arg)
	if err != nil {
		return err
	}
	if state.workdir != "" {
		workdirPath := filepath.Join(state.rootfsPath, strings.TrimPrefix(state.workdir, "/"))
		if err := os.MkdirAll(workdirPath, 0o755); err != nil {
			return err
		}
	}
	state.runScript = append(state.runScript, renderRunBlock(state.env, state.workdir, runLine)...)
	return nil
}

func (s *ImageService) applyCmd(state *buildState, arg string) error {
	cmd, err := parseShellOrExec(arg)
	if err != nil {
		return err
	}
	state.cmd = cmd
	return nil
}

func (s *ImageService) applyEntrypoint(state *buildState, arg string) error {
	entry, err := parseShellOrExec(arg)
	if err != nil {
		return err
	}
	state.entrypoint = entry
	return nil
}

func (s *ImageService) runCommandInContainer(state *buildState, bridge string, scriptLines []string) error {
	containerId := "build-" + utils.NewUlid()[:12]
	containerDir := filepath.Join(utils.ContainerRootDir, containerId)
	upperDir := filepath.Join(containerDir, "diff")
	workDir := filepath.Join(containerDir, "work")
	mergedDir := filepath.Join(containerDir, "merged")
	outputDir := containerDir

	filesystemHandler := utils.NewFilesystemExecutor()
	runtimeHandler := droplet.NewDropletHandler()
	ipamHandler := ipam.NewIpamManager(ipam.NewIpamStore(utils.IpamStorePath))
	csmHandler := csm.NewCsmManager(csm.NewCsmStore(utils.CsmStorePath))

	rollback := buildRollback{}
	defer func() {
		if rollback.releaseAddr {
			_ = ipamHandler.Release(containerId)
		}
		if rollback.cgroup {
			_ = filesystemHandler.RemoveAll(filepath.Join(utils.CgroupRuntimeDir, containerId))
		}
		if rollback.containerDir {
			_ = filesystemHandler.RemoveAll(containerDir)
		}
		if rollback.csmEntry {
			_ = csmHandler.RemoveContainer(containerId)
		}
	}()

	containerGateway, containerAddr, err := allocateBuildAddress(ipamHandler, containerId, bridge)
	if err != nil {
		return err
	}
	rollback.releaseAddr = true

	if err := setupBuildContainerDirectory(filesystemHandler, containerDir); err != nil {
		return err
	}
	rollback.containerDir = true

	if err := setupBuildEtcFiles(filesystemHandler, containerId, containerAddr, containerGateway); err != nil {
		return err
	}
	if err := setupBuildCgroup(filesystemHandler, containerId); err != nil {
		return err
	}
	rollback.cgroup = true

	hostInterface, err := ipamHandler.GetDefaultInterface()
	if err != nil {
		return err
	}

	// hook
	hookAddr, err := ipamHandler.GetDefaultInterfaceAddr()
	if err != nil {
		return err
	}
	hookAddr = strings.Split(hookAddr, "/")[0]
	createRuntimeHook := []string{
		strings.Join([]string{
			"/usr/local/bin/condenser-hook-agent",
			"--url", "https://localhost:7757/v1/pki/sign",
			"--event", "requestCert",
			"--ca", utils.PublicCertPath,
			"--cert", utils.HookClientCertPath,
			"--key", utils.HookClientKeyPath,
		}, ","),
		strings.Join([]string{
			"/usr/local/bin/condenser-hook-agent",
			"--url", "https://" + hookAddr + ":7756/v1/hooks/droplet",
			"--event", "createRuntime",
			"--ca", utils.PublicCertPath,
			"--cert", filepath.Join(utils.ContainerRootDir, containerId, "/cert/client.crt"),
			"--key", filepath.Join(utils.ContainerRootDir, containerId, "/cert/client.key"),
		}, ","),
	}
	createRuntimeHookEnv := []string{
		"RAIND-HOOK-SETTER=CONDENSER",
		"RAIND-HOOK-SETTER=CONDENSER",
	}
	createContainerHook := []string{
		strings.Join([]string{
			"/usr/local/bin/condenser-hook-agent",
			"--url", "https://" + hookAddr + ":7756/v1/hooks/droplet",
			"--event", "createContainer",
			"--ca", utils.PublicCertPath,
			"--cert", filepath.Join(utils.ContainerRootDir, containerId, "/cert/client.crt"),
			"--key", filepath.Join(utils.ContainerRootDir, containerId, "/cert/client.key"),
		}, ","),
	}
	createContainerHookEnv := []string{
		"RAIND-HOOK-SETTER=CONDENSER",
	}
	poststartHook := []string{
		strings.Join([]string{
			"/usr/local/bin/condenser-hook-agent",
			"--url", "https://" + hookAddr + ":7756/v1/hooks/droplet",
			"--event", "poststart",
			"--ca", utils.PublicCertPath,
			"--cert", filepath.Join(utils.ContainerRootDir, containerId, "/cert/client.crt"),
			"--key", filepath.Join(utils.ContainerRootDir, containerId, "/cert/client.key"),
		}, ","),
	}
	poststartHookEnv := []string{
		"RAIND-HOOK-SETTER=CONDENSER",
	}
	stopContainerHook := []string{
		strings.Join([]string{
			"/usr/local/bin/condenser-hook-agent",
			"--url", "https://" + hookAddr + ":7756/v1/hooks/droplet",
			"--event", "stopContainer",
			"--ca", utils.PublicCertPath,
			"--cert", filepath.Join(utils.ContainerRootDir, containerId, "/cert/client.crt"),
			"--key", filepath.Join(utils.ContainerRootDir, containerId, "/cert/client.key"),
		}, ","),
	}
	stopContainerHookEnv := []string{
		"RAIND-HOOK-SETTER=CONDENSER",
	}

	runLine := strings.Join(scriptLines, "\n")
	if strings.TrimSpace(runLine) == "" {
		return errors.New("run command is empty")
	}
	runScriptPath, err := writeBuildRunScript(upperDir, runLine)
	if err != nil {
		return err
	}

	if err := csmHandler.StoreContainer(
		containerId,
		"creating",
		0,
		true,
		"build",
		"build",
		[]string{"/bin/sh", "-e", runScriptPath},
		containerId,
		"",
		filepath.Join(containerDir, "logs", "console.log"),
		"",
	); err != nil {
		return err
	}
	rollback.csmEntry = true

	spec := runtime.SpecModel{
		Rootfs:    mergedDir,
		Cwd:       state.workdir,
		Command:   buildCommand([]string{"/bin/sh", "-e"}, []string{runScriptPath}),
		Namespace: []string{"mount", "network", "uts", "pid", "ipc", "user", "cgroup"},
		Hostname:  containerId,
		Env:       cloneSlice(state.env),
		Mount:     []string{},

		HostInterface:          hostInterface,
		BridgeInterface:        bridge,
		ContainerInterface:     buildVethName(containerId),
		ContainerInterfaceAddr: containerAddr,
		ContainerGateway:       containerGateway,
		ContainerDns:           []string{"8.8.8.8"},

		ImageLayer: []string{state.rootfsPath},
		UpperDir:   upperDir,
		WorkDir:    workDir,

		CreateRuntimeHook:      createRuntimeHook,
		CreateRuntimeHookEnv:   createRuntimeHookEnv,
		CreateContainerHook:    createContainerHook,
		CreateContainerHookEnv: createContainerHookEnv,
		PoststartHook:          poststartHook,
		PoststartHookEnv:       poststartHookEnv,
		StopContainerHook:      stopContainerHook,
		StopContainerHookEnv:   stopContainerHookEnv,

		Output: outputDir,
	}

	if err := runtimeHandler.Spec(spec); err != nil {
		return err
	}
	if err := copyBuildConfig(containerDir, containerId); err != nil {
		return err
	}
	if err := runtimeHandler.Create(runtime.CreateModel{ContainerId: containerId, Tty: true}, 0); err != nil {
		return err
	}
	if err := runtimeHandler.Start(runtime.StartModel{ContainerId: containerId, Tty: true}); err != nil {
		return err
	}
	// RUN is executed as the container's init process.
	info, err := waitBuildContainerStopped(csmHandler, containerId, 10*time.Minute)
	if err != nil {
		return err
	}
	if info.ExitCode != 0 && info.Message != "process down detected." {
		return buildRunFailedError(info, containerDir)
	}
	_ = removeBuildRunScript(upperDir)
	if err := runtimeHandler.Delete(runtime.DeleteModel{ContainerId: containerId}); err != nil {
		return err
	}

	if err := applyOverlayUpper(upperDir, state.rootfsPath); err != nil {
		return err
	}

	return nil
}

type buildRollback struct {
	releaseAddr  bool
	containerDir bool
	cgroup       bool
	csmEntry     bool
}

func allocateBuildAddress(ipamHandler ipam.IpamHandler, containerId string, bridge string) (string, string, error) {
	containerInterfaceAddr, err := ipamHandler.Allocate(containerId, bridge)
	if err != nil {
		return "", "", err
	}
	containerInterfaceAddr = containerInterfaceAddr + "/24"
	containerGateway, err := ipamHandler.GetBridgeAddr(bridge)
	if err != nil {
		return "", "", err
	}
	containerGateway = strings.Split(containerGateway, "/")[0]
	return containerGateway, containerInterfaceAddr, nil
}

func setupBuildContainerDirectory(filesystemHandler utils.FilesystemHandler, containerDir string) error {
	dirs := []string{
		containerDir,
		filepath.Join(containerDir, "diff"),
		filepath.Join(containerDir, "work"),
		filepath.Join(containerDir, "merged"),
		filepath.Join(containerDir, "etc"),
		filepath.Join(containerDir, "logs"),
		filepath.Join(containerDir, "cert"),
	}
	for _, dir := range dirs {
		if err := filesystemHandler.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func setupBuildEtcFiles(filesystemHandler utils.FilesystemHandler, containerId string, containerAddr string, containerGateway string) error {
	etcDir := filepath.Join(utils.ContainerRootDir, containerId, "etc")

	hostsPath := filepath.Join(etcDir, "hosts")
	hostsData := fmt.Sprintf("127.0.0.1 localhost\n%s %s\n", strings.SplitN(containerAddr, "/", 2)[0], containerId)
	if err := filesystemHandler.WriteFile(hostsPath, []byte(hostsData), 0o644); err != nil {
		return err
	}

	hostnamePath := filepath.Join(etcDir, "hostname")
	hostnameData := fmt.Sprintf("%s\n", containerId)
	if err := filesystemHandler.WriteFile(hostnamePath, []byte(hostnameData), 0o644); err != nil {
		return err
	}

	resolvPath := filepath.Join(etcDir, "resolv.conf")
	resolvData := "nameserver " + containerGateway + "\n"
	if err := filesystemHandler.WriteFile(resolvPath, []byte(resolvData), 0o644); err != nil {
		return err
	}

	return nil
}

func setupBuildCgroup(filesystemHandler utils.FilesystemHandler, containerId string) error {
	cgroupPath := filepath.Join(utils.CgroupRuntimeDir, containerId)
	return filesystemHandler.MkdirAll(cgroupPath, 0o755)
}

func parseDripfile(path string) ([]buildInstruction, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	var joined []string
	var buf strings.Builder
	for _, line := range lines {
		l := strings.TrimSpace(line)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		if strings.HasSuffix(l, "\\") {
			buf.WriteString(strings.TrimSuffix(l, "\\"))
			buf.WriteString(" ")
			continue
		}
		if buf.Len() > 0 {
			buf.WriteString(l)
			joined = append(joined, buf.String())
			buf.Reset()
			continue
		}
		joined = append(joined, l)
	}
	if buf.Len() > 0 {
		joined = append(joined, buf.String())
	}

	var instructions []buildInstruction
	for _, line := range joined {
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
		op := strings.ToUpper(parts[0])
		args := strings.TrimSpace(line[len(parts[0]):])
		instructions = append(instructions, buildInstruction{op: op, args: args})
	}
	return instructions, nil
}

func parseShellOrExec(arg string) ([]string, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return nil, errors.New("command is empty")
	}
	if strings.HasPrefix(arg, "[") {
		var arr []string
		if err := json.Unmarshal([]byte(arg), &arr); err != nil {
			return nil, fmt.Errorf("invalid exec form: %w", err)
		}
		return arr, nil
	}
	return []string{"/bin/sh", "-c", arg}, nil
}

func buildCommand(entrypoint, cmd []string) string {
	var all []string
	all = append(all, entrypoint...)
	all = append(all, cmd...)

	var quoted []string
	for _, a := range all {
		quoted = append(quoted, shellescape.Quote(a))
	}
	return strings.Join(quoted, " ")
}

func cloneSlice(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func setEnvVar(env []string, key string, value string) []string {
	key = strings.TrimSpace(key)
	if key == "" {
		return env
	}
	prefix := key + "="
	for i, e := range env {
		if strings.HasPrefix(e, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

func buildVethName(containerId string) string {
	token := containerId
	if parts := strings.SplitN(containerId, "-", 2); len(parts) == 2 && parts[1] != "" {
		token = parts[1]
	}
	token = strings.ReplaceAll(token, "-", "")
	name := "rd_" + token
	if len(name) > 15 {
		name = name[:15]
	}
	if name == "rd_" {
		name = "rd_" + containerId
		if len(name) > 15 {
			name = name[:15]
		}
	}
	return name
}

func runLineFromArg(arg string) (string, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return "", errors.New("RUN requires command")
	}
	if strings.HasPrefix(arg, "[") {
		var arr []string
		if err := json.Unmarshal([]byte(arg), &arr); err != nil {
			return "", fmt.Errorf("invalid exec form: %w", err)
		}
		var quoted []string
		for _, a := range arr {
			quoted = append(quoted, shellescape.Quote(a))
		}
		return strings.Join(quoted, " "), nil
	}
	return arg, nil
}

func renderRunBlock(env []string, workdir string, runLine string) []string {
	var lines []string
	for _, kv := range env {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			continue
		}
		lines = append(lines, "export "+parts[0]+"="+shellescape.Quote(parts[1]))
	}
	if workdir != "" {
		lines = append(lines, "mkdir -p "+shellescape.Quote(workdir))
		lines = append(lines, "cd "+shellescape.Quote(workdir))
	}
	lines = append(lines, runLine)
	return lines
}

func writeBuildRunScript(upperDir string, runLine string) (string, error) {
	scriptPath := "/raind-build/run.sh"
	hostPath := filepath.Join(upperDir, "raind-build", "run.sh")
	if err := os.MkdirAll(filepath.Dir(hostPath), 0o755); err != nil {
		return "", err
	}
	content := "#!/bin/sh\nset -ex\n" + runLine + "\n"
	if err := os.WriteFile(hostPath, []byte(content), 0o755); err != nil {
		return "", err
	}
	return scriptPath, nil
}

func removeBuildRunScript(mergedDir string) error {
	runDir := filepath.Join(mergedDir, "raind-build")
	hostPath := filepath.Join(runDir, "run.sh")
	if err := os.Remove(hostPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	entries, err := os.ReadDir(runDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(entries) == 0 {
		if err := os.Remove(runDir); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func waitBuildContainerStopped(csmHandler csm.CsmHandler, containerId string, timeout time.Duration) (csm.ContainerInfo, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		info, err := csmHandler.GetContainerById(containerId)
		if err == nil {
			switch info.State {
			case "stopped":
				return info, nil
			case "running", "created", "creating":
				time.Sleep(500 * time.Millisecond)
				continue
			default:
				return csm.ContainerInfo{}, fmt.Errorf("unexpected container state: %s", info.State)
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return csm.ContainerInfo{}, fmt.Errorf("timeout waiting for build container to stop: %s", containerId)
}

func buildRunFailedError(info csm.ContainerInfo, containerDir string) error {
	msg := fmt.Sprintf("RUN command failed: exit_code=%d", info.ExitCode)
	if strings.TrimSpace(info.Reason) != "" {
		msg += ", reason=" + info.Reason
	}
	if strings.TrimSpace(info.Message) != "" {
		msg += ", message=" + info.Message
	}
	if tail := readBuildLogTail(info.LogPath, containerDir); tail != "" {
		msg += "\n--- build log tail ---\n" + tail
	}
	return errors.New(msg)
}

func readBuildLogTail(logPath string, containerDir string) string {
	candidates := []string{}
	if strings.TrimSpace(logPath) != "" {
		candidates = append(candidates, logPath)
	}
	candidates = append(
		candidates,
		filepath.Join(containerDir, "logs", "console.log"),
		filepath.Join(containerDir, "logs", "init.log"),
	)
	seen := map[string]struct{}{}
	for _, p := range candidates {
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		b, err := os.ReadFile(p)
		if err != nil || len(b) == 0 {
			continue
		}
		if len(b) > 4096 {
			b = b[len(b)-4096:]
		}
		out := strings.TrimSpace(string(b))
		if out != "" {
			return out
		}
	}
	return ""
}

func safeJoin(baseDir string, rel string) (string, error) {
	clean := filepath.Clean(rel)
	if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		return "", fmt.Errorf("invalid path: %s", rel)
	}
	full := filepath.Join(baseDir, clean)
	if !strings.HasPrefix(full, filepath.Clean(baseDir)+string(os.PathSeparator)) && filepath.Clean(baseDir) != full {
		return "", fmt.Errorf("invalid path: %s", rel)
	}
	return full, nil
}

func copyDir(src string, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", src)
	}
	if err := os.MkdirAll(dst, info.Mode()); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == src {
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		mode := info.Mode()
		if mode.IsDir() {
			return os.MkdirAll(target, mode.Perm())
		}
		if mode&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		}
		if mode.IsRegular() {
			return copyFile(path, target, mode)
		}
		return nil
	})
}

func copyFile(src string, dst string, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func applyOverlayUpper(upperDir string, rootfs string) error {
	return filepath.WalkDir(upperDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == upperDir {
			return nil
		}
		rel, err := filepath.Rel(upperDir, path)
		if err != nil {
			return err
		}
		base := filepath.Base(rel)
		if strings.HasPrefix(base, ".wh.") {
			dir := filepath.Dir(rel)
			if base == ".wh..wh..opq" {
				targetDir := filepath.Join(rootfs, dir)
				entries, err := os.ReadDir(targetDir)
				if err != nil {
					if os.IsNotExist(err) {
						return nil
					}
					return err
				}
				for _, e := range entries {
					if err := os.RemoveAll(filepath.Join(targetDir, e.Name())); err != nil {
						return err
					}
				}
				return nil
			}
			target := filepath.Join(rootfs, dir, strings.TrimPrefix(base, ".wh."))
			return os.RemoveAll(target)
		}

		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		target := filepath.Join(rootfs, rel)
		mode := info.Mode()
		if mode.IsDir() {
			return os.MkdirAll(target, mode.Perm())
		}
		if mode&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			_ = os.RemoveAll(target)
			return os.Symlink(link, target)
		}
		if mode.IsRegular() {
			return copyFile(path, target, mode)
		}
		return nil
	})
}

func copyBuildConfig(containerDir, containerId string) error {
	src := filepath.Join(containerDir, "config.json")
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	debugDst := filepath.Join("/tmp", "raind-build-debug-config.json")
	return os.WriteFile(debugDst, b, 0o644)
}

func (s *ImageService) storeBuiltImage(imageRepo, imageRef string, state buildState) error {
	repoName := imageRepo
	if strings.Contains(imageRepo, "/") {
		parts := strings.Split(imageRepo, "/")
		repoName = parts[len(parts)-1]
	}
	repoOut := filepath.Join(utils.LayerRootDir, repoName, imageRef)

	if s.ilmHandler.IsImageExist(imageRepo, imageRef) {
		if err := s.ilmHandler.RemoveImage(imageRepo, imageRef); err != nil {
			return err
		}
		_ = os.RemoveAll(repoOut)
	}

	if err := os.MkdirAll(repoOut, 0o755); err != nil {
		return err
	}
	rootfsPath := filepath.Join(repoOut, "rootfs")
	if err := copyDir(state.rootfsPath, rootfsPath); err != nil {
		return err
	}

	config := ImageConfigFile{
		Config: ImageConfigObject{
			Env:        cloneSlice(state.env),
			Cmd:        cloneSlice(state.cmd),
			Entrypoint: cloneSlice(state.entrypoint),
			WorkingDir: state.workdir,
		},
	}
	configPath := filepath.Join(repoOut, "config.json")
	b, err := json.Marshal(config)
	if err != nil {
		return err
	}
	if err := os.WriteFile(configPath, b, 0o644); err != nil {
		return err
	}

	if err := s.ilmHandler.StoreImage(imageRepo, imageRef, repoOut, configPath, rootfsPath); err != nil {
		return err
	}
	return nil
}

// extractTarToDir extracts a tar stream into a directory and prevents path traversal.
func ExtractTarToDir(r io.Reader, dst string) error {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if hdr.Name == "" {
			continue
		}
		target, err := safeJoin(dst, hdr.Name)
		if err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				_ = f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return err
			}
		default:
			// ignore other types
		}
	}
}
