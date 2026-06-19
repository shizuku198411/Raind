package pod

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"raind/internal/condenser/store/psm"

	"gopkg.in/yaml.v3"
)

type PodManifest struct {
	Kind             string
	Name             string
	Namespace        string
	Labels           map[string]string
	Annotations      map[string]string
	Containers       []psm.ContainerTemplateSpec
	ConfigMapEnvFrom []ContainerConfigMapRef
	ConfigMapEnvKeys []ContainerConfigMapKeyRef
	SecretEnvFrom    []ContainerSecretRef
	SecretEnvKeys    []ContainerSecretKeyRef
	Rootless         bool
	Replicas         int
	Selector         map[string]string
}

type ContainerConfigMapRef struct {
	ContainerIndex int
	Name           string
}

type ContainerConfigMapKeyRef struct {
	ContainerIndex int
	EnvName        string
	Name           string
	Key            string
}

type ContainerSecretRef struct {
	ContainerIndex int
	Name           string
}

type ContainerSecretKeyRef struct {
	ContainerIndex int
	EnvName        string
	Name           string
	Key            string
}

type manifestMeta struct {
	Name        string            `yaml:"name"`
	Namespace   string            `yaml:"namespace"`
	Labels      map[string]string `yaml:"labels"`
	Annotations map[string]string `yaml:"annotations"`
}

type podManifestSpec struct {
	HostUsers  *bool               `yaml:"hostUsers"`
	Containers []containerManifest `yaml:"containers"`
	Volumes    []manifestVolume    `yaml:"volumes"`
}

type podManifest struct {
	APIVersion string          `yaml:"apiVersion"`
	Kind       string          `yaml:"kind"`
	Metadata   manifestMeta    `yaml:"metadata"`
	Spec       podManifestSpec `yaml:"spec"`
}

type rsTemplate struct {
	Metadata manifestMeta    `yaml:"metadata"`
	Spec     podManifestSpec `yaml:"spec"`
}

type replicaSetManifest struct {
	APIVersion string       `yaml:"apiVersion"`
	Kind       string       `yaml:"kind"`
	Metadata   manifestMeta `yaml:"metadata"`
	Spec       struct {
		Selector struct {
			MatchLabels map[string]string `yaml:"matchLabels"`
		} `yaml:"selector"`
		Replicas *int       `yaml:"replicas"`
		Template rsTemplate `yaml:"template"`
	} `yaml:"spec"`
}

type deploymentManifest struct {
	APIVersion string       `yaml:"apiVersion"`
	Kind       string       `yaml:"kind"`
	Metadata   manifestMeta `yaml:"metadata"`
	Spec       struct {
		Selector struct {
			MatchLabels map[string]string `yaml:"matchLabels"`
		} `yaml:"selector"`
		Replicas *int       `yaml:"replicas"`
		Template rsTemplate `yaml:"template"`
	} `yaml:"spec"`
}

type containerManifest struct {
	Name         string                `yaml:"name"`
	Image        string                `yaml:"image"`
	Command      []string              `yaml:"command"`
	Args         []string              `yaml:"args"`
	Env          []manifestEnvVar      `yaml:"env"`
	EnvFrom      []manifestEnvFrom     `yaml:"envFrom"`
	Ports        []manifestPort        `yaml:"ports"`
	Mount        []string              `yaml:"mount"`
	SecurityCtx  manifestSecurityCtx   `yaml:"securityContext"`
	VolumeMounts []manifestVolumeMount `yaml:"volumeMounts"`
	Tty          bool                  `yaml:"tty"`
}

type manifestSecurityCtx struct {
	Capabilities manifestCapabilities `yaml:"capabilities"`
}

type manifestCapabilities struct {
	Add  []string `yaml:"add"`
	Drop []string `yaml:"drop"`
}

type manifestEnvVar struct {
	Name      string               `yaml:"name"`
	Value     string               `yaml:"value"`
	ValueFrom manifestEnvValueFrom `yaml:"valueFrom"`
}

type manifestEnvFrom struct {
	ConfigMapRef manifestConfigMapRef `yaml:"configMapRef"`
	SecretRef    manifestSecretRef    `yaml:"secretRef"`
}

type manifestEnvValueFrom struct {
	ConfigMapKeyRef manifestConfigMapKeyRef `yaml:"configMapKeyRef"`
	SecretKeyRef    manifestSecretKeyRef    `yaml:"secretKeyRef"`
}

type manifestConfigMapRef struct {
	Name string `yaml:"name"`
}

type manifestConfigMapKeyRef struct {
	Name string `yaml:"name"`
	Key  string `yaml:"key"`
}

type manifestSecretRef struct {
	Name string `yaml:"name"`
}

type manifestSecretKeyRef struct {
	Name string `yaml:"name"`
	Key  string `yaml:"key"`
}

type manifestPort struct {
	ContainerPort int `yaml:"containerPort"`
	HostPort      int `yaml:"hostPort"`
}

type manifestVolume struct {
	Name     string           `yaml:"name"`
	HostPath manifestHostPath `yaml:"hostPath"`
}

type manifestHostPath struct {
	Path string `yaml:"path"`
	Type string `yaml:"type"`
}

type manifestVolumeMount struct {
	Name      string `yaml:"name"`
	MountPath string `yaml:"mountPath"`
	ReadOnly  bool   `yaml:"readOnly"`
}

func DecodeK8sManifests(body []byte) ([]PodManifest, error) {
	dec := yaml.NewDecoder(bytes.NewReader(body))

	var result []PodManifest
	for {
		var raw map[string]any
		if err := dec.Decode(&raw); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if len(raw) == 0 {
			continue
		}
		kind, _ := raw["kind"].(string)
		if kind == "" {
			return nil, fmt.Errorf("kind is required")
		}

		rawBytes, err := yaml.Marshal(raw)
		if err != nil {
			return nil, err
		}

		switch kind {
		case "Pod":
			var pod podManifest
			if err := yaml.Unmarshal(rawBytes, &pod); err != nil {
				return nil, err
			}
			manifest, err := buildPodManifest(pod.Metadata, pod.Spec)
			if err != nil {
				return nil, err
			}
			manifest.Kind = "Pod"
			if manifest.Name == "" {
				return nil, fmt.Errorf("pod name is required")
			}
			result = append(result, manifest)
		case "ReplicaSet":
			var rs replicaSetManifest
			if err := yaml.Unmarshal(rawBytes, &rs); err != nil {
				return nil, err
			}
			meta := rs.Spec.Template.Metadata
			if meta.Name == "" {
				meta.Name = rs.Metadata.Name
			}
			manifest, err := buildPodManifest(meta, rs.Spec.Template.Spec)
			if err != nil {
				return nil, err
			}
			manifest.Kind = "ReplicaSet"
			manifest.Replicas = 1
			if rs.Spec.Replicas != nil {
				manifest.Replicas = *rs.Spec.Replicas
			}
			if manifest.Replicas < 0 {
				return nil, fmt.Errorf("replicas must be >= 0")
			}
			if manifest.Replicas == 0 && rs.Spec.Replicas == nil {
				manifest.Replicas = 1
			}
			if rs.Spec.Selector.MatchLabels != nil {
				manifest.Selector = rs.Spec.Selector.MatchLabels
			} else {
				manifest.Selector = manifest.Labels
			}
			manifest.Name = rs.Metadata.Name
			manifest.Namespace = rs.Metadata.Namespace
			if manifest.Namespace == "" {
				manifest.Namespace = "default"
			}
			if !selectorMatchesLabels(manifest.Selector, manifest.Labels) {
				return nil, fmt.Errorf("replicaset selector must match template labels")
			}
			if manifest.Name == "" {
				return nil, fmt.Errorf("replicaset name is required")
			}
			result = append(result, manifest)
		case "Deployment":
			var deploy deploymentManifest
			if err := yaml.Unmarshal(rawBytes, &deploy); err != nil {
				return nil, err
			}
			meta := deploy.Spec.Template.Metadata
			if meta.Name == "" {
				meta.Name = deploy.Metadata.Name
			}
			manifest, err := buildPodManifest(meta, deploy.Spec.Template.Spec)
			if err != nil {
				return nil, err
			}
			manifest.Kind = "Deployment"
			manifest.Replicas = 1
			if deploy.Spec.Replicas != nil {
				manifest.Replicas = *deploy.Spec.Replicas
			}
			if manifest.Replicas < 0 {
				return nil, fmt.Errorf("replicas must be >= 0")
			}
			if manifest.Replicas == 0 && deploy.Spec.Replicas == nil {
				manifest.Replicas = 1
			}
			if deploy.Spec.Selector.MatchLabels != nil {
				manifest.Selector = deploy.Spec.Selector.MatchLabels
			} else {
				manifest.Selector = manifest.Labels
			}
			manifest.Name = deploy.Metadata.Name
			manifest.Namespace = deploy.Metadata.Namespace
			if manifest.Namespace == "" {
				manifest.Namespace = "default"
			}
			if !selectorMatchesLabels(manifest.Selector, manifest.Labels) {
				return nil, fmt.Errorf("deployment selector must match template labels")
			}
			if manifest.Name == "" {
				return nil, fmt.Errorf("deployment name is required")
			}
			result = append(result, manifest)
		default:
			return nil, fmt.Errorf("unsupported kind: %s", kind)
		}
	}

	return result, nil
}

func selectorMatchesLabels(selector, labels map[string]string) bool {
	if len(selector) == 0 {
		return true
	}
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}
	return true
}

func buildHostPathVolumeMap(volumes []manifestVolume) (map[string]string, error) {
	volumeHostPath := map[string]string{}
	for _, v := range volumes {
		if v.Name == "" {
			continue
		}
		if _, exists := volumeHostPath[v.Name]; exists {
			return nil, fmt.Errorf("volume %q: duplicate volume name", v.Name)
		}
		if v.HostPath.Path == "" {
			return nil, fmt.Errorf("volume %q: only hostPath volumes are supported", v.Name)
		}
		if !filepath.IsAbs(v.HostPath.Path) {
			return nil, fmt.Errorf("volume %q: hostPath.path must be absolute", v.Name)
		}

		switch v.HostPath.Type {
		case "", "Directory":
			info, err := os.Stat(v.HostPath.Path)
			if err != nil {
				return nil, fmt.Errorf("volume %q: hostPath directory %q is not available: %w", v.Name, v.HostPath.Path, err)
			}
			if !info.IsDir() {
				return nil, fmt.Errorf("volume %q: hostPath %q is not a directory", v.Name, v.HostPath.Path)
			}
		case "DirectoryOrCreate":
			if err := os.MkdirAll(v.HostPath.Path, 0755); err != nil {
				return nil, fmt.Errorf("volume %q: create hostPath directory %q: %w", v.Name, v.HostPath.Path, err)
			}
			info, err := os.Stat(v.HostPath.Path)
			if err != nil {
				return nil, fmt.Errorf("volume %q: stat hostPath directory %q: %w", v.Name, v.HostPath.Path, err)
			}
			if !info.IsDir() {
				return nil, fmt.Errorf("volume %q: hostPath %q is not a directory", v.Name, v.HostPath.Path)
			}
		default:
			return nil, fmt.Errorf("volume %q: unsupported hostPath.type %q", v.Name, v.HostPath.Type)
		}
		volumeHostPath[v.Name] = v.HostPath.Path
	}
	return volumeHostPath, nil
}

func buildPodManifest(meta manifestMeta, podSpec podManifestSpec) (PodManifest, error) {
	if meta.Namespace == "" {
		meta.Namespace = "default"
	}
	volumeHostPath, err := buildHostPathVolumeMap(podSpec.Volumes)
	if err != nil {
		return PodManifest{}, err
	}

	specs := make([]psm.ContainerTemplateSpec, 0, len(podSpec.Containers))
	var configMapEnvFrom []ContainerConfigMapRef
	var configMapEnvKeys []ContainerConfigMapKeyRef
	var secretEnvFrom []ContainerSecretRef
	var secretEnvKeys []ContainerSecretKeyRef
	for i, c := range podSpec.Containers {
		cmd := c.Command
		if len(c.Args) > 0 {
			cmd = append(append([]string{}, c.Command...), c.Args...)
		}
		envs := make([]string, 0, len(c.Env))
		for _, ref := range c.EnvFrom {
			if ref.ConfigMapRef.Name != "" {
				configMapEnvFrom = append(configMapEnvFrom, ContainerConfigMapRef{ContainerIndex: i, Name: ref.ConfigMapRef.Name})
			}
			if ref.SecretRef.Name != "" {
				secretEnvFrom = append(secretEnvFrom, ContainerSecretRef{ContainerIndex: i, Name: ref.SecretRef.Name})
			}
		}
		for _, e := range c.Env {
			if e.Name == "" {
				continue
			}
			if e.ValueFrom.ConfigMapKeyRef.Name != "" || e.ValueFrom.ConfigMapKeyRef.Key != "" {
				configMapEnvKeys = append(configMapEnvKeys, ContainerConfigMapKeyRef{
					ContainerIndex: i,
					EnvName:        e.Name,
					Name:           e.ValueFrom.ConfigMapKeyRef.Name,
					Key:            e.ValueFrom.ConfigMapKeyRef.Key,
				})
				continue
			}
			if e.ValueFrom.SecretKeyRef.Name != "" || e.ValueFrom.SecretKeyRef.Key != "" {
				secretEnvKeys = append(secretEnvKeys, ContainerSecretKeyRef{
					ContainerIndex: i,
					EnvName:        e.Name,
					Name:           e.ValueFrom.SecretKeyRef.Name,
					Key:            e.ValueFrom.SecretKeyRef.Key,
				})
				continue
			}
			envs = append(envs, e.Name+"="+e.Value)
		}
		ports := make([]string, 0, len(c.Ports))
		for _, p := range c.Ports {
			if p.ContainerPort == 0 {
				continue
			}
			if p.HostPort != 0 {
				ports = append(ports, fmt.Sprintf("%d:%d", p.HostPort, p.ContainerPort))
			}
		}
		mounts := append([]string{}, c.Mount...)
		for _, vm := range c.VolumeMounts {
			if vm.Name == "" || vm.MountPath == "" {
				continue
			}
			if !filepath.IsAbs(vm.MountPath) {
				return PodManifest{}, fmt.Errorf("container %q: volumeMount %q mountPath must be absolute", c.Name, vm.Name)
			}
			hostPath, ok := volumeHostPath[vm.Name]
			if !ok {
				return PodManifest{}, fmt.Errorf("container %q: volume %q not found", c.Name, vm.Name)
			}
			m := hostPath + ":" + vm.MountPath
			if vm.ReadOnly {
				m += ":ro"
			}
			mounts = append(mounts, m)
		}
		specs = append(specs, psm.ContainerTemplateSpec{
			Name:    c.Name,
			Image:   c.Image,
			Command: cmd,
			Env:     envs,
			Port:    ports,
			Mount:   mounts,
			CapAdd:  c.SecurityCtx.Capabilities.Add,
			CapDrop: c.SecurityCtx.Capabilities.Drop,
			Tty:     c.Tty,
		})
	}
	return PodManifest{
		Name:             meta.Name,
		Namespace:        meta.Namespace,
		Labels:           meta.Labels,
		Annotations:      meta.Annotations,
		Containers:       specs,
		ConfigMapEnvFrom: configMapEnvFrom,
		ConfigMapEnvKeys: configMapEnvKeys,
		SecretEnvFrom:    secretEnvFrom,
		SecretEnvKeys:    secretEnvKeys,
		Rootless:         rootlessFromHostUsers(podSpec.HostUsers),
	}, nil
}

func rootlessFromHostUsers(hostUsers *bool) bool {
	return hostUsers != nil && !*hostUsers
}

func mergeLabels(base, extra map[string]string) map[string]string {
	if base == nil && extra == nil {
		return nil
	}
	out := map[string]string{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}
