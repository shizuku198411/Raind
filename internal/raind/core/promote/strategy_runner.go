package promote

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	bottlecore "raind/internal/raind/core/bottle"
	"raind/internal/raind/core/container"
	"raind/internal/raind/core/pod"
	policycore "raind/internal/raind/core/policy"
	resourcecore "raind/internal/raind/core/resource"
)

type StrategyRunner struct {
	spec       StrategySpec
	opt        StrategyOptions
	result     StrategyRunResult
	totalSteps int
}

func NewStrategyRunner(spec StrategySpec, opt StrategyOptions) *StrategyRunner {
	return &StrategyRunner{spec: spec, opt: opt, totalSteps: estimateStrategyStepCount(spec, opt)}
}

func (r *StrategyRunner) Run() (StrategyRunResult, error) {
	r.result = StrategyRunResult{Name: r.spec.Metadata.Name}
	if r.opt.ProgressStart != nil {
		r.opt.ProgressStart(r.result.Name)
	}
	if r.opt.DryRun {
		r.addStep("plan", "ok")
		return r.result, nil
	}

	if err := r.runContainerStage(); err != nil {
		return r.result, err
	}
	if r.reached("container") || r.reached("bottle-draft") {
		return r.result, nil
	}

	if err := r.runBottleStage(); err != nil {
		return r.result, err
	}
	if r.reached("bottle") || r.reached("resources-draft") {
		return r.result, nil
	}

	if err := r.runResourcesStage(); err != nil {
		return r.result, err
	}
	return r.result, nil
}

func (r *StrategyRunner) runContainerStage() error {
	ids, err := r.createContainers()
	if err != nil {
		if cleanupErr := r.deleteContainers(ids); cleanupErr != nil {
			r.addStep("container/delete", "warning: "+cleanupErr.Error())
		}
		return err
	}

	err = r.runStageChecks("container", r.spec.Stages.Container)
	if err == nil {
		err = r.promoteContainersToBottle()
	}

	cleanupErr := r.deleteContainers(ids)
	if err != nil {
		if cleanupErr != nil {
			r.addStep("container/delete", "warning: "+cleanupErr.Error())
		}
		return err
	}
	return cleanupErr
}

func (r *StrategyRunner) runBottleStage() error {
	bottleName, err := r.applyBottle()
	if err != nil {
		return err
	}

	err = r.runStageChecks("bottle", r.spec.Stages.Bottle)
	if err == nil {
		err = r.promoteBottleToResources()
	}

	cleanupErr := r.deleteBottle(bottleName)
	if err != nil {
		if cleanupErr != nil {
			r.addStep("bottle/delete", "warning: "+cleanupErr.Error())
		}
		return err
	}
	return cleanupErr
}

func (r *StrategyRunner) runResourcesStage() error {
	path, err := r.applyResources()
	if err != nil {
		return err
	}

	err = r.runStageChecks("resources", r.spec.Stages.Resources)
	cleanupErr := r.deleteResources(path)
	if err != nil {
		if cleanupErr != nil {
			r.addStep("resources/delete", "warning: "+cleanupErr.Error())
		}
		return err
	}
	return cleanupErr
}

func (r *StrategyRunner) createContainers() ([]string, error) {
	createdIDs := []string{}
	for _, c := range r.spec.Source.Containers {
		step := "container/create " + c.Name
		r.beginStep(step)
		create := container.NewServiceContainerCreate()
		id, err := create.Create(container.ServiceCreateModel{
			Image:           c.Image,
			Command:         []string(c.Command),
			Network:         c.Network,
			Volume:          mergeStringSlices([]string(c.Volume), []string(c.Mount)),
			Publish:         mergeStringSlices([]string(c.Publish), []string(c.Ports)),
			Device:          []string(c.Device),
			Env:             normalizeEnv([]string(c.Env)),
			CapAdd:          []string(c.CapAdd),
			CapDrop:         []string(c.CapDrop),
			SecurityProfile: c.SecurityProfile,
			Tty:             c.Tty,
			Name:            c.Name,
		})
		if err != nil {
			r.failStep(step, err)
			return createdIDs, fmt.Errorf("create container %s: %w", c.Name, err)
		}
		createdIDs = append(createdIDs, id)
		if err := container.NewServiceContainerStart().Start(container.ServiceStartModel{Id: id, Tty: c.Tty}); err != nil {
			r.failStep(step, err)
			return createdIDs, fmt.Errorf("start container %s: %w", c.Name, err)
		}
		r.addStep(step, "ok")
	}
	step := "container/runtime"
	r.beginStep(step)
	if err := r.waitContainersRunning(); err != nil {
		r.failStep(step, err)
		return createdIDs, err
	}
	r.addStep(step, "ok")
	return createdIDs, nil
}

func (r *StrategyRunner) waitContainersRunning() error {
	deadline := time.Now().Add(defaultStrategyTimeout)
	for {
		allRunning := true
		for _, c := range r.spec.Source.Containers {
			state, err := container.NewServiceContainerInspect().Get(c.Name)
			if err != nil {
				allRunning = false
				continue
			}
			if !strings.EqualFold(strings.TrimSpace(state.State), "running") {
				allRunning = false
			}
		}
		if allRunning {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("container runtime check timed out")
		}
		time.Sleep(defaultStrategyInterval)
	}
}

func (r *StrategyRunner) promoteContainersToBottle() error {
	step := "promote/container-to-bottle"
	r.beginStep(step)
	inspectService := container.NewServiceContainerInspect()
	inspects := make([]container.ContainerInspectModel, 0, len(r.spec.Source.Containers))
	for _, c := range r.spec.Source.Containers {
		inspect, err := inspectService.Get(c.Name)
		if err != nil {
			r.failStep(step, err)
			return fmt.Errorf("inspect container %s: %w", c.Name, err)
		}
		inspects = append(inspects, inspect)
	}

	draft, err := BuildBottleDraftFromContainers(inspects, ContainerToBottleOptions{
		BottleName:           r.spec.Metadata.Name,
		PreserveSensitiveEnv: true,
	})
	if err != nil {
		r.failStep(step, err)
		return err
	}
	if policyData, err := policycore.NewServicePolicyList().Get("RAIND-EW"); err == nil {
		AttachSecurityPoliciesFromPolicyList(&draft, inspects, policyData.Policies)
	} else {
		draft.Warnings = append(draft.Warnings, Warning{Code: "security-policy", Message: fmt.Sprintf("could not load running security policies from condenser API: %v", err)})
	}

	bottleData, err := RenderBottlefile(draft)
	if err != nil {
		r.failStep(step, err)
		return err
	}
	reviewData, err := RenderBottleReview(draft)
	if err != nil {
		r.failStep(step, err)
		return err
	}
	output := r.bottleOutput()
	if err := WriteBottlePromotionOutputs(output, bottleData, reviewData, true); err != nil {
		r.failStep(step, err)
		return err
	}
	r.result.BottleOutput = output
	r.addStep(step, output)
	return nil
}

func (r *StrategyRunner) deleteContainers(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	step := "container/delete"
	r.beginStep(step)
	var errs []string
	for i := len(ids) - 1; i >= 0; i-- {
		id := strings.TrimSpace(ids[i])
		if id == "" {
			continue
		}
		if err := container.NewServiceContainerStop().Stop(container.ServiceStopModel{Id: id}); err != nil {
			errs = append(errs, fmt.Sprintf("stop %s: %v", id, err))
		}
		if err := container.NewServiceContainerRemove().Remove(container.ServiceRemoveModel{Id: id}); err != nil {
			errs = append(errs, fmt.Sprintf("remove %s: %v", id, err))
		}
	}
	if len(errs) > 0 {
		err := errors.New(strings.Join(errs, "; "))
		r.failStep(step, err)
		return err
	}
	r.addStep(step, "ok")
	return nil
}

func (r *StrategyRunner) applyBottle() (string, error) {
	step := "bottle/apply"
	r.beginStep(step)
	path := r.spec.Stages.Bottle.Apply.File
	if strings.TrimSpace(path) == "" {
		path = r.bottleOutput()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		r.failStep(step, err)
		return "", fmt.Errorf("read bottle file: %w", err)
	}
	created, err := bottlecore.NewServiceBottleCreate().Create(bottlecore.ServiceBottleCreateModel{Yaml: data})
	if err != nil {
		r.failStep(step, err)
		return "", fmt.Errorf("bottle create failed: %w", err)
	}
	if err := bottlecore.NewServiceBottleStart().Start(bottlecore.ServiceBottleStartModel{Target: created.BottleName}); err != nil {
		r.failStep(step, err)
		return created.BottleName, fmt.Errorf("bottle start failed: %w", err)
	}
	r.addStep(step, created.BottleName)
	return created.BottleName, nil
}

func (r *StrategyRunner) promoteBottleToResources() error {
	step := "promote/bottle-to-resources"
	r.beginStep(step)
	draft, err := BuildResourceDraftFromRunningBottleFile(r.bottleOutput(), BottleToResourcesOptions{
		Namespace:   firstNonEmpty(r.opt.Namespace, r.spec.Metadata.Name),
		IngressHost: r.ingressHost(),
	})
	if err != nil {
		r.failStep(step, err)
		return err
	}
	files, err := RenderResourceFiles(draft, RenderResourcesOptions{
		IngressHost:       r.ingressHost(),
		ServiceType:       "NodePort",
		PreserveHostPorts: true,
	})
	if err != nil {
		r.failStep(step, err)
		return err
	}
	output := r.resourcesOutput()
	if err := WriteResourceFiles(output, files, true); err != nil {
		r.failStep(step, err)
		return err
	}
	r.result.ResourcesOutput = output
	r.addStep(step, output)
	return nil
}

func (r *StrategyRunner) deleteBottle(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	step := "bottle/delete"
	r.beginStep(step)
	if err := bottlecore.NewServiceBottleDelete().Delete(bottlecore.ServiceBottleDeleteModel{Target: name}); err != nil {
		r.failStep(step, err)
		return err
	}
	r.addStep(step, "ok")
	return nil
}

func (r *StrategyRunner) applyResources() (string, error) {
	step := "resources/apply"
	r.beginStep(step)
	path := strings.TrimSpace(r.spec.Stages.Resources.Apply.File)
	if path == "" {
		path = filepath.Join(r.resourcesOutput(), "all.yaml")
	}
	if strings.TrimSpace(r.spec.Stages.Resources.Apply.Path) != "" {
		path = filepath.Join(r.spec.Stages.Resources.Apply.Path, "all.yaml")
	}
	if _, err := os.Stat(path); err != nil {
		r.failStep(step, err)
		return "", fmt.Errorf("resource apply file %q is not readable: %w", path, err)
	}
	if _, err := pod.NewServicePodApply().Apply(pod.ServicePodApplyModel{FilePath: path}); err != nil {
		r.failStep(step, err)
		return path, fmt.Errorf("resource apply failed: %w", err)
	}
	r.addStep(step, path)
	return path, nil
}

func (r *StrategyRunner) deleteResources(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	step := "resources/delete"
	r.beginStep(step)
	if _, err := resourcecore.NewServiceResourceDelete().Delete(resourcecore.ServiceResourceDeleteModel{FilePath: path}); err != nil {
		r.failStep(step, err)
		return err
	}
	r.addStep(step, "ok")
	return nil
}

func (r *StrategyRunner) runStageChecks(name string, stage StrategyStage) error {
	checks := append([]StrategyCheck{}, stage.Checks.Runtime...)
	checks = append(checks, stage.Checks.Application...)
	checks = append(checks, stage.Health...)
	for _, check := range checks {
		step := name + "/check " + checkName(check)
		r.beginStep(step)
		if err := RunStrategyCheck(check); err != nil {
			r.failStep(step, err)
			return fmt.Errorf("%s check %s failed: %w", name, checkName(check), err)
		}
		r.addStep(step, "ok")
	}
	return nil
}

func (r *StrategyRunner) bottleOutput() string {
	return strategyDefaultedOutput(
		DefaultBottlePromotionOutput,
		[]string{"bottle.yaml"},
		r.spec.Stages.Container.Promote.Output,
		r.spec.Outputs.Bottle,
	)
}

func (r *StrategyRunner) resourcesOutput() string {
	return strategyDefaultedOutput(
		DefaultResourcePromotionOutput,
		[]string{"manifests", "manifests/"},
		r.spec.Stages.Bottle.Promote.Output,
		r.spec.Outputs.Resources,
	)
}

func strategyDefaultedOutput(defaultValue string, legacyDefaults []string, values ...string) string {
	value := firstNonEmpty(values...)
	if value == "" || isLegacyStrategyDefaultOutput(value, legacyDefaults) {
		return defaultValue
	}
	return value
}

func isLegacyStrategyDefaultOutput(value string, legacyDefaults []string) bool {
	normalized := filepath.Clean(strings.TrimSpace(value))
	for _, legacy := range legacyDefaults {
		if normalized == filepath.Clean(legacy) {
			return true
		}
	}
	return false
}

func (r *StrategyRunner) ingressHost() string {
	return firstNonEmpty(r.opt.IngressHost)
}

func (r *StrategyRunner) reached(stage string) bool {
	until := strings.ToLower(strings.TrimSpace(r.opt.Until))
	return until != "" && until == strings.ToLower(stage)
}

func (r *StrategyRunner) beginStep(name string) {
	if r.opt.Progress == nil {
		return
	}
	r.opt.Progress(StrategyProgressEvent{
		Name:   name,
		Status: "running",
		Index:  len(r.result.Steps) + 1,
		Total:  r.totalSteps,
		Done:   false,
	})
}

func (r *StrategyRunner) failStep(name string, err error) {
	status := "failed"
	if err != nil {
		status = "failed: " + err.Error()
	}
	r.addStep(name, status)
}

func (r *StrategyRunner) addStep(name, status string) {
	step := StrategyStepResult{Name: name, Status: status}
	r.result.Steps = append(r.result.Steps, step)
	if r.opt.Progress != nil {
		r.opt.Progress(StrategyProgressEvent{
			Name:   name,
			Status: status,
			Index:  len(r.result.Steps),
			Total:  r.totalSteps,
			Done:   true,
		})
	}
}

func estimateStrategyStepCount(spec StrategySpec, opt StrategyOptions) int {
	containerSteps := len(spec.Source.Containers) + 1 + strategyCheckCount(spec.Stages.Container) + 1 + 1
	bottleSteps := 1 + strategyCheckCount(spec.Stages.Bottle) + 1 + 1
	resourceSteps := 1 + strategyCheckCount(spec.Stages.Resources) + 1
	switch strings.ToLower(strings.TrimSpace(opt.Until)) {
	case "container", "bottle-draft":
		return containerSteps
	case "bottle", "resources-draft":
		return containerSteps + bottleSteps
	default:
		return containerSteps + bottleSteps + resourceSteps
	}
}

func strategyCheckCount(stage StrategyStage) int {
	return len(stage.Checks.Runtime) + len(stage.Checks.Application) + len(stage.Health)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func mergeStringSlices(a, b []string) []string {
	out := append([]string{}, a...)
	out = append(out, b...)
	return out
}

func normalizeEnv(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
