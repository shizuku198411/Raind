package command

import (
	"fmt"
	"os"
	"strings"

	"raind/internal/droplet/spec"

	"github.com/google/shlex"
	"github.com/urfave/cli/v2"
)

func commandSpec() *cli.Command {
	return &cli.Command{
		Name:  "spec",
		Usage: "create a new specification file",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "rootfs",
				Usage: "path to container root filesystem",
				Value: "rootfs",
			},
			&cli.StringSliceFlag{
				Name:  "mount",
				Usage: "mount info (source:dest:options)",
			},
			&cli.StringFlag{
				Name:  "cwd",
				Usage: "container working directory",
				Value: "/",
			},
			&cli.StringSliceFlag{
				Name:  "env",
				Usage: "environment variables (KEY=VALUE)",
			},
			&cli.StringSliceFlag{
				Name:  "cap-add",
				Usage: "add Linux capabilities (e.g. CAP_NET_ADMIN)",
			},
			&cli.StringSliceFlag{
				Name:  "cap-drop",
				Usage: "drop Linux capabilities (e.g. CAP_NET_RAW)",
			},
			&cli.StringFlag{
				Name:  "command",
				Usage: "container entrypoint",
				Value: "sh",
			},
			&cli.StringSliceFlag{
				Name:  "ns",
				Usage: "namespace target [mount|network|uts|pid|ipc|user|cgroup]",
			},
			&cli.StringSliceFlag{
				Name:  "ns-path",
				Usage: "namespace join target (format: type=path, type in [mount|network|uts|pid|ipc|user|cgroup])",
			},
			&cli.StringFlag{
				Name:  "hostname",
				Usage: "container hostname",
			},
			&cli.BoolFlag{
				Name:  "rootless",
				Usage: "mark this spec as rootless and request non-root host-ID mapping",
				Value: false,
			},
			&cli.StringFlag{
				Name:  "rootless-mode",
				Usage: "rootless ID mapping mode [shifted-root|login-root]",
				Value: spec.RootlessModeShiftedRoot,
			},
			&cli.IntFlag{
				Name:  "rootless-root-uid",
				Usage: "host UID mapped to container uid 0 in login-root mode",
				Value: 0,
			},
			&cli.IntFlag{
				Name:  "rootless-root-gid",
				Usage: "host GID mapped to container gid 0 in login-root mode",
				Value: 0,
			},

			// network
			&cli.StringFlag{
				Name:  "host_if_name",
				Usage: "host interface name",
				Value: "eth0",
			},
			&cli.StringFlag{
				Name:  "bridge_if_name",
				Usage: "bridge interface name",
				Value: "raind_br0",
			},
			&cli.StringFlag{
				Name:  "if_name",
				Usage: "container interface name",
				Value: "eth0",
			},
			&cli.StringFlag{
				Name:  "if_addr",
				Usage: "container interface address",
				Value: "172.16.0.1/24",
			},
			&cli.StringFlag{
				Name:  "if_gateway",
				Usage: "container interface gateway",
				Value: "172.16.0.254",
			},
			&cli.StringSliceFlag{
				Name:  "dns",
				Usage: "dns server",
			},

			// layer
			&cli.StringSliceFlag{
				Name:  "image_layer",
				Usage: "image layer path",
			},
			&cli.StringFlag{
				Name:  "upper_dir",
				Usage: "upper directory",
			},
			&cli.StringFlag{
				Name:  "work_dir",
				Usage: "work directory",
			},

			// hook
			&cli.StringSliceFlag{
				Name:  "hook-prestart",
				Usage: "(DEPRECATED) prestart hook (format: path[,arg1,arg2,...])",
			},
			&cli.StringSliceFlag{
				Name:  "hook-prestart-env",
				Usage: "(DEPRECATED) prestart hook env (format: KEY=VALUE)",
			},
			&cli.StringSliceFlag{
				Name:  "hook-create-runtime",
				Usage: "createRuntime hook (format: path[,arg1,arg2,...])",
			},
			&cli.StringSliceFlag{
				Name:  "hook-create-runtime-env",
				Usage: "createRuntime hook env (format: KEY=VALUE)",
			},
			&cli.StringSliceFlag{
				Name:  "hook-create-container",
				Usage: "createContainer hook (format: path[,arg1,arg2,...])",
			},
			&cli.StringSliceFlag{
				Name:  "hook-create-container-env",
				Usage: "createContainer hook env (format: KEY=VALUE)",
			},
			&cli.StringSliceFlag{
				Name:  "hook-start-container",
				Usage: "startContainer hook (format: path[,arg1,arg2,...])",
			},
			&cli.StringSliceFlag{
				Name:  "hook-start-container-env",
				Usage: "startContainer hook env (format: KEY=VALUE)",
			},
			&cli.StringSliceFlag{
				Name:  "hook-poststart",
				Usage: "poststart hook (format: path[,arg1,arg2,...])",
			},
			&cli.StringSliceFlag{
				Name:  "hook-poststart-env",
				Usage: "poststart hook env (format: KEY=VALUE)",
			},
			&cli.StringSliceFlag{
				Name:  "hook-stop-container",
				Usage: "stopContainer hook (format: path[,arg1,arg2,...])",
			},
			&cli.StringSliceFlag{
				Name:  "hook-stop-container-env",
				Usage: "stopContainer hook env (format: KEY=VALUE)",
			},
			&cli.StringSliceFlag{
				Name:  "hook-poststop",
				Usage: "poststop hook (format: path[,arg1,arg2,...])",
			},
			&cli.StringSliceFlag{
				Name:  "hook-poststop-env",
				Usage: "poststop hook env (format: KEY=VALUE)",
			},

			&cli.StringFlag{
				Name:  "output",
				Usage: "output path",
				Value: ".",
			},
		},
		Action: runCreateConfigFile,
	}
}

func runCreateConfigFile(ctx *cli.Context) error {
	// create config options
	configOptions, err := createConfigOptions(ctx)
	if err != nil {
		return err
	}

	// build configuration file(config.json)
	if err := spec.CreateConfigFile(ctx.String("output")+"/config.json", configOptions); err != nil {
		return err
	}

	return nil
}

func createConfigOptions(ctx *cli.Context) (spec.ConfigOptions, error) {
	// parse flags and create ConfigOptions
	// rootfs
	rootfs := ctx.String("rootfs")

	// rootless
	rootless, rootlessMode, err := rootlessOptionsFromSpecCLI(ctx)
	if err != nil {
		return spec.ConfigOptions{}, err
	}
	rootlessRootUID := ctx.Int("rootless-root-uid")
	rootlessRootGID := ctx.Int("rootless-root-gid")

	// mount
	mounts, err := parseMountFlag(ctx.StringSlice("mount"))
	if err != nil {
		return spec.ConfigOptions{}, err
	}

	// process
	// cwd
	cwd := ctx.String("cwd")
	// env
	env := ctx.StringSlice("env")
	capAdd := normalizeCapabilityFlag(ctx.StringSlice("cap-add"))
	capDrop := normalizeCapabilityFlag(ctx.StringSlice("cap-drop"))
	// args
	args, err := parseCommandFlag(ctx.String("command"))
	if err != nil {
		return spec.ConfigOptions{}, err
	}

	// namespace
	namespace, err := parseNamespaceFlag(ctx.StringSlice("ns"))
	if err != nil {
		return spec.ConfigOptions{}, err
	}
	namespaceWithPath, err := parseNamespacePathFlag(ctx.StringSlice("ns-path"))
	if err != nil {
		return spec.ConfigOptions{}, err
	}
	namespace = mergeNamespaceOptions(namespace, namespaceWithPath)
	if rootless && !hasNamespaceOption(namespace, "user") {
		namespace = append(namespace, spec.NamespaceOption{Type: "user"})
	}

	// hostname
	hostname := ctx.String("hostname")

	// net
	// host interface name
	hostIfName := ctx.String("host_if_name")
	// bridge interface name
	brIfName := ctx.String("bridge_if_name")
	// interface name
	ifName := ctx.String("if_name")
	// interface address
	ifAddr := ctx.String("if_addr")
	// gateway
	ifGateway := ctx.String("if_gateway")
	// dns
	dns := ctx.StringSlice("dns")

	// image
	// image layer
	imageLayer := ctx.StringSlice("image_layer")
	// upper dir
	upperDir := ctx.String("upper_dir")
	// work dir
	workDir := ctx.String("work_dir")

	// hook
	// prestart
	prestartHook, err := parseHookFlag(ctx.StringSlice("hook-prestart"), ctx.StringSlice("hook-prestart-env"))
	if err != nil {
		return spec.ConfigOptions{}, err
	}
	createRuntimeHook, err := parseHookFlag(ctx.StringSlice("hook-create-runtime"), ctx.StringSlice("hook-create-runtime-env"))
	if err != nil {
		return spec.ConfigOptions{}, err
	}
	createContainerHook, err := parseHookFlag(ctx.StringSlice("hook-create-container"), ctx.StringSlice("hook-create-container-env"))
	if err != nil {
		return spec.ConfigOptions{}, err
	}
	startContainerHook, err := parseHookFlag(ctx.StringSlice("hook-start-container"), ctx.StringSlice("hook-start-container-env"))
	if err != nil {
		return spec.ConfigOptions{}, err
	}
	poststartHook, err := parseHookFlag(ctx.StringSlice("hook-poststart"), ctx.StringSlice("hook-poststart-env"))
	if err != nil {
		return spec.ConfigOptions{}, err
	}
	stopContainerHook, err := parseHookFlag(ctx.StringSlice("hook-stop-container"), ctx.StringSlice("hook-stop-container-env"))
	if err != nil {
		return spec.ConfigOptions{}, err
	}
	poststopHook, err := parseHookFlag(ctx.StringSlice("hook-poststop"), ctx.StringSlice("hook-poststop-env"))
	if err != nil {
		return spec.ConfigOptions{}, err
	}

	return spec.ConfigOptions{
		Rootfs:          rootfs,
		Rootless:        rootless,
		RootlessMode:    rootlessMode,
		RootlessRootUID: rootlessRootUID,
		RootlessRootGID: rootlessRootGID,
		Mounts:          mounts,
		Process: spec.ProcessOption{
			Cwd:     cwd,
			Env:     env,
			Args:    args,
			CapAdd:  capAdd,
			CapDrop: capDrop,
		},
		Namespace: namespace,
		Hostname:  hostname,
		Net: spec.NetOption{
			HostInterface:       hostIfName,
			BridgeInterfaceName: brIfName,
			InterfaceName:       ifName,
			Address:             ifAddr,
			Gateway:             ifGateway,
			Dns:                 dns,
		},
		Image: spec.ImageOption{
			ImageLayer: imageLayer,
			UpperDir:   upperDir,
			WorkDir:    workDir,
		},
		Hooks: spec.HookLifecycleOption{
			Prestart:        prestartHook,
			CreateRuntime:   createRuntimeHook,
			CreateContainer: createContainerHook,
			StartContainer:  startContainerHook,
			Poststart:       poststartHook,
			StopContainer:   stopContainerHook,
			Poststop:        poststopHook,
		},
	}, nil
}

func normalizeCapabilityFlag(items []string) []string {
	out := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		v := strings.TrimSpace(strings.ToUpper(item))
		if v == "" {
			continue
		}
		if !strings.HasPrefix(v, "CAP_") {
			v = "CAP_" + v
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func parseMountFlag(mounts []string) ([]spec.MountOption, error) {
	var mountOption []spec.MountOption
	for _, mount := range mounts {
		parts := strings.SplitN(mount, ":", 3)
		if len(parts) < 2 {
			return []spec.MountOption{}, fmt.Errorf("invalid mount format")
		}

		// source, deestination
		src := parts[0]
		dst := parts[1]

		var mountType string
		// check file type
		fi, err := os.Stat(src)
		if err != nil {
			return []spec.MountOption{}, err
		}
		if fi.IsDir() {
			mountType = ""
		} else {
			mountType = "bind"
		}

		// options
		var opts []string
		if len(parts) == 3 && parts[2] != "" {
			opts = strings.Split(parts[2], ",")
		}

		if !hasBindMountOption(opts) {
			if fi.IsDir() {
				opts = append([]string{"bind"}, opts...)
			} else {
				opts = append([]string{"rbind", "rprivate"}, opts...)
			}
		}

		mountOption = append(mountOption, spec.MountOption{
			Destination: dst,
			Type:        mountType,
			Source:      src,
			Options:     opts,
		})
	}
	return mountOption, nil
}

func hasBindMountOption(options []string) bool {
	for _, opt := range options {
		if opt == "bind" || opt == "rbind" {
			return true
		}
	}
	return false
}

func parseCommandFlag(s string) ([]string, error) {
	args, err := shlex.Split(s)
	if err != nil {
		return []string{}, err
	}
	return args, nil
}

func parseNamespaceFlag(namespaces []string) ([]spec.NamespaceOption, error) {
	var out []spec.NamespaceOption
	for _, ns := range namespaces {
		if ns == "" {
			continue
		}
		if !isValidNamespaceType(ns) {
			return []spec.NamespaceOption{}, fmt.Errorf("invalid namespace type: %q", ns)
		}
		out = append(out, spec.NamespaceOption{Type: ns})
	}
	return out, nil
}

func parseNamespacePathFlag(items []string) ([]spec.NamespaceOption, error) {
	var out []spec.NamespaceOption
	for _, item := range items {
		if item == "" {
			continue
		}
		parts := strings.SplitN(item, "=", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return []spec.NamespaceOption{}, fmt.Errorf("invalid ns-path: %q (expected type=path)", item)
		}
		nsType := parts[0]
		path := parts[1]
		if !isValidNamespaceType(nsType) {
			return []spec.NamespaceOption{}, fmt.Errorf("invalid namespace type: %q", nsType)
		}
		out = append(out, spec.NamespaceOption{Type: nsType, Path: path})
	}
	return out, nil
}

func mergeNamespaceOptions(base []spec.NamespaceOption, overrides []spec.NamespaceOption) []spec.NamespaceOption {
	merged := make(map[string]spec.NamespaceOption)
	for _, ns := range base {
		if ns.Type == "" {
			continue
		}
		merged[ns.Type] = ns
	}
	for _, ns := range overrides {
		if ns.Type == "" {
			continue
		}
		merged[ns.Type] = ns
	}
	out := make([]spec.NamespaceOption, 0, len(merged))
	for _, ns := range merged {
		out = append(out, ns)
	}
	return out
}

func isValidNamespaceType(ns string) bool {
	switch ns {
	case "mount", "network", "uts", "pid", "ipc", "user", "cgroup":
		return true
	default:
		return false
	}
}

func parseHookFlag(command []string, env []string) ([]spec.HookOption, error) {
	var hooks []spec.HookOption

	// command
	for _, v := range command {
		if v == "" {
			continue
		}
		parts := strings.Split(v, ",")
		if len(parts) == 0 || parts[0] == "" {
			return []spec.HookOption{}, fmt.Errorf("invalid hook: %q (path is required)", v)
		}
		h := spec.HookOption{
			Path: parts[0],
		}
		if len(parts) > 1 {
			h.Args = parts[1:]
		}
		hooks = append(hooks, h)
	}
	// env
	for i, v := range env {
		if v == "" {
			continue
		}
		if i >= len(hooks) {
			return []spec.HookOption{}, fmt.Errorf("hook env has no matching hook: %q", v)
		}
		parts := strings.SplitN(v, "=", 2)
		if len(parts) != 2 {
			return []spec.HookOption{}, fmt.Errorf("invalid hook env: %q", v)
		}
		hooks[i].Env = append(hooks[i].Env, v)
	}
	return hooks, nil
}

func hasNamespaceOption(items []spec.NamespaceOption, nsType string) bool {
	for _, item := range items {
		if item.Type == nsType {
			return true
		}
	}
	return false
}
