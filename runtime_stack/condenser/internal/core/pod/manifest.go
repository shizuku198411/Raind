package pod

import (
	"bytes"
	"fmt"
	"io"

	"condenser/internal/store/psm"

	"gopkg.in/yaml.v3"
)

type PodManifest struct {
	Kind        string
	Name        string
	Namespace   string
	Labels      map[string]string
	Annotations map[string]string
	Containers  []psm.ContainerTemplateSpec
	Replicas    int
	Selector    map[string]string
}

type manifestMeta struct {
	Name        string            `yaml:"name"`
	Namespace   string            `yaml:"namespace"`
	Labels      map[string]string `yaml:"labels"`
	Annotations map[string]string `yaml:"annotations"`
}

type podManifestSpec struct {
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
		Replicas int        `yaml:"replicas"`
		Template rsTemplate `yaml:"template"`
	} `yaml:"spec"`
}

type containerManifest struct {
	Name         string                `yaml:"name"`
	Image        string                `yaml:"image"`
	Command      []string              `yaml:"command"`
	Args         []string              `yaml:"args"`
	Env          []manifestEnvVar      `yaml:"env"`
	Ports        []manifestPort        `yaml:"ports"`
	Mount        []string              `yaml:"mount"`
	VolumeMounts []manifestVolumeMount `yaml:"volumeMounts"`
	Tty          bool                  `yaml:"tty"`
}

type manifestEnvVar struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
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
			manifest, err := buildPodManifest(pod.Metadata, pod.Spec.Containers, pod.Spec.Volumes)
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
			manifest, err := buildPodManifest(meta, rs.Spec.Template.Spec.Containers, rs.Spec.Template.Spec.Volumes)
			if err != nil {
				return nil, err
			}
			manifest.Kind = "ReplicaSet"
			manifest.Replicas = rs.Spec.Replicas
			if manifest.Replicas == 0 {
				manifest.Replicas = 1
			}
			if rs.Spec.Selector.MatchLabels != nil {
				manifest.Selector = rs.Spec.Selector.MatchLabels
			} else {
				manifest.Selector = manifest.Labels
			}
			if manifest.Name == "" {
				return nil, fmt.Errorf("replicaset template name is required")
			}
			result = append(result, manifest)
		default:
			return nil, fmt.Errorf("unsupported kind: %s", kind)
		}
	}

	return result, nil
}

func buildPodManifest(meta manifestMeta, containers []containerManifest, volumes []manifestVolume) (PodManifest, error) {
	if meta.Namespace == "" {
		meta.Namespace = "default"
	}
	volumeHostPath := map[string]string{}
	for _, v := range volumes {
		if v.Name == "" {
			continue
		}
		if v.HostPath.Path == "" {
			return PodManifest{}, fmt.Errorf("volume %q: only hostPath volumes are supported", v.Name)
		}
		volumeHostPath[v.Name] = v.HostPath.Path
	}

	specs := make([]psm.ContainerTemplateSpec, 0, len(containers))
	for _, c := range containers {
		cmd := c.Command
		if len(c.Args) > 0 {
			cmd = append(append([]string{}, c.Command...), c.Args...)
		}
		envs := make([]string, 0, len(c.Env))
		for _, e := range c.Env {
			if e.Name == "" {
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
			Tty:     c.Tty,
		})
	}
	return PodManifest{
		Name:        meta.Name,
		Namespace:   meta.Namespace,
		Labels:      meta.Labels,
		Annotations: meta.Annotations,
		Containers:  specs,
	}, nil
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
