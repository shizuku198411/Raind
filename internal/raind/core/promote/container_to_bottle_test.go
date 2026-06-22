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
		t.Fatalf("rendered bottle.yaml leaked secret value:\n%s", text)
	}
	if !strings.Contains(text, "APP_ENV=dev") {
		t.Fatalf("rendered bottle.yaml did not include non-secret env:\n%s", text)
	}
	if !strings.Contains(text, "env example: DB_PASSWORD=<redacted>") {
		t.Fatalf("rendered bottle.yaml did not include redacted secret hint:\n%s", text)
	}
	if !strings.Contains(text, `"8080:3000"`) {
		t.Fatalf("rendered bottle.yaml did not include port mapping:\n%s", text)
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
		t.Fatalf("rendered bottle.yaml included internal mount:\n%s", text)
	}
	if !strings.Contains(text, `"/host/www:/usr/share/nginx/html:ro"`) {
		t.Fatalf("rendered bottle.yaml did not include read-only user mount:\n%s", text)
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

func TestBuildBottleDraftFromContainerIncludesBindMountWithEmptyType(t *testing.T) {
	inspect := container.ContainerInspectModel{
		ContainerId:     "ctr-db",
		Name:            "db",
		ImageRepository: "library/alpine",
		ImageReference:  "latest",
		Config: map[string]any{
			"mounts": []any{
				map[string]any{
					"type":        "",
					"source":      "/mnt/data",
					"destination": "/data",
					"options":     []any{"bind"},
				},
			},
		},
	}

	draft, err := BuildBottleDraftFromContainer(inspect, ContainerToBottleOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(draft.Mounts) != 1 {
		t.Fatalf("expected one mount, got %#v", draft.Mounts)
	}
	if draft.Mounts[0].Source != "/mnt/data" || draft.Mounts[0].Destination != "/data" {
		t.Fatalf("unexpected mount: %#v", draft.Mounts[0])
	}

	out, err := RenderBottlefile(draft)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"/mnt/data:/data"`) {
		t.Fatalf("rendered bottle.yaml did not include bind mount:\n%s", string(out))
	}
}

func TestBuildBottleDraftFromContainersGeneratesMultipleServicesAndDependsOn(t *testing.T) {
	inspects := []container.ContainerInspectModel{
		{
			ContainerId:     "db1",
			Name:            "db",
			ImageRepository: "mysql",
			ImageReference:  "latest",
			Config: map[string]any{
				"process": map[string]any{
					"env": []any{"MYSQL_ROOT_PASSWORD=secret", "PATH=/usr/bin"},
				},
			},
		},
		{
			ContainerId:     "web1",
			Name:            "web",
			ImageRepository: "myapp",
			ImageReference:  "latest",
			Config: map[string]any{
				"process": map[string]any{
					"env": []any{"MYSQL_HOST=db", "APP_ENV=dev", "PATH=/usr/bin"},
				},
			},
			Forwards: []container.ForwardInfoModel{{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"}},
		},
	}

	draft, err := BuildBottleDraftFromContainers(inspects, ContainerToBottleOptions{BottleName: "stack"})
	if err != nil {
		t.Fatal(err)
	}
	if len(draft.Services) != 2 {
		t.Fatalf("expected two services, got %#v", draft.Services)
	}
	if draft.Services[0].Name != "db" || draft.Services[1].Name != "web" {
		t.Fatalf("services were not sorted by service name: %#v", draft.Services)
	}
	if len(draft.Services[1].DependsOn) != 1 || draft.Services[1].DependsOn[0] != "db" {
		t.Fatalf("expected web to depend on db, got %#v", draft.Services[1].DependsOn)
	}

	out, err := RenderBottlefile(draft)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	for _, want := range []string{
		`bottle:`,
		`  name: "stack"`,
		`  db:`,
		`    image: "mysql:latest"`,
		`  web:`,
		`    image: "myapp:latest"`,
		`      - "MYSQL_HOST=db"`,
		`    depends_on:`,
		`      - "db"`,
		`      - "8080:80"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered bottle.yaml missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "root-secret") {
		t.Fatalf("rendered bottle.yaml leaked secret value:\n%s", text)
	}
	if !strings.Contains(text, "env example: MYSQL_ROOT_PASSWORD=<redacted>") {
		t.Fatalf("rendered bottle.yaml did not include redacted secret hint;\n%s", text)
	}
}

func TestBuildBottleDraftFromContainersInfersDependencyFromURLValue(t *testing.T) {
	inspects := []container.ContainerInspectModel{
		{Name: "db", ImageRepository: "postgres", ImageReference: "latest"},
		{
			Name:            "api",
			ImageRepository: "api",
			ImageReference:  "latest",
			Config: map[string]any{
				"process": map[string]any{
					"env": []any{"DATABASE_URL=postgres://user:pass@db:5432/app"},
				},
			},
		},
	}

	draft, err := BuildBottleDraftFromContainers(inspects, ContainerToBottleOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var api ServiceDraft
	for _, svc := range draft.Services {
		if svc.Name == "api" {
			api = svc
		}
	}
	if len(api.DependsOn) != 1 || api.DependsOn[0] != "db" {
		t.Fatalf("expected api to depend on db, got %#v", api.DependsOn)
	}
}

func TestBuildBottleDraftFromContainersRejectsServiceNameForMultipleTargets(t *testing.T) {
	_, err := BuildBottleDraftFromContainers(
		[]container.ContainerInspectModel{{Name: "db"}, {Name: "web"}},
		ContainerToBottleOptions{ServiceName: "app"},
	)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildBottleDraftFromContainersRejectsDuplicateServiceNames(t *testing.T) {
	_, err := BuildBottleDraftFromContainers(
		[]container.ContainerInspectModel{{Name: "my_app"}, {Name: "my-app"}},
		ContainerToBottleOptions{},
	)
	if err == nil {
		t.Fatal("expected duplicate service name error")
	}
}

func TestRenderBottleReviewRedactsSecretValues(t *testing.T) {
	draft := BottleDraft{
		SourceContainer: "container/myapp",
		BottleName:      "myapp",
		Services: []ServiceDraft{{
			Name:  "app",
			Image: "myapp:dev",
			Env: []EnvVar{
				{Key: "APP_ENV", Value: "dev"},
				{Key: "DB_PASSWORD", Value: "super-secret", Sensitive: true},
			},
			Ports:  []PortMapping{{HostPort: 8080, ContainerPort: 3000, Protocol: "tcp"}},
			Mounts: []MountMapping{{Source: "/host/data", Destination: "/data"}},
		}},
	}

	out, err := RenderBottleReview(draft)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	if strings.Contains(text, "super-secret") {
		t.Fatalf("rendered review leaked secret value:\n%s", text)
	}
	for _, want := range []string{
		"# Generated Bottle Draft Review",
		"Source container(s): `container/myapp`",
		"Service `app`",
		"Secret candidates: `DB_PASSWORD`",
		"Review host-specific absolute mounts: `app:/host/data -> /data`.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered review missing %q:\n%s", want, text)
		}
	}
}

func TestBottleReviewPathForOutputUsesOutputDirectory(t *testing.T) {
	if got := BottleReviewPathForOutput("out/bottle.yaml"); got != "out/REVIEW_BOTTLE.md" {
		t.Fatalf("unexpected review path: %s", got)
	}
	if got := BottleReviewPathForOutput("bottle.yaml"); got != "REVIEW_BOTTLE.md" {
		t.Fatalf("unexpected review path: %s", got)
	}
}

func TestBuildBottleDraftFromContainersCoversRuntimeFieldCombinations(t *testing.T) {
	inspects := []container.ContainerInspectModel{
		{
			ContainerId:     "ctr-db",
			Name:            "DB_Service",
			ImageRepository: "mysql",
			ImageReference:  "8",
			SecurityProfile: "strict",
			Tty:             true,
			Config: map[string]any{
				"process": map[string]any{
					"args": []any{"mysqld", "--skip-name-resolve"},
					"env":  []any{"MYSQL_ROOT_PASSWORD=root-secret", "MYSQL_DATABASE=app", "PATH=/usr/bin", "MYSQL_DATABASE=ignored-duplicate"},
				},
				"mounts": []any{
					map[string]any{"type": "bind", "source": "/host/mysql", "destination": "/var/lib/mysql", "options": []any{"bind", "ro"}},
					map[string]any{"type": "tmpfs", "source": "tmpfs", "destination": "/tmp"},
				},
			},
		},
		{
			ContainerId:     "ctr-web",
			Name:            "web",
			ImageRepository: "example.com/web",
			ImageReference:  "sha256:abcdef",
			Command:         []string{"/app/server"},
			Config: map[string]any{
				"process": map[string]any{
					"env": []any{"MYSQL_HOST=db-service", "APP_ENV=dev", "HOME=/root"},
				},
			},
			Forwards: []container.ForwardInfoModel{
				{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
				{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
				{HostPort: 5353, ContainerPort: 53, Protocol: "udp"},
				{HostPort: 0, ContainerPort: 9999, Protocol: "tcp"},
			},
		},
	}

	draft, err := BuildBottleDraftFromContainers(inspects, ContainerToBottleOptions{BottleName: "My_App"})
	if err != nil {
		t.Fatal(err)
	}
	if draft.BottleName != "My_App" {
		t.Fatalf("expected explicit bottle name to be preserved, got %q", draft.BottleName)
	}
	if draft.SourceContainer != "container/DB_Service, container/web" {
		t.Fatalf("unexpected source container string: %q", draft.SourceContainer)
	}
	if len(draft.Services) != 2 || draft.Services[0].Name != "db-service" || draft.Services[1].Name != "web" {
		t.Fatalf("unexpected services: %#v", draft.Services)
	}
	db := draft.Services[0]
	if db.Image != "mysql:8" || !db.Tty {
		t.Fatalf("unexpected db image/tty: %#v", db)
	}
	if len(db.Command) != 2 || db.Command[0] != "mysqld" || db.Command[1] != "--skip-name-resolve" {
		t.Fatalf("expected command from process args, got %#v", db.Command)
	}
	if len(db.Env) != 2 || db.Env[0].Key != "MYSQL_DATABASE" || db.Env[1].Key != "MYSQL_ROOT_PASSWORD" || !db.Env[1].Sensitive {
		t.Fatalf("expected sorted env with secret classification and duplicate/default filtering, got %#v", db.Env)
	}
	if len(db.Mounts) != 1 || db.Mounts[0].Destination != "/var/lib/mysql" || !db.Mounts[0].ReadOnly {
		t.Fatalf("unexpected db mounts: %#v", db.Mounts)
	}
	web := draft.Services[1]
	if web.Image != "example.com/web@sha256:abcdef" {
		t.Fatalf("expected digest image format, got %q", web.Image)
	}
	if len(web.Ports) != 2 || web.Ports[0].HostPort != 5353 || web.Ports[0].Protocol != "udp" || web.Ports[1].HostPort != 8080 || web.Ports[1].Protocol != "tcp" {
		t.Fatalf("expected deduplicated and sorted ports, got %#v", web.Ports)
	}
	if len(web.DependsOn) != 1 || web.DependsOn[0] != "db-service" {
		t.Fatalf("expected dependency inferred from MYSQL_HOST, got %#v", web.DependsOn)
	}
	if len(draft.Warnings) != 1 || draft.Warnings[0].Code != "security-profile" {
		t.Fatalf("expected security profile warning, got %#v", draft.Warnings)
	}
}

func TestBuildBottleDraftFromContainerAllowsPodContainerWhenExplicit(t *testing.T) {
	draft, err := BuildBottleDraftFromContainer(
		container.ContainerInspectModel{Name: "pod-app", PodId: "pod1", ImageRepository: "alpine", ImageReference: "latest"},
		ContainerToBottleOptions{AllowPodContainer: true, ServiceName: "app"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if draft.ServiceName != "app" || draft.Image != "alpine:latest" {
		t.Fatalf("unexpected draft: %#v", draft)
	}
}

func TestRenderBottleReviewIncludesPoliciesAndWarnings(t *testing.T) {
	draft := BottleDraft{
		SourceContainer: "container/web, container/db",
		BottleName:      "myapp",
		Services: []ServiceDraft{
			{Name: "db", Image: "mysql:8"},
			{Name: "web", Image: "web:dev", DependsOn: []string{"db"}},
		},
		Policies: []PolicyDraft{{Type: "east-west", Source: "web", Destination: "db", Protocol: "tcp", DestPort: 3306}},
		Warnings: []Warning{{Code: "missing-image", Message: "image missing"}},
	}

	out, err := RenderBottleReview(draft)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	for _, want := range []string{
		"Services generated: 2",
		"Service `web`",
		"Inferred dependencies: `db`",
		"`web` -> `db` (type `east-west`, protocol `tcp`, destination port `3306`)",
		"image missing",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered review missing %q:\n%s", want, text)
		}
	}
}

func TestPromoteImageOutputOmitsDefaultLibraryPrefix(t *testing.T) {
	cases := []struct {
		name string
		repo string
		ref  string
		want string
	}{
		{name: "implicit docker hub library tag", repo: "library/mysql", ref: "8.0", want: "mysql:8.0"},
		{name: "docker io library tag", repo: "docker.io/library/wordpress", ref: "latest", want: "wordpress:latest"},
		{name: "index docker io library digest", repo: "index.docker.io/library/nginx", ref: "sha256:abcdef", want: "nginx@sha256:abcdef"},
		{name: "non library namespace preserved", repo: "docker.io/acme/web", ref: "v1", want: "acme/web:v1"},
		{name: "registry preserved", repo: "registry.example.com/library/mysql", ref: "8.0", want: "registry.example.com/library/mysql:8.0"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := formatImage(tc.repo, tc.ref)
			if got != tc.want {
				t.Fatalf("formatImage(%q, %q) = %q, want %q", tc.repo, tc.ref, got, tc.want)
			}
		})
	}
}

func TestRenderComposefileOmitsPolicies(t *testing.T) {
	draft := BottleDraft{
		SourceContainer: "container/app",
		BottleName:      "stack",
		Services: []ServiceDraft{{
			Name:      "app",
			Image:     "wordpress:latest",
			Env:       []EnvVar{{Key: "APP_ENV", Value: "dev"}, {Key: "DB_PASSWORD", Value: "P@ssw0rd", Sensitive: true}},
			Ports:     []PortMapping{{HostPort: 9850, ContainerPort: 80}},
			DependsOn: []string{"mysql"},
		}},
		Policies: []PolicyDraft{{Type: "ew", Source: "app", Destination: "mysql", Protocol: "tcp", DestPort: 3306}},
	}

	out, err := RenderComposefile(draft)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	if strings.Contains(text, "policies:") || strings.Contains(text, "dest_port:") {
		t.Fatalf("rendered compose.yaml included Raind-only policies:\n%s", text)
	}
	for _, want := range []string{
		"compose.yaml",
		"services:",
		"  app:",
		"    image: \"wordpress:latest\"",
		"    depends_on:",
		"      - \"mysql\"",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered compose.yaml missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "P@ssw0rd") {
		t.Fatalf("rendered compose.yaml leaked secret value:\n%s", text)
	}
}

func TestDefaultComposePromotionOutput(t *testing.T) {
	if DefaultComposePromotionOutput != "raind_promote/compose/compose.yaml" {
		t.Fatalf("unexpected compose output path: %s", DefaultComposePromotionOutput)
	}
}
