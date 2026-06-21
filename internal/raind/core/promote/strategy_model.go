package promote

import (
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type StrategyStringList []string

func (s *StrategyStringList) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		return nil
	}
	switch value.Kind {
	case yaml.SequenceNode:
		out := make([]string, 0, len(value.Content))
		for _, item := range value.Content {
			out = append(out, strings.TrimSpace(item.Value))
		}
		*s = out
		return nil
	case yaml.MappingNode:
		out := make([]string, 0, len(value.Content)/2)
		for i := 0; i+1 < len(value.Content); i += 2 {
			key := strings.TrimSpace(value.Content[i].Value)
			val := strings.TrimSpace(value.Content[i+1].Value)
			if key != "" {
				out = append(out, key+"="+val)
			}
		}
		*s = out
		return nil
	case yaml.ScalarNode:
		text := strings.TrimSpace(value.Value)
		if text == "" {
			*s = nil
			return nil
		}
		*s = []string{text}
		return nil
	default:
		return fmt.Errorf("unsupported string list yaml node kind %d", value.Kind)
	}
}

type StrategyDuration time.Duration

func (d *StrategyDuration) UnmarshalYAML(value *yaml.Node) error {
	if value == nil || strings.TrimSpace(value.Value) == "" {
		return nil
	}
	parsed, err := time.ParseDuration(strings.TrimSpace(value.Value))
	if err == nil {
		*d = StrategyDuration(parsed)
		return nil
	}
	var numeric int64
	if decodeErr := value.Decode(&numeric); decodeErr == nil {
		*d = StrategyDuration(time.Duration(numeric))
		return nil
	}
	return err
}

func (d StrategyDuration) Duration() time.Duration {
	return time.Duration(d)
}

type StrategyOptions struct {
	File        string
	Until       string
	DryRun      bool
	IngressHost string
	Namespace   string
}

type StrategySpec struct {
	APIVersion string              `yaml:"apiVersion"`
	Kind       string              `yaml:"kind"`
	Metadata   StrategyMetadata    `yaml:"metadata"`
	Source     StrategySource      `yaml:"source"`
	Containers []StrategyContainer `yaml:"containers"`
	Stages     StrategyStages      `yaml:"stages"`
	Outputs    StrategyOutputs     `yaml:"outputs"`
}

type StrategyMetadata struct {
	Name string `yaml:"name"`
}

type StrategySource struct {
	Mode       string              `yaml:"mode"`
	Containers []StrategyContainer `yaml:"containers"`
}

type StrategyContainer struct {
	Name            string             `yaml:"name"`
	Image           string             `yaml:"image"`
	Command         StrategyStringList `yaml:"command"`
	Network         string             `yaml:"network"`
	Volume          StrategyStringList `yaml:"volume"`
	Mount           StrategyStringList `yaml:"mount"`
	Publish         StrategyStringList `yaml:"publish"`
	Ports           StrategyStringList `yaml:"ports"`
	Device          StrategyStringList `yaml:"device"`
	Env             StrategyStringList `yaml:"env"`
	CapAdd          StrategyStringList `yaml:"capAdd"`
	CapDrop         StrategyStringList `yaml:"capDrop"`
	SecurityProfile string             `yaml:"securityProfile"`
	Tty             bool               `yaml:"tty"`
	DependsOn       StrategyStringList `yaml:"dependsOn"`
}

type StrategyStages struct {
	Container StrategyStage `yaml:"container"`
	Bottle    StrategyStage `yaml:"bottle"`
	Resources StrategyStage `yaml:"resources"`
}

type StrategyStage struct {
	Apply   StrategyApply   `yaml:"apply"`
	Checks  StrategyChecks  `yaml:"checks"`
	Promote StrategyPromote `yaml:"promote"`
	Health  []StrategyCheck `yaml:"healthChecks"`
}

type StrategyApply struct {
	File string `yaml:"file"`
	Path string `yaml:"path"`
}

type StrategyChecks struct {
	Runtime     []StrategyCheck `yaml:"runtime"`
	Application []StrategyCheck `yaml:"application"`
}

type StrategyCheck struct {
	Name     string              `yaml:"name"`
	Type     string              `yaml:"type"`
	Target   string              `yaml:"target"`
	Expect   StrategyCheckExpect `yaml:"expect"`
	Timeout  StrategyDuration    `yaml:"timeout"`
	Interval StrategyDuration    `yaml:"interval"`
}

type StrategyCheckExpect struct {
	State        string `yaml:"state"`
	Status       int    `yaml:"status"`
	BodyContains string `yaml:"bodyContains"`
}

type StrategyPromote struct {
	To     string `yaml:"to"`
	Output string `yaml:"output"`
}

type StrategyOutputs struct {
	Bottle    string `yaml:"bottle"`
	Resources string `yaml:"resources"`
}

type StrategyRunResult struct {
	Name            string
	BottleOutput    string
	ResourcesOutput string
	Steps           []StrategyStepResult
}

type StrategyStepResult struct {
	Name   string
	Status string
}
