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
	File          string
	Until         string
	DryRun        bool
	IngressHost   string
	Namespace     string
	ProgressStart StrategyProgressStartFunc
	Progress      StrategyProgressFunc
	InternalLog   StrategyInternalLogFunc
}

type StrategyProgressStartFunc func(name string)

type StrategyProgressFunc func(event StrategyProgressEvent)

type StrategyInternalLogFunc func(event StrategyInternalLogEvent)

type StrategyProgressEvent struct {
	Name   string
	Status string
	Index  int
	Total  int
	Done   bool
}

type StrategyInternalLogEvent struct {
	Step string
	Line string
	Done bool
}

type StrategySpec struct {
	APIVersion string              `yaml:"apiVersion"`
	Kind       string              `yaml:"kind"`
	Metadata   StrategyMetadata    `yaml:"metadata"`
	Source     StrategySource      `yaml:"source"`
	Containers []StrategyContainer `yaml:"containers"`
	Stages     StrategyStages      `yaml:"stages"`
}

type StrategyMetadata struct {
	Name string `yaml:"name"`
}

type StrategySource struct {
	Mode       string              `yaml:"mode"`
	Containers []StrategyContainer `yaml:"containers"`
	Policies   []StrategyPolicy    `yaml:"policies"`
}

type StrategyPolicy struct {
	Type        string `yaml:"type"`
	Source      string `yaml:"source"`
	Destination string `yaml:"destination"`
	Protocol    string `yaml:"protocol"`
	DestPort    int    `yaml:"destPort"`
	DPort       int    `yaml:"dport"`
	Comment     string `yaml:"comment"`
}

func (p StrategyPolicy) EffectiveDestPort() int {
	if p.DestPort != 0 {
		return p.DestPort
	}
	return p.DPort
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

	containerDefined bool
	bottleDefined    bool
	resourcesDefined bool
}

func (s *StrategyStages) UnmarshalYAML(value *yaml.Node) error {
	type strategyStagesAlias StrategyStages
	var out strategyStagesAlias
	if err := value.Decode(&out); err != nil {
		return err
	}
	if value != nil && value.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(value.Content); i += 2 {
			switch strings.TrimSpace(value.Content[i].Value) {
			case "container":
				out.containerDefined = true
			case "bottle":
				out.bottleDefined = true
			case "resources":
				out.resourcesDefined = true
			}
		}
	}
	*s = StrategyStages(out)
	return nil
}

type StrategyStage struct {
	Apply  StrategyApply   `yaml:"apply"`
	Checks StrategyChecks  `yaml:"checks"`
	Health []StrategyCheck `yaml:"healthChecks"`
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

type StrategyRunResult struct {
	Name            string
	BottleOutput    string
	ComposeOutput   string
	ResourcesOutput string
	Steps           []StrategyStepResult
}

type StrategyStepResult struct {
	Name   string
	Status string
}
