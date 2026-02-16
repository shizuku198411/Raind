package droplet

import (
	"condenser/internal/runtime"
	"condenser/internal/utils"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

func NewDropletHandler() *DropletHandler {
	return &DropletHandler{
		commandFactory: utils.NewCommandFactory(),
	}
}

type DropletHandler struct {
	commandFactory utils.CommandFactory
}

const runtimePath = "droplet"

func (h *DropletHandler) Spec(specParameter runtime.SpecModel) error {
	args := []string{
		"spec",
		"--rootfs", specParameter.Rootfs,
		"--cwd", specParameter.Cwd,
		"--command", specParameter.Command,
		"--hostname", specParameter.Hostname,
		"--host_if_name", specParameter.HostInterface,
		"--bridge_if_name", specParameter.BridgeInterface,
		"--if_name", specParameter.ContainerInterface,
		"--if_addr", specParameter.ContainerInterfaceAddr,
		"--if_gateway", specParameter.ContainerGateway,
		"--upper_dir", specParameter.UpperDir,
		"--work_dir", specParameter.WorkDir,
		"--output", specParameter.Output,
	}
	for _, v := range specParameter.Namespace {
		args = slices.Concat(args, []string{"--ns", v})
	}
	for _, v := range specParameter.NSPath {
		args = slices.Concat(args, []string{"--ns-path", v})
	}
	for _, v := range specParameter.Env {
		args = slices.Concat(args, []string{"--env", v})
	}
	for _, v := range specParameter.Mount {
		args = slices.Concat(args, []string{"--mount", v})
	}
	for _, v := range specParameter.ContainerDns {
		args = slices.Concat(args, []string{"--dns", v})
	}
	for _, v := range specParameter.ImageLayer {
		args = slices.Concat(args, []string{"--image_layer", v})
	}
	for _, v := range specParameter.CreateRuntimeHook {
		args = slices.Concat(args, []string{"--hook-create-runtime", v})
	}
	for _, v := range specParameter.CreateRuntimeHookEnv {
		args = slices.Concat(args, []string{"--hook-create-runtime-env", v})
	}
	for _, v := range specParameter.CreateContainerHook {
		args = slices.Concat(args, []string{"--hook-create-container", v})
	}
	for _, v := range specParameter.CreateContainerHookEnv {
		args = slices.Concat(args, []string{"--hook-create-container-env", v})
	}
	for _, v := range specParameter.StartContainerHook {
		args = slices.Concat(args, []string{"--hook-start-container", v})
	}
	for _, v := range specParameter.StartContainerHookEnv {
		args = slices.Concat(args, []string{"--hook-start-container-env", v})
	}
	for _, v := range specParameter.PoststartHook {
		args = slices.Concat(args, []string{"--hook-poststart", v})
	}
	for _, v := range specParameter.PoststartHookEnv {
		args = slices.Concat(args, []string{"--hook-poststart-env", v})
	}
	for _, v := range specParameter.StopContainerHook {
		args = slices.Concat(args, []string{"--hook-stop-container", v})
	}
	for _, v := range specParameter.StopContainerHookEnv {
		args = slices.Concat(args, []string{"--hook-stop-container-env", v})
	}
	for _, v := range specParameter.PoststopHook {
		args = slices.Concat(args, []string{"--hook-poststop", v})
	}
	for _, v := range specParameter.PoststopHookEnv {
		args = slices.Concat(args, []string{"--hook-poststop-env", v})
	}

	runtimeSpec := h.commandFactory.Command(runtimePath, args...)
	out, err := runtimeSpec.CombineOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return fmt.Errorf("droplet spec failed: %w", err)
		}
		return fmt.Errorf("droplet spec failed: %s: %w", msg, err)
	}
	return nil
}

func (h *DropletHandler) Create(createParameter runtime.CreateModel, podPid int) error {
	if podPid <= 0 {
		var args []string
		if createParameter.Tty {
			args = []string{
				"create",
				"-t",
				createParameter.ContainerId,
			}
		} else {
			args = []string{
				"create",
				createParameter.ContainerId,
			}
		}
		runtimeCreate := h.commandFactory.Command(runtimePath, args...)
		out, err := runtimeCreate.CombineOutput()
		if err != nil {
			msg := strings.TrimSpace(string(out))
			if msg == "" {
				return fmt.Errorf("droplet create failed: %w", err)
			}
			return fmt.Errorf("droplet create failed: %s: %w", msg, err)
		}
		return nil
	} else {
		var args []string
		if createParameter.Tty {
			args = []string{
				"-t", strconv.Itoa(podPid), "-U", "--",
				runtimePath,
				"create",
				"-t",
				createParameter.ContainerId,
			}
		} else {
			args = []string{
				"-t", strconv.Itoa(podPid), "-U", "--",
				runtimePath,
				"create",
				createParameter.ContainerId,
			}
		}
		runtimeCreate := h.commandFactory.Command("nsenter", args...)
		out, err := runtimeCreate.CombineOutput()
		if err != nil {
			msg := strings.TrimSpace(string(out))
			if msg == "" {
				return fmt.Errorf("droplet create failed: %w", err)
			}
			return fmt.Errorf("droplet create failed: %s: %w", msg, err)
		}
		return nil
	}
}

func (h *DropletHandler) Start(startParameter runtime.StartModel) error {
	args := []string{
		"start",
		startParameter.ContainerId,
	}
	runtimeStart := h.commandFactory.Command(runtimePath, args...)
	out, err := runtimeStart.CombineOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return fmt.Errorf("droplet start failed: %w", err)
		}
		return fmt.Errorf("droplet start failed: %s: %w", msg, err)
	}
	return nil
}

func (h *DropletHandler) Delete(deleteParameter runtime.DeleteModel) error {
	args := []string{
		"delete",
		deleteParameter.ContainerId,
	}
	runtimeDelete := h.commandFactory.Command(runtimePath, args...)
	out, err := runtimeDelete.CombineOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return fmt.Errorf("droplet delete failed: %w", err)
		}
		return fmt.Errorf("droplet delete failed: %s: %w", msg, err)
	}
	return nil
}

func (h *DropletHandler) Stop(stopParameter runtime.StopModel) error {
	args := []string{
		"kill",
		stopParameter.ContainerId,
	}
	runtimeStop := h.commandFactory.Command(runtimePath, args...)
	out, err := runtimeStop.CombineOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return fmt.Errorf("droplet stop failed: %w", err)
		}
		return fmt.Errorf("droplet stop failed: %s: %w", msg, err)
	}
	return nil
}

func (h *DropletHandler) Exec(execParameter runtime.ExecModel) error {
	var args []string
	if execParameter.Tty {
		args = []string{
			"exec",
			"-t",
			execParameter.ContainerId,
		}
	} else {
		args = []string{
			"exec",
			execParameter.ContainerId,
		}
	}
	args = append(args, execParameter.Entrypoint...)
	runtimeExec := h.commandFactory.Command(runtimePath, args...)
	out, err := runtimeExec.CombineOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return fmt.Errorf("droplet exec failed: %w", err)
		}
		return fmt.Errorf("droplet exec failed: %s: %w", msg, err)
	}
	return nil
}
