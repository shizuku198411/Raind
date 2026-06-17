package image

import (
	"archive/tar"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"raind/internal/condenser/runtime"
	"raind/internal/condenser/runtime/droplet"
	"raind/internal/condenser/store/csm"
	"raind/internal/condenser/store/ipam"
	"raind/internal/condenser/utils"
	"strconv"
	"strings"
	"time"

	"al.essio.dev/pkg/shellescape"
	"golang.org/x/sys/unix"
)

type buildState struct {
	imageRepo  string
	imageRef   string
	rootfsPath string
	alias      string

	env        []string
	workdir    string
	cmd        []string
	entrypoint []string
	runScript  []string
	user       string
	shell      []string
}

type buildInstruction struct {
	op   string
	args string
}

type buildStage struct {
	name  string
	index int
	state buildState
}

type fromSpec struct {
	image string
	alias string
}

type copySpec struct {
	from    string
	chmod   string
	sources []string
	dest    string
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
	reportBuildProgress(buildParameter.Progress, "parsing", "buildfile", "parsing build file")
	instructions, err := parseDripfile(buildParameter.DripfilePath)
	if err != nil {
		return "", err
	}
	reportBuildProgress(buildParameter.Progress, "parsed", "buildfile", fmt.Sprintf("parsed %d instructions", len(instructions)))

	var stages []buildStage
	stageByName := map[string]buildStage{}
	var cleanupRootfs []string
	state := newBuildState()
	defer func() {
		for _, rootfs := range cleanupRootfs {
			if rootfs != "" {
				_ = os.RemoveAll(rootfs)
			}
		}
	}()

	for _, ins := range instructions {
		switch ins.op {
		case "FROM":
			if state.rootfsPath != "" {
				if err := s.flushRunScript(&state, bridge); err != nil {
					return "", err
				}
				stage := buildStage{name: state.alias, index: len(stages), state: state}
				stages = append(stages, stage)
				if stage.name != "" {
					stageByName[strings.ToLower(stage.name)] = stage
				}
			}
			from, err := parseFromSpec(ins.args)
			if err != nil {
				return "", err
			}
			state = newBuildState()
			state.alias = from.alias
			reportBuildProgress(buildParameter.Progress, "loading", from.image, "loading base image")
			if err := s.applyFrom(&state, from.image, buildParameter.Progress); err != nil {
				return "", err
			}
			reportBuildProgress(buildParameter.Progress, "loaded", from.image, "base image ready")
			cleanupRootfs = append(cleanupRootfs, state.rootfsPath)
		case "WORKDIR":
			if err := s.applyWorkdir(&state, ins.args); err != nil {
				return "", err
			}
			reportBuildProgress(buildParameter.Progress, "workdir", state.workdir, "set workdir")
		case "ENV":
			if err := s.applyEnv(&state, ins.args); err != nil {
				return "", err
			}
			reportBuildProgress(buildParameter.Progress, "env", "ENV", "set environment")
		case "COPY", "ADD":
			if err := s.flushRunScript(&state, bridge); err != nil {
				return "", err
			}
			reportBuildProgress(buildParameter.Progress, "copying", ins.op, truncateBuildDetail(ins.args))
			if err := s.applyCopy(&state, buildParameter.ContextDir, stages, stageByName, ins.args); err != nil {
				return "", err
			}
			reportBuildProgress(buildParameter.Progress, "copied", ins.op, truncateBuildDetail(ins.args))
		case "RUN":
			if err := s.applyRun(&state, ins.args); err != nil {
				return "", err
			}
			reportBuildProgress(buildParameter.Progress, "running", "RUN", truncateBuildDetail(ins.args))
			if err := s.flushRunScript(&state, bridge); err != nil {
				return "", err
			}
			reportBuildProgress(buildParameter.Progress, "complete", "RUN", "run completed")
		case "CMD":
			if err := s.applyCmd(&state, ins.args); err != nil {
				return "", err
			}
			reportBuildProgress(buildParameter.Progress, "cmd", "CMD", "set default command")
		case "ENTRYPOINT":
			if err := s.applyEntrypoint(&state, ins.args); err != nil {
				return "", err
			}
			reportBuildProgress(buildParameter.Progress, "entrypoint", "ENTRYPOINT", "set entrypoint")
		case "USER":
			if err := s.applyUser(&state, ins.args); err != nil {
				return "", err
			}
			reportBuildProgress(buildParameter.Progress, "user", state.user, "set user")
		case "SHELL":
			if err := s.applyShell(&state, ins.args); err != nil {
				return "", err
			}
			reportBuildProgress(buildParameter.Progress, "shell", "SHELL", "set shell")
		case "ARG", "LABEL", "EXPOSE", "VOLUME", "STOPSIGNAL", "HEALTHCHECK", "ONBUILD", "MAINTAINER":
			// Parsed for Dockerfile compatibility. Raind does not persist these metadata fields yet.
			reportBuildProgress(buildParameter.Progress, "metadata", ins.op, truncateBuildDetail(ins.args))
		default:
			return "", fmt.Errorf("unsupported instruction: %s", ins.op)
		}
	}

	if state.rootfsPath == "" {
		return "", errors.New("missing FROM instruction")
	}

	if err := s.flushRunScript(&state, bridge); err != nil {
		return "", err
	}

	imageRepo, imageRef, err := s.parseImageRef(buildParameter.Image)
	if err != nil {
		return "", err
	}

	reportBuildProgress(buildParameter.Progress, "storing", buildParameter.Image, "storing image")
	if err := s.storeBuiltImage(imageRepo, imageRef, state); err != nil {
		return "", err
	}
	reportBuildProgress(buildParameter.Progress, "done", imageRepo+":"+imageRef, "build complete")
	return imageRepo + ":" + imageRef, nil
}

func newBuildState() buildState {
	return buildState{
		workdir: "/",
		shell:   []string{"/bin/sh", "-c"},
	}
}

func reportBuildProgress(progress ProgressFunc, status string, id string, detail string) {
	if progress == nil {
		return
	}
	progress(PullProgressEvent{
		Status: status,
		ID:     id,
		Detail: detail,
	})
}

func truncateBuildDetail(detail string) string {
	const max = 120
	detail = strings.Join(strings.Fields(detail), " ")
	if len(detail) <= max {
		return detail
	}
	return detail[:max-3] + "..."
}

func (s *ImageService) applyFrom(state *buildState, image string, progress ProgressFunc) error {
	image = strings.TrimSpace(strings.Fields(image)[0])
	if image == "" {
		return errors.New("FROM requires image")
	}
	if image == "scratch" {
		tmpRootfs, err := os.MkdirTemp("", "raind-build-rootfs-")
		if err != nil {
			return err
		}
		state.imageRepo = "scratch"
		state.imageRef = "latest"
		state.rootfsPath = tmpRootfs
		state.env = nil
		state.workdir = "/"
		state.cmd = nil
		state.entrypoint = nil
		state.user = ""
		return nil
	}

	imageRepo, imageRef, err := s.parseImageRef(image)
	if err != nil {
		return err
	}

	if !s.ilmHandler.IsImageExist(imageRepo, imageRef) {
		if err := s.Pull(ServicePullModel{Image: image, Progress: progress}); err != nil {
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
	state.user = imageConfig.Config.User
	if len(state.shell) == 0 {
		state.shell = []string{"/bin/sh", "-c"}
	}
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
	parts, err := splitDockerWords(arg)
	if err != nil {
		return err
	}
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

func (s *ImageService) applyCopy(state *buildState, contextDir string, stages []buildStage, stageByName map[string]buildStage, arg string) error {
	spec, err := parseCopySpec(arg)
	if err != nil {
		return err
	}
	if len(spec.sources) < 1 {
		return errors.New("COPY/ADD requires src and dest")
	}

	sourceRoot := contextDir
	stageSource := false
	if spec.from != "" {
		stage, err := resolveCopyStage(spec.from, stages, stageByName)
		if err != nil {
			if _, atoiErr := strconv.Atoi(spec.from); atoiErr == nil {
				return err
			}
			external := newBuildState()
			if fromErr := s.applyFrom(&external, spec.from, nil); fromErr != nil {
				return fmt.Errorf("%w; also failed to use %q as external image: %v", err, spec.from, fromErr)
			}
			defer os.RemoveAll(external.rootfsPath)
			sourceRoot = external.rootfsPath
			stageSource = true
		} else {
			sourceRoot = stage.state.rootfsPath
			stageSource = true
		}
	}

	dst := spec.dest
	dstAbs := dst
	if !filepath.IsAbs(dstAbs) {
		dstAbs = filepath.Join(state.workdir, dstAbs)
	}
	dstPath := filepath.Join(state.rootfsPath, strings.TrimPrefix(filepath.Clean(dstAbs), "/"))
	if strings.HasSuffix(dst, "/") || len(spec.sources) > 1 {
		if err := os.MkdirAll(dstPath, 0o755); err != nil {
			return err
		}
	}
	if len(spec.sources) > 1 {
		if dstInfo, err := os.Lstat(dstPath); err != nil || !dstInfo.IsDir() {
			return errors.New("COPY/ADD with multiple sources requires destination directory")
		}
	}

	for _, src := range spec.sources {
		if src == "" {
			return errors.New("COPY/ADD requires src and dest")
		}
		srcPath, err := safeBuildSourceJoin(sourceRoot, src, stageSource)
		if err != nil {
			return err
		}
		resolvedSrcPath, info, err := resolveCopySourceWithinRoot(sourceRoot, srcPath, src)
		if err != nil {
			return err
		}
		targetPath := dstPath
		sourceBase := filepath.Base(filepath.Clean(src))
		if len(spec.sources) > 1 {
			targetPath = filepath.Join(dstPath, sourceBase)
		}
		if info.IsDir() {
			if err := copyDir(resolvedSrcPath, targetPath); err != nil {
				return err
			}
			continue
		}
		if dstInfo, err := os.Lstat(targetPath); err == nil && dstInfo.IsDir() {
			targetPath = filepath.Join(targetPath, sourceBase)
		}
		if err := copyFile(resolvedSrcPath, targetPath, info.Mode()); err != nil {
			return err
		}
		if spec.chmod != "" {
			mode, err := parseChmod(spec.chmod)
			if err != nil {
				return err
			}
			if err := os.Chmod(targetPath, mode); err != nil {
				return err
			}
		}
	}
	return nil
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

func (s *ImageService) applyUser(state *buildState, arg string) error {
	user := strings.TrimSpace(arg)
	if user == "" {
		return errors.New("USER requires user")
	}
	state.user = user
	return nil
}

func (s *ImageService) applyShell(state *buildState, arg string) error {
	shell, err := parseExecArray(arg)
	if err != nil {
		return err
	}
	if len(shell) == 0 {
		return errors.New("SHELL requires command")
	}
	state.shell = shell
	return nil
}

func (s *ImageService) flushRunScript(state *buildState, bridge string) error {
	if len(state.runScript) == 0 {
		return nil
	}
	script := cloneSlice(state.runScript)
	state.runScript = nil
	return s.runCommandInContainer(state, bridge, script)
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
	hookAgentPath := utils.HookAgentBinPath()
	createRuntimeHook := []string{
		strings.Join([]string{
			hookAgentPath,
			"--url", "https://localhost:7757/v1/pki/sign",
			"--event", "requestCert",
			"--ca", utils.PublicCertPath,
			"--cert", utils.HookClientCertPath,
			"--key", utils.HookClientKeyPath,
		}, ","),
		strings.Join([]string{
			hookAgentPath,
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
			hookAgentPath,
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
			hookAgentPath,
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
			hookAgentPath,
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
		Namespace: []string{"mount", "network", "uts", "pid", "ipc", "cgroup"},
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
	// RUN is executed as the container's init process. Judge it only by a
	// finalized exit status. A process-down monitor fallback is not a success.
	info, err := waitBuildContainerStopped(csmHandler, containerId, 10*time.Minute)
	if err != nil {
		return err
	}
	if info.ExitCode != 0 {
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

func parseFromSpec(arg string) (fromSpec, error) {
	parts, err := splitDockerWords(arg)
	if err != nil {
		return fromSpec{}, err
	}
	var image string
	var alias string
	for i := 0; i < len(parts); i++ {
		part := parts[i]
		if strings.HasPrefix(part, "--platform=") {
			continue
		}
		if part == "--platform" {
			i++
			continue
		}
		if image == "" {
			image = part
			continue
		}
		if strings.EqualFold(part, "AS") {
			if i+1 >= len(parts) || parts[i+1] == "" {
				return fromSpec{}, errors.New("FROM AS requires stage name")
			}
			alias = parts[i+1]
			i++
			continue
		}
		return fromSpec{}, fmt.Errorf("invalid FROM argument: %s", part)
	}
	if image == "" {
		return fromSpec{}, errors.New("FROM requires image")
	}
	return fromSpec{image: image, alias: alias}, nil
}

func parseCopySpec(arg string) (copySpec, error) {
	parts, err := splitDockerInstructionArgs(arg)
	if err != nil {
		return copySpec{}, err
	}
	var out copySpec
	var operands []string
	for i := 0; i < len(parts); i++ {
		part := parts[i]
		if !strings.HasPrefix(part, "--") {
			operands = append(operands, part)
			continue
		}
		key, value, hasValue := strings.Cut(strings.TrimPrefix(part, "--"), "=")
		switch key {
		case "from":
			if !hasValue {
				i++
				if i >= len(parts) {
					return copySpec{}, errors.New("COPY --from requires value")
				}
				value = parts[i]
			}
			out.from = value
		case "chmod":
			if !hasValue {
				i++
				if i >= len(parts) {
					return copySpec{}, errors.New("COPY --chmod requires value")
				}
				value = parts[i]
			}
			out.chmod = value
		case "chown", "link":
			if !hasValue && key == "chown" {
				i++
				if i >= len(parts) {
					return copySpec{}, errors.New("COPY --chown requires value")
				}
			}
			// Accepted for Dockerfile compatibility. Ownership is not changed by Raind yet.
		default:
			return copySpec{}, fmt.Errorf("unsupported COPY/ADD option: --%s", key)
		}
	}
	if len(operands) == 1 && strings.HasPrefix(strings.TrimSpace(operands[0]), "[") {
		var arr []string
		if err := json.Unmarshal([]byte(operands[0]), &arr); err != nil {
			return copySpec{}, fmt.Errorf("invalid COPY/ADD JSON form: %w", err)
		}
		operands = arr
	}
	if len(operands) < 2 {
		return copySpec{}, errors.New("COPY/ADD requires src and dest")
	}
	out.sources = operands[:len(operands)-1]
	out.dest = operands[len(operands)-1]
	return out, nil
}

func splitDockerInstructionArgs(arg string) ([]string, error) {
	arg = strings.TrimSpace(arg)
	if idx := strings.Index(arg, "["); idx > 0 {
		prefix := strings.TrimSpace(arg[:idx])
		array := strings.TrimSpace(arg[idx:])
		parts, err := splitDockerWords(prefix)
		if err != nil {
			return nil, err
		}
		var probe []string
		if err := json.Unmarshal([]byte(array), &probe); err != nil {
			return nil, fmt.Errorf("invalid JSON form: %w", err)
		}
		return append(parts, array), nil
	}
	return splitDockerWords(arg)
}

func splitDockerWords(s string) ([]string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	if strings.HasPrefix(s, "[") {
		return []string{s}, nil
	}

	var out []string
	var b strings.Builder
	var quote rune
	escaped := false
	for _, r := range s {
		if escaped {
			b.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
				continue
			}
			b.WriteRune(r)
			continue
		}
		switch r {
		case '\'', '"':
			quote = r
		case ' ', '\t', '\r', '\n':
			if b.Len() > 0 {
				out = append(out, b.String())
				b.Reset()
			}
		default:
			b.WriteRune(r)
		}
	}
	if escaped {
		b.WriteRune('\\')
	}
	if quote != 0 {
		return nil, errors.New("unterminated quote")
	}
	if b.Len() > 0 {
		out = append(out, b.String())
	}
	return out, nil
}

func resolveCopyStage(name string, stages []buildStage, stageByName map[string]buildStage) (buildStage, error) {
	if name == "" {
		return buildStage{}, errors.New("empty stage name")
	}
	if idx, err := strconv.Atoi(name); err == nil {
		if idx < 0 || idx >= len(stages) {
			return buildStage{}, fmt.Errorf("unknown build stage: %s", name)
		}
		return stages[idx], nil
	}
	if stage, ok := stageByName[strings.ToLower(name)]; ok {
		return stage, nil
	}
	return buildStage{}, fmt.Errorf("unknown build stage: %s", name)
}

func safeBuildSourceJoin(root string, src string, allowAbs bool) (string, error) {
	clean := filepath.Clean(src)
	if filepath.IsAbs(clean) {
		clean = strings.TrimPrefix(clean, string(os.PathSeparator))
	}
	return safeJoin(root, clean)
}

func resolveCopySourceWithinRoot(root string, srcPath string, originalSource string) (string, os.FileInfo, error) {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", nil, err
	}
	resolvedSrc, err := filepath.EvalSymlinks(srcPath)
	if err != nil {
		return "", nil, err
	}
	if !pathWithinRoot(resolvedRoot, resolvedSrc) {
		return "", nil, fmt.Errorf("COPY/ADD source escapes build context: %s", originalSource)
	}
	info, err := os.Stat(resolvedSrc)
	if err != nil {
		return "", nil, err
	}
	return resolvedSrc, info, nil
}

func pathWithinRoot(root string, target string) bool {
	root = filepath.Clean(root)
	target = filepath.Clean(target)
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && !filepath.IsAbs(rel)
}

func parseChmod(value string) (fs.FileMode, error) {
	mode, err := strconv.ParseUint(value, 8, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid chmod value: %s", value)
	}
	return fs.FileMode(mode), nil
}

func parseShellOrExec(arg string) ([]string, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return nil, errors.New("command is empty")
	}
	if strings.HasPrefix(arg, "[") {
		return parseExecArray(arg)
	}
	return []string{"/bin/sh", "-c", arg}, nil
}

func parseExecArray(arg string) ([]string, error) {
	var arr []string
	if err := json.Unmarshal([]byte(strings.TrimSpace(arg)), &arr); err != nil {
		return nil, fmt.Errorf("invalid exec form: %w", err)
	}
	return arr, nil
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

var buildStatusPollInterval = 500 * time.Millisecond
var buildExitStatusFinalizeTimeout = 5 * time.Second

func waitBuildContainerStopped(csmHandler csm.CsmHandler, containerId string, timeout time.Duration) (csm.ContainerInfo, error) {
	stopDeadline := time.Now().Add(timeout)
	var finalDeadline time.Time
	var lastInfo csm.ContainerInfo

	for {
		now := time.Now()
		if finalDeadline.IsZero() && now.After(stopDeadline) {
			return csm.ContainerInfo{}, fmt.Errorf("timeout waiting for build container to stop: %s", containerId)
		}
		if !finalDeadline.IsZero() && now.After(finalDeadline) {
			return csm.ContainerInfo{}, buildRunExitStatusNotFinalError(lastInfo)
		}

		info, err := csmHandler.GetContainerById(containerId)
		if err == nil {
			lastInfo = info
			switch info.State {
			case "stopped":
				if isBuildExitStatusFinal(info) {
					return info, nil
				}
				if finalDeadline.IsZero() {
					finalDeadline = time.Now().Add(buildExitStatusFinalizeTimeout)
				}
			case "running", "created", "creating":
				// Still waiting for the build RUN container to terminate.
			default:
				return csm.ContainerInfo{}, fmt.Errorf("unexpected container state: %s", info.State)
			}
		}
		time.Sleep(buildStatusPollInterval)
	}
}

func isBuildExitStatusFinal(info csm.ContainerInfo) bool {
	if info.State != "stopped" {
		return false
	}
	if info.ExitCode < 0 {
		return false
	}
	if info.ExitCode != 0 {
		return true
	}
	return strings.TrimSpace(info.Reason) != "" || strings.TrimSpace(info.Message) != ""
}

func buildRunExitStatusNotFinalError(info csm.ContainerInfo) error {
	msg := fmt.Sprintf("RUN command exit status was not finalized: container_id=%s", info.ContainerId)
	if strings.TrimSpace(info.State) != "" {
		msg += ", state=" + info.State
	}
	msg += fmt.Sprintf(", exit_code=%d", info.ExitCode)
	if strings.TrimSpace(info.Reason) != "" {
		msg += ", reason=" + info.Reason
	}
	if strings.TrimSpace(info.Message) != "" {
		msg += ", message=" + info.Message
	}
	return errors.New(msg)
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
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid path: %s", rel)
	}

	baseClean := filepath.Clean(baseDir)
	full := filepath.Join(baseClean, clean)
	relToBase, err := filepath.Rel(baseClean, full)
	if err != nil || relToBase == ".." || strings.HasPrefix(relToBase, ".."+string(os.PathSeparator)) || filepath.IsAbs(relToBase) {
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
			User:       state.user,
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
	return ExtractTarToDirWithOptions(r, dst, DefaultExtractTarOptions())
}

const (
	DefaultBuildContextMaxBytes = int64(1 << 30)
	DefaultBuildFileMaxBytes    = int64(512 << 20)
	DefaultBuildMaxEntries      = 100_000
)

type ExtractTarOptions struct {
	MaxBytes   int64
	MaxFile    int64
	MaxEntries int
}

func DefaultExtractTarOptions() ExtractTarOptions {
	return ExtractTarOptions{
		MaxBytes:   DefaultBuildContextMaxBytes,
		MaxFile:    DefaultBuildFileMaxBytes,
		MaxEntries: DefaultBuildMaxEntries,
	}
}

// ExtractTarToDirWithOptions extracts a tar stream into a directory and applies
// resource limits while preserving path traversal protections.
func ExtractTarToDirWithOptions(r io.Reader, dst string, opt ExtractTarOptions) error {
	if opt.MaxBytes <= 0 {
		opt.MaxBytes = DefaultBuildContextMaxBytes
	}
	if opt.MaxFile <= 0 {
		opt.MaxFile = DefaultBuildFileMaxBytes
	}
	if opt.MaxEntries <= 0 {
		opt.MaxEntries = DefaultBuildMaxEntries
	}

	limited := &buildContextLimitReader{r: r, max: opt.MaxBytes}
	tr := tar.NewReader(limited)
	entries := 0
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
		entries++
		if entries > opt.MaxEntries {
			return fmt.Errorf("build context has too many entries: max=%d", opt.MaxEntries)
		}
		target, err := safeJoin(dst, hdr.Name)
		if err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := ensureSafeExtractionDir(dst, target, os.FileMode(hdr.Mode).Perm()); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := ensureSafeExtractionDir(dst, filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if hdr.Size < 0 {
				return fmt.Errorf("invalid tar entry size: %s", hdr.Name)
			}
			if hdr.Size > opt.MaxFile {
				return fmt.Errorf("build context file too large: %s max=%d bytes", hdr.Name, opt.MaxFile)
			}
			f, err := openRegularFileNoFollow(target, os.FileMode(hdr.Mode).Perm())
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
			if err := ensureSafeExtractionDir(dst, filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := ensureExtractionTargetAbsentOrNotSymlink(target, hdr.Name); err != nil {
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

func ensureSafeExtractionDir(root string, dir string, perm fs.FileMode) error {
	root = filepath.Clean(root)
	dir = filepath.Clean(dir)
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return err
	}
	if rel == "." {
		return nil
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("invalid extraction directory: %s", dir)
	}

	current := root
	for _, part := range strings.Split(rel, string(os.PathSeparator)) {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			return fmt.Errorf("invalid extraction directory: %s", dir)
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("refusing to follow symlink while extracting tar: %s", current)
			}
			if !info.IsDir() {
				return fmt.Errorf("not a directory while extracting tar: %s", current)
			}
			continue
		}
		if !os.IsNotExist(err) {
			return err
		}
		if err := os.Mkdir(current, perm); err != nil {
			if !os.IsExist(err) {
				return err
			}
			info, statErr := os.Lstat(current)
			if statErr != nil {
				return statErr
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("refusing to follow symlink while extracting tar: %s", current)
			}
			if !info.IsDir() {
				return fmt.Errorf("not a directory while extracting tar: %s", current)
			}
		}
	}
	return nil
}

func ensureExtractionTargetAbsentOrNotSymlink(target string, name string) error {
	info, err := os.Lstat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to overwrite symlink while extracting tar: %s", name)
	}
	return nil
}

func openRegularFileNoFollow(target string, mode fs.FileMode) (*os.File, error) {
	fd, err := unix.Open(target, unix.O_CREAT|unix.O_WRONLY|unix.O_TRUNC|unix.O_NOFOLLOW|unix.O_CLOEXEC, uint32(mode.Perm()))
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), target), nil
}

type buildContextLimitReader struct {
	r     io.Reader
	max   int64
	total int64
}

func (r *buildContextLimitReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	remaining := r.max - r.total
	if remaining < 0 {
		return 0, fmt.Errorf("build context too large: max=%d bytes", r.max)
	}
	if int64(len(p)) > remaining+1 {
		p = p[:remaining+1]
	}
	n, err := r.r.Read(p)
	r.total += int64(n)
	if r.total > r.max {
		return n, fmt.Errorf("build context too large: max=%d bytes", r.max)
	}
	return n, err
}
