package service

import (
	"bytes"
	"fmt"
	"io"

	"condenser/internal/store/ssm"

	"gopkg.in/yaml.v3"
)

type ServiceManifest struct {
	Name      string
	Namespace string
	Selector  map[string]string
	Ports     []ssm.ServicePort
}

type serviceMeta struct {
	Name      string            `yaml:"name"`
	Namespace string            `yaml:"namespace"`
	Labels    map[string]string `yaml:"labels"`
}

type servicePortManifest struct {
	Port       int    `yaml:"port"`
	TargetPort int    `yaml:"targetPort"`
	Protocol   string `yaml:"protocol"`
}

type serviceSpec struct {
	Selector map[string]string     `yaml:"selector"`
	Ports    []servicePortManifest `yaml:"ports"`
}

type serviceManifest struct {
	APIVersion string      `yaml:"apiVersion"`
	Kind       string      `yaml:"kind"`
	Metadata   serviceMeta `yaml:"metadata"`
	Spec       serviceSpec `yaml:"spec"`
}

func DecodeK8sServiceManifest(body []byte) (ServiceManifest, error) {
	dec := yaml.NewDecoder(bytes.NewReader(body))
	for {
		var raw map[string]any
		if err := dec.Decode(&raw); err != nil {
			if err == io.EOF {
				break
			}
			return ServiceManifest{}, err
		}
		if len(raw) == 0 {
			continue
		}
		kind, _ := raw["kind"].(string)
		if kind == "" {
			return ServiceManifest{}, fmt.Errorf("kind is required")
		}
		if kind != "Service" {
			return ServiceManifest{}, fmt.Errorf("unsupported kind: %s", kind)
		}
		rawBytes, err := yaml.Marshal(raw)
		if err != nil {
			return ServiceManifest{}, err
		}
		var svc serviceManifest
		if err := yaml.Unmarshal(rawBytes, &svc); err != nil {
			return ServiceManifest{}, err
		}
		return buildServiceManifest(svc), nil
	}
	return ServiceManifest{}, fmt.Errorf("service manifest not found")
}

func buildServiceManifest(in serviceManifest) ServiceManifest {
	if in.Metadata.Namespace == "" {
		in.Metadata.Namespace = "default"
	}
	ports := make([]ssm.ServicePort, 0, len(in.Spec.Ports))
	for _, p := range in.Spec.Ports {
		if p.Port == 0 {
			continue
		}
		target := p.TargetPort
		if target == 0 {
			target = p.Port
		}
		proto := p.Protocol
		if proto == "" {
			proto = "tcp"
		}
		ports = append(ports, ssm.ServicePort{
			Port:       p.Port,
			TargetPort: target,
			Protocol:   proto,
		})
	}
	return ServiceManifest{
		Name:      in.Metadata.Name,
		Namespace: in.Metadata.Namespace,
		Selector:  in.Spec.Selector,
		Ports:     ports,
	}
}
