package spec

import (
	"path/filepath"
	"raind/internal/droplet/oci"
	"raind/internal/droplet/utils"
	"slices"
	"strings"
)

func buildRootSpec(opts ConfigOptions) RootObject {
	return RootObject{
		Path: opts.Rootfs,
	}
}

func buildMountSpec(opts ConfigOptions) []MountObject {
	var mounts = []MountObject{}

	// user mounts
	for _, user_mount := range opts.Mounts {
		mounts = append(mounts, MountObject{
			Destination: user_mount.Destination,
			Type:        user_mount.Type,
			Source:      user_mount.Source,
			Options:     user_mount.Options,
		})
	}

	return mounts
}

func buildProcessEnvSpec(specEnv []string) []string {
	// env preset
	envPreset := map[string]string{
		"PATH": "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"TERM": "xterm-256color",
		"LANG": "C.UTF-8",
		"HOME": "/root",
	}

	// []string -> map
	env := map[string]string{}
	for _, kv := range specEnv {
		k, v, ok := strings.Cut(kv, "=")
		if ok && k != "" {
			env[k] = v
		}
	}
	// set if the preset env is not exist
	for k, v := range envPreset {
		if _, ok := env[k]; !ok || strings.TrimSpace(env[k]) == "" {
			env[k] = v
		}
	}
	// map -> []string
	newEnv := make([]string, 0, len(env))
	for k, v := range env {
		newEnv = append(newEnv, k+"="+v)
	}
	return newEnv
}

func buildProcessSpec(opts ConfigOptions, profile SecurityProfile) ProcessObject {
	finalCaps := mergeCapabilities(profile.Capabilities.Base, opts.Process.CapAdd, opts.Process.CapDrop)

	return ProcessObject{
		Cwd:  opts.Process.Cwd,
		Env:  buildProcessEnvSpec(opts.Process.Env),
		Args: opts.Process.Args,
		Capabilities: CapabilityObject{
			Bounding:  slices.Clone(finalCaps),
			Effective: slices.Clone(finalCaps),
			Permitted: slices.Clone(finalCaps),
		},
	}
}

func normalizeCapabilityNames(caps []string) []string {
	out := make([]string, 0, len(caps))
	seen := map[string]struct{}{}
	for _, c := range caps {
		n := strings.TrimSpace(strings.ToUpper(c))
		if n == "" {
			continue
		}
		if !strings.HasPrefix(n, "CAP_") {
			n = "CAP_" + n
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	return out
}

func mergeCapabilities(base []string, capAdd []string, capDrop []string) []string {
	final := normalizeCapabilityNames(base)
	dropSet := map[string]struct{}{}
	for _, c := range normalizeCapabilityNames(capDrop) {
		dropSet[c] = struct{}{}
	}

	filtered := make([]string, 0, len(final))
	for _, c := range final {
		if _, dropped := dropSet[c]; dropped {
			continue
		}
		filtered = append(filtered, c)
	}

	existing := map[string]struct{}{}
	for _, c := range filtered {
		existing[c] = struct{}{}
	}
	for _, c := range normalizeCapabilityNames(capAdd) {
		if _, dropped := dropSet[c]; dropped {
			continue
		}
		if _, ok := existing[c]; ok {
			continue
		}
		existing[c] = struct{}{}
		filtered = append(filtered, c)
	}
	return filtered
}

func buildLinuxSpec(opts ConfigOptions, profile SecurityProfile) LinuxSpecObject {
	var linuxSpec = LinuxSpecObject{
		Resources: ResourceObject{
			Memory: MemoryObject{ // memory limit: 1024MiB
				Limit: 1073741824,
			},
			Cpu: CpuObject{ // cpu limit: 80%
				Period: 100000,
				Quota:  80000,
			},
		},
		Seccomp:         cloneSeccompObject(profile.Seccomp),
		AppArmorProfile: profile.AppArmorProfile,
		Namespaces:      []NamespaceObject{},
	}

	for _, ns := range opts.Namespace {
		linuxSpec.Namespaces = append(linuxSpec.Namespaces, NamespaceObject{
			Type: ns.Type,
			Path: ns.Path,
		})
	}

	return linuxSpec
}

func buildNetSpec(opts ConfigOptions) NetConfigObject {
	return NetConfigObject{
		HostInterface:   opts.Net.HostInterface,
		BridgeInterface: opts.Net.BridgeInterfaceName,
		Interface: InterfaceObject{
			Name: opts.Net.InterfaceName,
			IPv4: IPv4Object{
				Address: opts.Net.Address,
				Gateway: opts.Net.Gateway,
			},
			Dns: DnsObject{
				Servers: opts.Net.Dns,
			},
		},
	}
}

func buildImageSpec(opts ConfigOptions) ImageConfigObject {
	return ImageConfigObject{
		RootfsType: "overlay",
		ImageLayer: opts.Image.ImageLayer,
		UpperDir:   opts.Image.UpperDir,
		WorkDir:    opts.Image.WorkDir,
	}
}

func buildHookSpec(opts ConfigOptions) HookLifecycleObject {
	var hookLifeCycleObject HookLifecycleObject

	// prestart
	for _, h := range opts.Hooks.Prestart {
		hookLifeCycleObject.Prestart = append(hookLifeCycleObject.Prestart,
			HookObject{
				Path:    h.Path,
				Args:    h.Args,
				Env:     h.Env,
				Timeout: h.Timeout,
			},
		)
	}
	// crateRuntime
	for _, h := range opts.Hooks.CreateRuntime {
		hookLifeCycleObject.CreateRuntime = append(hookLifeCycleObject.CreateRuntime,
			HookObject{
				Path:    h.Path,
				Args:    h.Args,
				Env:     h.Env,
				Timeout: h.Timeout,
			},
		)
	}
	// crateContainer
	for _, h := range opts.Hooks.CreateContainer {
		hookLifeCycleObject.CreateContainer = append(hookLifeCycleObject.CreateContainer,
			HookObject{
				Path:    h.Path,
				Args:    h.Args,
				Env:     h.Env,
				Timeout: h.Timeout,
			},
		)
	}
	// startContainer
	for _, h := range opts.Hooks.StartContainer {
		hookLifeCycleObject.StartContainer = append(hookLifeCycleObject.StartContainer,
			HookObject{
				Path:    h.Path,
				Args:    h.Args,
				Env:     h.Env,
				Timeout: h.Timeout,
			},
		)
	}
	// poststart
	for _, h := range opts.Hooks.Poststart {
		hookLifeCycleObject.Poststart = append(hookLifeCycleObject.Poststart,
			HookObject{
				Path:    h.Path,
				Args:    h.Args,
				Env:     h.Env,
				Timeout: h.Timeout,
			},
		)
	}
	// stopContainer
	for _, h := range opts.Hooks.StopContainer {
		hookLifeCycleObject.StopContainer = append(hookLifeCycleObject.StopContainer,
			HookObject{
				Path:    h.Path,
				Args:    h.Args,
				Env:     h.Env,
				Timeout: h.Timeout,
			},
		)
	}
	// poststop
	for _, h := range opts.Hooks.Poststop {
		hookLifeCycleObject.Poststop = append(hookLifeCycleObject.Poststop,
			HookObject{
				Path:    h.Path,
				Args:    h.Args,
				Env:     h.Env,
				Timeout: h.Timeout,
			},
		)
	}

	return hookLifeCycleObject
}

func buildRootlessSpec(opts ConfigOptions) RootlessConfigObject {
	mode := opts.RootlessMode
	if mode == "" {
		mode = RootlessModeShiftedRoot
	}
	return RootlessConfigObject{
		Enabled:     opts.Rootless,
		Mode:        mode,
		HostRootUID: opts.RootlessRootUID,
		HostRootGID: opts.RootlessRootGID,
	}
}

func buildAnnotationSpec(opts ConfigOptions) AnnotationObject {
	netSpec, _ := utils.JsonToString(buildNetSpec(opts))
	imageSpec, _ := utils.JsonToString(buildImageSpec(opts))
	rootlessSpec := ""
	if opts.Rootless {
		rootlessSpec, _ = utils.JsonToString(buildRootlessSpec(opts))
	}
	return AnnotationObject{
		Version:  oci.AnnotationVersion,
		Net:      netSpec,
		Image:    imageSpec,
		Rootless: rootlessSpec,
	}
}

func buildSpec(opts ConfigOptions) (Spec, error) {
	profile, err := ResolveSecurityOption(opts.Security)
	if err != nil {
		return Spec{}, err
	}

	ociVersion := oci.OCIVersion

	// root path
	root := buildRootSpec(opts)

	// mounts
	mounts := buildMountSpec(opts)

	// process
	process := buildProcessSpec(opts, profile)

	// hostname
	hostname := opts.Hostname

	// linux spec
	linuxSpec := buildLinuxSpec(opts, profile)

	// hook spec
	hookSpec := buildHookSpec(opts)

	// annotation
	annotation := buildAnnotationSpec(opts)

	return Spec{
		OciVersion:  ociVersion,
		Root:        root,
		Mounts:      mounts,
		Process:     process,
		Hostname:    hostname,
		LinuxSpec:   linuxSpec,
		Hooks:       hookSpec,
		Annotations: annotation,
	}, nil
}

func CreateConfigFile(path string, opts ConfigOptions) error {
	// build spec
	spec, err := buildSpec(opts)
	if err != nil {
		return err
	}

	// write spec to file
	configPath := filepath.Join(path)
	if err := utils.WriteJsonToFile(configPath, spec); err != nil {
		return err
	}
	return nil
}

func LoadConfigFile(path string) (Spec, error) {
	var spec Spec

	if err := utils.ReadJsonFile(path, &spec); err != nil {
		return Spec{}, err
	}

	return spec, nil
}
