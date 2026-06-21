package promote

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	bottlecore "raind/internal/raind/core/bottle"
	"raind/internal/raind/core/container"
	"raind/internal/raind/core/pod"
	policycore "raind/internal/raind/core/policy"
	resourcecore "raind/internal/raind/core/resource"
)

const strategyRaindCommandSettleDelay = 500 * time.Millisecond

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
		step := "container/create::" + c.Name
		r.beginStep(step)
		var id string
		create := container.NewServiceContainerCreate()
		err := r.captureInternalOutput(step, func() error {
			r.waitBeforeRaindCommand()
			createdID, err := create.Create(container.ServiceCreateModel{
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
				return fmt.Errorf("create container %s: %w", c.Name, err)
			}
			id = createdID
			r.waitBeforeRaindCommand()
			return container.NewServiceContainerStart().Start(container.ServiceStartModel{Id: id, Tty: c.Tty})
		})
		if err != nil {
			r.failStep(step, err)
			return createdIDs, err
		}
		createdIDs = append(createdIDs, id)
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
	step := "container/promote"
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
	_ = r.captureInternalOutput(step, func() error {
		for i := len(ids) - 1; i >= 0; i-- {
			id := strings.TrimSpace(ids[i])
			if id == "" {
				continue
			}
			r.waitBeforeRaindCommand()
			if err := container.NewServiceContainerStop().Stop(container.ServiceStopModel{Id: id}); err != nil {
				errs = append(errs, fmt.Sprintf("stop %s: %v", id, err))
			}
			r.waitBeforeRaindCommand()
			if err := container.NewServiceContainerRemove().Remove(container.ServiceRemoveModel{Id: id}); err != nil {
				errs = append(errs, fmt.Sprintf("remove %s: %v", id, err))
			}
		}
		return nil
	})
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
	var created bottlecore.CreateResponseDataModel
	err = r.captureInternalOutput(step, func() error {
		r.waitBeforeRaindCommand()
		createdBottle, err := bottlecore.NewServiceBottleCreate().Create(bottlecore.ServiceBottleCreateModel{Yaml: data})
		if err != nil {
			return fmt.Errorf("bottle create failed: %w", err)
		}
		created = createdBottle
		r.waitBeforeRaindCommand()
		if err := bottlecore.NewServiceBottleStart().Start(bottlecore.ServiceBottleStartModel{Target: created.BottleName}); err != nil {
			return fmt.Errorf("bottle start failed: %w", err)
		}
		return nil
	})
	if err != nil {
		r.failStep(step, err)
		return created.BottleName, err
	}
	r.addStep(step, created.BottleName)
	return created.BottleName, nil
}

func (r *StrategyRunner) promoteBottleToResources() error {
	step := "bottle/promote"
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
	if err := r.captureInternalOutput(step, func() error {
		r.waitBeforeRaindCommand()
		return bottlecore.NewServiceBottleDelete().Delete(bottlecore.ServiceBottleDeleteModel{Target: name})
	}); err != nil {
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
	if err := r.captureInternalOutput(step, func() error {
		r.waitBeforeRaindCommand()
		_, err := pod.NewServicePodApply().Apply(pod.ServicePodApplyModel{FilePath: path})
		if err != nil {
			return fmt.Errorf("resource apply failed: %w", err)
		}
		return nil
	}); err != nil {
		r.failStep(step, err)
		return path, err
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
	if err := r.captureInternalOutput(step, func() error {
		r.waitBeforeRaindCommand()
		_, err := resourcecore.NewServiceResourceDelete().Delete(resourcecore.ServiceResourceDeleteModel{FilePath: path})
		return err
	}); err != nil {
		r.failStep(step, err)
		return err
	}
	r.addStep(step, "ok")
	return nil
}

func (r *StrategyRunner) runStageChecks(name string, stage StrategyStage) error {
	if err := r.runCheckGroup(name, "runtime", stage.Checks.Runtime); err != nil {
		return err
	}
	if err := r.runCheckGroup(name, "application", stage.Checks.Application); err != nil {
		return err
	}
	if err := r.runCheckGroup(name, "health", stage.Health); err != nil {
		return err
	}
	return nil
}

func (r *StrategyRunner) runCheckGroup(stageName, group string, checks []StrategyCheck) error {
	for _, check := range checks {
		step := stageName + "/checks::" + group + "::" + checkName(check)
		r.beginStep(step)
		if err := RunStrategyCheck(check); err != nil {
			r.failStep(step, err)
			return fmt.Errorf("%s %s check %s failed: %w", stageName, group, checkName(check), err)
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
		Name:  name,
		Index: len(r.result.Steps) + 1,
		Total: r.totalSteps,
		Done:  false,
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

func (r *StrategyRunner) waitBeforeRaindCommand() {
	time.Sleep(strategyRaindCommandSettleDelay)
}

func (r *StrategyRunner) captureInternalOutput(step string, fn func() error) error {
	if r.opt.InternalLog == nil {
		return fn()
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		return fn()
	}

	originalStdout := os.Stdout
	originalStderr := os.Stderr
	os.Stdout = writer
	os.Stderr = writer

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 1024), 1024*1024)
		for scanner.Scan() {
			line := strings.TrimRight(scanner.Text(), "\r\n")
			if strings.TrimSpace(line) == "" {
				continue
			}
			r.opt.InternalLog(StrategyInternalLogEvent{Step: step, Line: line})
		}
	}()

	runErr := fn()

	_ = writer.Close()
	os.Stdout = originalStdout
	os.Stderr = originalStderr
	wg.Wait()
	_ = reader.Close()
	r.opt.InternalLog(StrategyInternalLogEvent{Step: step, Done: true})
	return runErr
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
