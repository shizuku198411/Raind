package promote

import "testing"

func TestParseStrategyInfersPromoteTargetsFromStages(t *testing.T) {
	spec, err := ParseStrategy([]byte(`
apiVersion: raind.io/v1alpha1
kind: PromoteStrategy
metadata:
  name: web-stack
source:
  containers:
    - name: web
      image: nginx:latest
stages:
  container:
    checks:
      runtime:
        - name: web-running
          type: containerStatus
          target: web
          expect:
            state: running
  bottle:
    checks:
      runtime:
        - name: bottle-running
          type: bottleStatus
          target: web-stack
  resources:
    checks:
      application:
        - name: web-http
          type: http
          target: http://127.0.0.1:8080
          expect:
            status: 200
`))
	if err != nil {
		t.Fatalf("ParseStrategy: %v", err)
	}
	if !strategyBottleStageDefined(spec) {
		t.Fatalf("expected bottle stage to be defined")
	}
	if !strategyResourcesStageDefined(spec) {
		t.Fatalf("expected resources stage to be defined")
	}
}

func TestParseStrategyTreatsEmptyStageAsDefined(t *testing.T) {
	spec, err := ParseStrategy([]byte(`
apiVersion: raind.io/v1alpha1
kind: PromoteStrategy
metadata:
  name: web-stack
source:
  containers:
    - name: web
      image: nginx:latest
stages:
  bottle: {}
  resources: {}
`))
	if err != nil {
		t.Fatalf("ParseStrategy: %v", err)
	}
	if !strategyBottleStageDefined(spec) {
		t.Fatalf("expected empty bottle stage to be defined")
	}
	if !strategyResourcesStageDefined(spec) {
		t.Fatalf("expected empty resources stage to be defined")
	}
}

func TestParseStrategyRejectsResourcesWithoutBottle(t *testing.T) {
	_, err := ParseStrategy([]byte(`
apiVersion: raind.io/v1alpha1
kind: PromoteStrategy
metadata:
  name: web-stack
source:
  containers:
    - name: web
      image: nginx:latest
stages:
  resources:
    checks:
      application:
        - name: web-http
          type: http
          target: http://127.0.0.1:8080
          expect:
            status: 200
`))
	if err == nil {
		t.Fatalf("expected error when resources stage is defined without bottle stage")
	}
}

func TestEstimateStrategyStepCountSkipsUndefinedDownstreamStages(t *testing.T) {
	spec := StrategySpec{
		Source: StrategySource{Containers: []StrategyContainer{{Name: "web", Image: "nginx:latest"}}},
		Stages: StrategyStages{
			Container: StrategyStage{Checks: StrategyChecks{Runtime: []StrategyCheck{{Name: "web-running"}}}},
		},
	}

	got := estimateStrategyStepCount(spec, StrategyOptions{})
	want := 4 // create, runtime, container check, delete
	if got != want {
		t.Fatalf("estimateStrategyStepCount without downstream stages = %d, want %d", got, want)
	}

	spec.Stages.Bottle = StrategyStage{Checks: StrategyChecks{Runtime: []StrategyCheck{{Name: "bottle-running"}}}}
	got = estimateStrategyStepCount(spec, StrategyOptions{})
	want = 8 // container stage + container promote + bottle apply/check/delete
	if got != want {
		t.Fatalf("estimateStrategyStepCount with bottle stage = %d, want %d", got, want)
	}

	spec.Stages.Resources = StrategyStage{Checks: StrategyChecks{Application: []StrategyCheck{{Name: "web-http"}}}}
	got = estimateStrategyStepCount(spec, StrategyOptions{})
	want = 12 // previous + bottle promote + resources apply/check/delete
	if got != want {
		t.Fatalf("estimateStrategyStepCount with resources stage = %d, want %d", got, want)
	}
}
