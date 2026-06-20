package promote

import (
	"raind/internal/raind/core/container"
	"strings"
	"testing"
)

func TestBuildBottleDraftFromContainerRedactsSecretEnv(t *testing.T) {
	inspect := container.ContainerInspectModel{
		ContainerId:     "ctr1",
		Name:            "myapp",
		ImageRepository: "example.com/acme/myapp",
		ImageReference:  "v1",
		Config: map[string]any{
			"process": map[string]any{
				"env": []any{"APP_ENV=dev", "DB_PASSWORD=super-secret", "PATH=/usr/bin"},
			},
		},
		Forwards: []container.ForwardInfoModel{{HostPort: 8080, ContainerPort: 3000, Protocol: "tcp"}},
	}

	draft, err := BuildBottleDraftFromContainer(inspect, ContainerToBottleOptions{})
	if err != nil {
		t.Fatal(err)
	}
	out, err := RenderBottlefile(draft)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	if strings.Contains(text, "super-secret") {
		t.Fatalf("rendered Dripfile leaked secret value:\n%s", text)
	}
	if !strings.Contains(text, "APP_ENV=dev") {
		t.Fatalf("rendered Dripfile did not include non-secret env:\n%s", text)
	}
	if !strings.Contains(text, "env example: DB_PASSWORD=<redacted>") {
		t.Fatalf("rendered Dripfile did not include redacted secret hint:\n%s", text)
	}
	if !strings.Contains(text, `"8080:3000"`) {
		t.Fatalf("rendered Dripfile did not include port mapping:\n%s", text)
	}
}

func TestBuildBottleDraftFromContainerRejectsPodMember(t *testing.T) {
	_, err := BuildBottleDraftFromContainer(container.ContainerInspectModel{Name: "app", PodId: "pod1"}, ContainerToBottleOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRenderPortKeepsUDPProtocol(t *testing.T) {
	got := renderPort(PortMapping{HostPort: 5353, ContainerPort: 53, Protocol: "udp"})
	if got != "5353:53:udp" {
		t.Fatalf("unexpected port: %s", got)
	}
}

func TestBuildBottleDraftFromContainerFiltersInternalMounts(t *testing.T) {
	inspect := container.ContainerInspectModel{
		ContainerId:     "ctr1",
		Name:            "myapp",
		ImageRepository: "nginx",
		ImageReference:  "latest",
		Config: map[string]any{
			"mounts": []any{
				map[string]any{"type": "bind", "source": "/var/lib/raind/internal", "destination": "/internal"},
				map[string]any{"type": "bind", "source": "/host/www", "destination": "/usr/share/nginx/html", "options": []any{"rbind", "ro"}},
				map[string]any{"type": "bind", "source": "/tmp/resolv.conf", "destination": "/etc/resolv.conf"},
			},
		},
	}

	draft, err := BuildBottleDraftFromContainer(inspect, ContainerToBottleOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(draft.Mounts) != 1 {
		t.Fatalf("expected only user mount to remain, got %#v", draft.Mounts)
	}
	mount := draft.Mounts[0]
	if mount.Source != "/host/www" || mount.Destination != "/usr/share/nginx/html" || !mount.ReadOnly {
		t.Fatalf("unexpected mount: %#v", mount)
	}
	out, err := RenderBottlefile(draft)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	if strings.Contains(text, "/var/lib/raind/internal") || strings.Contains(text, "/etc/resolv.conf") {
		t.Fatalf("rendered Dripfile included internal mount:\n%s", text)
	}
	if !strings.Contains(text, `"/host/www:/usr/share/nginx/html:ro"`) {
		t.Fatalf("rendered Dripfile did not include read-only user mount:\n%s", text)
	}
}

func TestBuildBottleDraftFromContainerProducesDeterministicOutput(t *testing.T) {
	inspect := container.ContainerInspectModel{
		ContainerId:     "ctr1",
		Name:            "My_App",
		ImageRepository: "example.com/app",
		ImageReference:  "v1",
		Command:         []string{"/app/server", "--port", "3000"},
		Config: map[string]any{
			"process": map[string]any{
				"env": []any{"Z_VALUE=last", "APP_ENV=dev", "PASSWORD=secret", "PATH=/usr/bin"},
			},
			"mounts": []any{
				map[string]any{"type": "bind", "source": "/z", "destination": "/z"},
				map[string]any{"type": "bind", "source": "/a", "destination": "/a"},
			},
		},
		Forwards: []container.ForwardInfoModel{
			{HostPort: 9000, ContainerPort: 9000, Protocol: "udp"},
			{HostPort: 8080, ContainerPort: 3000, Protocol: "tcp"},
		},
	}

	draft1, err := BuildBottleDraftFromContainer(inspect, ContainerToBottleOptions{})
	if err != nil {
		t.Fatal(err)
	}
	draft2, err := BuildBottleDraftFromContainer(inspect, ContainerToBottleOptions{})
	if err != nil {
		t.Fatal(err)
	}
	out1, err := RenderBottlefile(draft1)
	if err != nil {
		t.Fatal(err)
	}
	out2, err := RenderBottlefile(draft2)
	if err != nil {
		t.Fatal(err)
	}
	if string(out1) != string(out2) {
		t.Fatalf("output is not deterministic:\n--- first ---\n%s\n--- second ---\n%s", string(out1), string(out2))
	}
	text := string(out1)
	if strings.Index(text, "APP_ENV=dev") > strings.Index(text, "Z_VALUE=last") {
		t.Fatalf("env output is not sorted:\n%s", text)
	}
	if strings.Index(text, `"/a:/a"`) > strings.Index(text, `"/z:/z"`) {
		t.Fatalf("mount output is not sorted:\n%s", text)
	}
	if strings.Index(text, `"8080:3000"`) > strings.Index(text, `"9000:9000:udp"`) {
		t.Fatalf("port output is not sorted:\n%s", text)
	}
}

func TestBuildBottleDraftFromContainerIncludeImageEnv(t *testing.T) {
	inspect := container.ContainerInspectModel{
		ContainerId:     "ctr1",
		Name:            "app",
		ImageRepository: "nginx",
		ImageReference:  "latest",
		Config: map[string]any{
			"process": map[string]any{
				"env": []any{"PATH=/usr/bin", "HOME=/root", "APP_ENV=dev"},
			},
		},
	}

	defaultDraft, err := BuildBottleDraftFromContainer(inspect, ContainerToBottleOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defaultOut, err := RenderBottlefile(defaultDraft)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(defaultOut), "PATH=/usr/bin") || strings.Contains(string(defaultOut), "HOME=/root") {
		t.Fatalf("default output included image env:\n%s", string(defaultOut))
	}

	includeDraft, err := BuildBottleDraftFromContainer(inspect, ContainerToBottleOptions{IncludeImageEnv: true})
	if err != nil {
		t.Fatal(err)
	}
	includeOut, err := RenderBottlefile(includeDraft)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(includeOut), "PATH=/usr/bin") || !strings.Contains(string(includeOut), "HOME=/root") {
		t.Fatalf("include-image-env output did not include image env:\n%s", string(includeOut))
	}
}
