package promote

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const DefaultStrategyFile = "raind-strategy.yaml"

func RunStrategyFile(path string, opt StrategyOptions) (StrategyRunResult, error) {
	if strings.TrimSpace(path) == "" {
		path = DefaultStrategyFile
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return StrategyRunResult{}, fmt.Errorf("read strategy file: %w", err)
	}
	spec, err := ParseStrategy(body)
	if err != nil {
		return StrategyRunResult{}, err
	}
	return NewStrategyRunner(spec, opt).Run()
}

func ParseStrategy(data []byte) (StrategySpec, error) {
	var spec StrategySpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return StrategySpec{}, fmt.Errorf("parse strategy yaml: %w", err)
	}
	if strings.TrimSpace(spec.Metadata.Name) == "" {
		return StrategySpec{}, fmt.Errorf("metadata.name is required")
	}
	kind := strings.TrimSpace(spec.Kind)
	if kind != "" && !strings.EqualFold(kind, "PromoteStrategy") {
		return StrategySpec{}, fmt.Errorf("unsupported strategy kind %q", spec.Kind)
	}
	if len(spec.Source.Containers) == 0 && len(spec.Containers) > 0 {
		spec.Source.Containers = spec.Containers
	}
	mode := strings.TrimSpace(spec.Source.Mode)
	if mode == "" {
		spec.Source.Mode = "create"
	}
	if !strings.EqualFold(spec.Source.Mode, "create") {
		return StrategySpec{}, fmt.Errorf("unsupported strategy source.mode %q; only create is supported in this release", spec.Source.Mode)
	}
	if len(spec.Source.Containers) == 0 {
		return StrategySpec{}, fmt.Errorf("source.containers is required")
	}
	if to := strings.TrimSpace(spec.Stages.Container.Promote.To); to != "" && !strings.EqualFold(to, "bottle") {
		return StrategySpec{}, fmt.Errorf("unsupported container promote target %q; only bottle is supported", to)
	}
	if to := strings.TrimSpace(spec.Stages.Bottle.Promote.To); to != "" && !strings.EqualFold(to, "resources") {
		return StrategySpec{}, fmt.Errorf("unsupported bottle promote target %q; only resources is supported", to)
	}
	for i, c := range spec.Source.Containers {
		if strings.TrimSpace(c.Name) == "" {
			return StrategySpec{}, fmt.Errorf("source.containers[%d].name is required", i)
		}
		if strings.TrimSpace(c.Image) == "" {
			return StrategySpec{}, fmt.Errorf("source.containers[%d].image is required", i)
		}
	}
	return spec, nil
}
