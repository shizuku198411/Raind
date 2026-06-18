package container

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	httpclient "raind/internal/raind/core/client"
	"strings"
	"text/tabwriter"
	"time"
)

func NewServiceContainerInspect() *ServiceContainerInspect {
	return &ServiceContainerInspect{}
}

type ServiceContainerInspect struct{}

func (s *ServiceContainerInspect) Inspect(target string, asJSON bool) error {
	httpClient, err := httpclient.NewHttpClient()
	if err != nil {
		return err
	}
	httpClient.NewRequest(
		http.MethodGet,
		"/v1/containers/"+url.PathEscape(target)+"/inspect",
		nil,
	)
	resp, err := httpClient.Client.Do(httpClient.Request)
	if err != nil {
		return fmt.Errorf("Cannot connect to the Raind daemon. Is the raind daemon running?")
	}
	defer resp.Body.Close()

	var respModel InspectResponseModel
	if !httpClient.IsStatusOk(resp) {
		if decodeErr := json.NewDecoder(resp.Body).Decode(&respModel); decodeErr != nil {
			return fmt.Errorf("decode response: %w", decodeErr)
		}
		return fmt.Errorf("unexpected status: %s: %s", resp.Status, respModel.Message)
	}

	if err := json.NewDecoder(resp.Body).Decode(&respModel); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if asJSON {
		out, err := json.MarshalIndent(respModel.Data.Container, "", "  ")
		if err != nil {
			return fmt.Errorf("encode inspect json: %w", err)
		}
		fmt.Println(string(out))
		return nil
	}

	return s.print(respModel.Data.Container)
}

func (s *ServiceContainerInspect) print(inspect ContainerInspectModel) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	writeKV := func(key string, value any) {
		fmt.Fprintf(w, "%s:\t%v\n", key, value)
	}

	writeKV("ID", inspect.ContainerId)
	writeKV("Name", inspect.Name)
	writeKV("State", inspect.State)
	writeKV("PID", inspect.Pid)
	writeKV("Image", formatContainerImage(inspect.ImageRepository, inspect.ImageReference))
	writeKV("Command", shellJoin(inspect.Command))
	writeKV("Security Profile", inspect.SecurityProfile)
	writeKV("TTY", inspect.Tty)
	if inspect.PodId != "" {
		writeKV("Pod ID", inspect.PodId)
	}
	if inspect.DropletId != "" {
		writeKV("Droplet ID", inspect.DropletId)
	}
	writeKV("Log Path", inspect.LogPath)
	writeKV("Created", formatInspectTime(inspect.CreatedAt))
	if !inspect.StartedAt.IsZero() {
		writeKV("Started", formatInspectTime(inspect.StartedAt))
	}
	if !inspect.StoppedAt.IsZero() {
		writeKV("Stopped", formatInspectTime(inspect.StoppedAt))
	}
	fmt.Fprintln(w)

	printConfigSummary(w, inspect.Config)
	return w.Flush()
}

func printConfigSummary(w *tabwriter.Writer, config map[string]any) {
	writeKV := func(key string, value any) {
		fmt.Fprintf(w, "%s:\t%v\n", key, value)
	}

	if root, ok := config["root"].(map[string]any); ok {
		if path, ok := root["path"].(string); ok {
			writeKV("Rootfs", path)
		}
	}
	if hostname, ok := config["hostname"].(string); ok {
		writeKV("Hostname", hostname)
	}
	if process, ok := config["process"].(map[string]any); ok {
		if cwd, ok := process["cwd"].(string); ok {
			writeKV("Working Dir", cwd)
		}
		if args, ok := stringSliceFromAny(process["args"]); ok {
			writeKV("Args", shellJoin(args))
		}
		if env, ok := stringSliceFromAny(process["env"]); ok && len(env) > 0 {
			fmt.Fprintln(w, "Env:")
			for _, e := range env {
				fmt.Fprintf(w, "  %s\n", e)
			}
		}
	}
	if linuxSpec, ok := config["linux"].(map[string]any); ok {
		if namespaces, ok := linuxSpec["namespaces"].([]any); ok && len(namespaces) > 0 {
			fmt.Fprintln(w, "Namespaces:")
			for _, item := range namespaces {
				ns, ok := item.(map[string]any)
				if !ok {
					continue
				}
				nsType, _ := ns["type"].(string)
				nsPath, _ := ns["path"].(string)
				if nsPath == "" {
					fmt.Fprintf(w, "  %s\n", nsType)
				} else {
					fmt.Fprintf(w, "  %s\t%s\n", nsType, nsPath)
				}
			}
		}
	}
	if mounts, ok := config["mounts"].([]any); ok && len(mounts) > 0 {
		fmt.Fprintln(w, "Mounts:")
		for _, item := range mounts {
			mount, ok := item.(map[string]any)
			if !ok {
				continue
			}
			source, _ := mount["source"].(string)
			destination, _ := mount["destination"].(string)
			mountType, _ := mount["type"].(string)
			options, _ := stringSliceFromAny(mount["options"])
			fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", destination, source, mountType, strings.Join(options, ","))
		}
	}
}

func stringSliceFromAny(value any) ([]string, bool) {
	switch v := value.(type) {
	case []string:
		return v, true
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			out = append(out, s)
		}
		return out, true
	default:
		return nil, false
	}
}

func shellJoin(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return strings.Join(args, " ")
}

func formatInspectTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}
