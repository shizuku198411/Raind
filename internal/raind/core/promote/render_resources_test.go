package promote

import (
	"bytes"
	"strings"
	"testing"

	bottlecore "raind/internal/raind/core/bottle"
	"raind/internal/raind/core/container"
)

func TestRenderResourceFilesUsesDraftIngressHost(t *testing.T) {
	body := []byte(`bottle:
  name: myapp
services:
  web:
    image: alpine:latest
    ports:
      - "8080:80"
`)

	draft, err := BuildResourceDraftFromBottle(body, BottleToResourcesOptions{IngressHost: "app.raind.local"})
	if err != nil {
		t.Fatalf("BuildResourceDraftFromBottle: %v", err)
	}

	files, err := RenderResourceFiles(draft, RenderResourcesOptions{})
	if err != nil {
		t.Fatalf("RenderResourceFiles: %v", err)
	}

	var foundIngress bool
	var all []byte
	for _, f := range files {
		if f.Name == "06-ingress.yaml" {
			foundIngress = true
			if !bytes.Contains(f.Data, []byte(`host: "app.raind.local"`)) {
				t.Fatalf("ingress file does not contain requested host:\n%s", f.Data)
			}
		}
		if f.Name == "all.yaml" {
			all = f.Data
		}
	}
	if !foundIngress {
		t.Fatalf("expected 06-ingress.yaml to be generated")
	}
	if !bytes.Contains(all, []byte("kind: Ingress")) {
		t.Fatalf("expected all.yaml to include Ingress:\n%s", all)
	}
}

func TestRenderResourceFilesErrorsWhenIngressHostHasNoTCPServicePort(t *testing.T) {
	draft := BottleDraft{
		BottleName:  "myapp",
		IngressHost: "app.raind.local",
		Services: []ServiceDraft{{
			Name:  "worker",
			Image: "alpine:latest",
		}},
	}

	_, err := RenderResourceFiles(draft, RenderResourcesOptions{})
	if err == nil {
		t.Fatalf("expected error when ingress host is set without a TCP service port")
	}
}

func TestBuildResourceDraftFromBottleParsesCurrentPromoteSubset(t *testing.T) {
	body := []byte(`bottle:
  name: My_App
services:
  db:
    image: mysql:8
    command:
      - mysqld
    env:
      - MYSQL_ROOT_PASSWORD=root-secret
      - MYSQL_DATABASE=app
    mount:
      - /host/mysql:/var/lib/mysql:ro
    cap-add:
      - NET_ADMIN
    capDrop:
      - MKNOD
    tty: true
  web:
    image: web:dev
    env:
      - APP_ENV=dev
      - MYSQL_HOST=db
    ports:
      - "8080:80"
      - "5353:53:udp"
    depends_on:
      - db
policies:
  - type: east-west
    source: web
    destination: db
    protocol: tcp
    dest_port: 3306
  - type: east-west
    source: web
    destination: db
    protocol: icmp
`)

	draft, err := BuildResourceDraftFromBottle(body, BottleToResourcesOptions{Namespace: "dev-ns", IngressHost: "app.raind.local"})
	if err != nil {
		t.Fatal(err)
	}
	if draft.SourceContainer != "bottle/My_App" || draft.BottleName != "dev-ns" || draft.IngressHost != "app.raind.local" {
		t.Fatalf("unexpected draft metadata: %#v", draft)
	}
	if len(draft.Services) != 2 || draft.Services[0].Name != "db" || draft.Services[1].Name != "web" {
		t.Fatalf("unexpected services: %#v", draft.Services)
	}
	db := draft.Services[0]
	if len(db.Env) != 2 || db.Env[1].Key != "MYSQL_ROOT_PASSWORD" || !db.Env[1].Sensitive {
		t.Fatalf("expected secret-like db env classification, got %#v", db.Env)
	}
	if len(db.Mounts) != 1 || db.Mounts[0].Destination != "/var/lib/mysql" || !db.Mounts[0].ReadOnly {
		t.Fatalf("unexpected db mounts: %#v", db.Mounts)
	}
	if len(db.CapAdd) != 1 || db.CapAdd[0] != "NET_ADMIN" || len(db.CapDrop) != 1 || db.CapDrop[0] != "MKNOD" || !db.Tty {
		t.Fatalf("unexpected capability/tty fields: %#v", db)
	}
	web := draft.Services[1]
	if len(web.Ports) != 2 || web.Ports[0].ContainerPort != 53 || web.Ports[0].Protocol != "udp" || web.Ports[1].ContainerPort != 80 || web.Ports[1].Protocol != "tcp" {
		t.Fatalf("expected sorted TCP/UDP ports, got %#v", web.Ports)
	}
	if len(web.DependsOn) != 1 || web.DependsOn[0] != "db" {
		t.Fatalf("expected depends_on to be preserved, got %#v", web.DependsOn)
	}
	if len(draft.Policies) != 2 || draft.Policies[0].Protocol != "icmp" || draft.Policies[1].Protocol != "tcp" || draft.Policies[1].DestPort != 3306 {
		t.Fatalf("unexpected policies: %#v", draft.Policies)
	}
}

func TestRenderResourceFilesCoversGeneratedManifestCombinations(t *testing.T) {
	draft := BottleDraft{
		SourceContainer: "running bottle/myapp",
		BottleName:      "myapp",
		IngressHost:     "app.raind.local",
		Services: []ServiceDraft{
			{
				Name:    "db",
				Image:   "mysql:8",
				Command: []string{"mysqld"},
				Env: []EnvVar{
					{Key: "MYSQL_DATABASE", Value: "app"},
					{Key: "MYSQL_ROOT_PASSWORD", Value: "root-secret", Sensitive: true},
				},
				Mounts:  []MountMapping{{Source: "/host/mysql", Destination: "/var/lib/mysql", ReadOnly: true}},
				CapAdd:  []string{"NET_ADMIN"},
				CapDrop: []string{"MKNOD"},
				Tty:     true,
			},
			{
				Name:      "web",
				Image:     "web:dev",
				Env:       []EnvVar{{Key: "APP_ENV", Value: "dev"}, {Key: "MYSQL_HOST", Value: "db"}},
				Ports:     []PortMapping{{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"}, {HostPort: 5353, ContainerPort: 53, Protocol: "udp"}},
				DependsOn: []string{"db"},
			},
		},
		Policies: []PolicyDraft{
			{Type: "east-west", Source: "web", Destination: "db", Protocol: "tcp", DestPort: 3306},
			{Type: "east-west", Source: "web", Destination: "db", Protocol: "icmp"},
		},
	}

	files, err := RenderResourceFiles(draft, RenderResourcesOptions{})
	if err != nil {
		t.Fatal(err)
	}
	byName := resourceFilesByName(files)
	for _, name := range []string{"00-namespace.yaml", "01-configmap.yaml", "02-secret.example.yaml", "03-pvcs.yaml", "04-deployments.yaml", "05-services.yaml", "06-ingress.yaml", "07-networkpolicies.yaml", "REVIEW.md", "all.yaml"} {
		if len(byName[name]) == 0 {
			t.Fatalf("expected generated file %s; got names %#v", name, resourceFileNames(files))
		}
	}
	if strings.Contains(string(byName["02-secret.example.yaml"]), "root-secret") || !strings.Contains(string(byName["02-secret.example.yaml"]), "<replace-me>") {
		t.Fatalf("secret example leaked or missed placeholder:\n%s", byName["02-secret.example.yaml"])
	}
	deployments := string(byName["04-deployments.yaml"])
	for _, want := range []string{
		`image: "mysql:8"`,
		`secretRef:`,
		`claimName: "db-mysql"`,
		`mountPath: "/var/lib/mysql"`,
		`readOnly: true`,
		`securityContext:`,
		`- "NET_ADMIN"`,
		`- "MKNOD"`,
		`tty: true`,
		`containerPort: 80`,
		`containerPort: 53`,
	} {
		if !strings.Contains(deployments, want) {
			t.Fatalf("deployments missing %q:\n%s", want, deployments)
		}
	}
	services := string(byName["05-services.yaml"])
	for _, want := range []string{`name: "db"`, `port: 3306`, `protocol: "UDP"`} {
		if !strings.Contains(services, want) {
			t.Fatalf("services missing %q:\n%s", want, services)
		}
	}
	if !strings.Contains(string(byName["01-configmap.yaml"]), `MYSQL_HOST: "db.myapp.svc.cluster.local"`) {
		t.Fatalf("service host env was not rewritten to Kubernetes Service FQDN:\n%s", byName["01-configmap.yaml"])
	}
	if !strings.Contains(string(byName["06-ingress.yaml"]), `host: "app.raind.local"`) || !strings.Contains(string(byName["06-ingress.yaml"]), `name: "web"`) {
		t.Fatalf("ingress did not target requested host/web service:\n%s", byName["06-ingress.yaml"])
	}
	np := string(byName["07-networkpolicies.yaml"])
	if !strings.Contains(np, `kind: NetworkPolicy`) || !strings.Contains(np, `port: 3306`) {
		t.Fatalf("expected TCP NetworkPolicy with destination port:\n%s", np)
	}
	if strings.Contains(np, "ICMP") || strings.Contains(np, "icmp") {
		t.Fatalf("ICMP policy should be skipped by current Kubernetes-style subset:\n%s", np)
	}
	all := string(byName["all.yaml"])
	if !strings.Contains(all, "kind: Ingress") || !strings.Contains(all, "kind: NetworkPolicy") || strings.Contains(all, "REVIEW.md") {
		t.Fatalf("combined manifest did not include resource docs only:\n%s", all)
	}
	review := string(byName["REVIEW.md"])
	if !strings.Contains(review, "Ingress draft was requested for host `app.raind.local`") || !strings.Contains(review, "depends_on: db") {
		t.Fatalf("review missed ingress/dependency notes:\n%s", review)
	}
}

func TestRenderResourceFilesPromotesWordPressMySQLConnectivity(t *testing.T) {
	body := []byte(`bottle:
  name: wordpress-mysql
services:
  mysql:
    image: mysql:8.0
    env:
      - MYSQL_ROOT_PASSWORD=root-password
      - MYSQL_DATABASE=wordpress
      - MYSQL_USER=wordpress
      - MYSQL_PASSWORD=wordpress-password
  wordpress:
    image: wordpress:latest
    env:
      - WORDPRESS_DB_HOST=mysql
      - WORDPRESS_DB_NAME=wordpress
      - WORDPRESS_DB_USER=wordpress
      - WORDPRESS_DB_PASSWORD=wordpress-password
    ports:
      - "9850:80"
    depends_on:
      - mysql
policies:
  - type: east-west
    source: wordpress
    destination: mysql
    protocol: tcp
    dest_port: 3306
`)

	draft, err := BuildResourceDraftFromBottle(body, BottleToResourcesOptions{IngressHost: "wordpress.raind.local"})
	if err != nil {
		t.Fatal(err)
	}
	files, err := RenderResourceFiles(draft, RenderResourcesOptions{})
	if err != nil {
		t.Fatal(err)
	}
	byName := resourceFilesByName(files)
	configMap := string(byName["01-configmap.yaml"])
	if !strings.Contains(configMap, `WORDPRESS_DB_HOST: "mysql.wordpress-mysql.svc.cluster.local"`) {
		t.Fatalf("wordpress DB host should be promoted to Service FQDN:\n%s", configMap)
	}
	services := string(byName["05-services.yaml"])
	for _, want := range []string{`name: "mysql"`, `port: 3306`, `targetPort: 3306`, `name: "wordpress"`, `port: 80`} {
		if !strings.Contains(services, want) {
			t.Fatalf("services missing %q:\n%s", want, services)
		}
	}
	ingress := string(byName["06-ingress.yaml"])
	if !strings.Contains(ingress, `name: "wordpress"`) || strings.Contains(ingress, `name: "mysql"`) {
		t.Fatalf("ingress should target the published wordpress service, not the policy-inferred mysql service:\n%s", ingress)
	}
}

func TestBuildResourceDraftFromBottleDetailRequiresRunningAndUsesRuntimeState(t *testing.T) {
	detail := bottlecore.BottleDetailModel{
		BottleName: "myapp",
		Services: map[string]bottlecore.BottleServiceModel{
			"web": {
				Image:     "spec/web:dev",
				Command:   []string{"spec-cmd"},
				Env:       []string{"APP_ENV=dev"},
				Ports:     []string{"8080:80"},
				DependsOn: []string{"db"},
			},
			"db": {
				Image: "mysql:8",
				Env:   []string{"MYSQL_ROOT_PASSWORD=root-secret"},
			},
		},
		Containers: map[string]container.ContainerStateModel{
			"web": {
				ContainerId: "ctr-web",
				State:       "running",
				Repository:  "runtime/web",
				Reference:   "sha256:abcdef",
				Command:     []string{"runtime-cmd", "--serve"},
				Forwards:    []container.ForwardInfoModel{{HostPort: 18080, ContainerPort: 8080, Protocol: "tcp"}},
			},
			"db": {
				ContainerId: "ctr-db",
				State:       "running",
				Repository:  "runtime/db",
				Reference:   "latest",
			},
		},
		Policies: []bottlecore.BottlePolicyModel{{Type: "east-west", Source: "web", Destination: "db", Protocol: "tcp", DestPort: 3306}},
	}

	draft, err := BuildResourceDraftFromBottleDetail(detail, BottleToResourcesOptions{Namespace: "dev", IngressHost: "app.raind.local"})
	if err != nil {
		t.Fatal(err)
	}
	if draft.SourceContainer != "running bottle/myapp" || draft.BottleName != "dev" || draft.IngressHost != "app.raind.local" {
		t.Fatalf("unexpected draft metadata: %#v", draft)
	}
	if len(draft.Services) != 2 || draft.Services[1].Name != "web" {
		t.Fatalf("unexpected services: %#v", draft.Services)
	}
	web := draft.Services[1]
	if web.Image != "runtime/web:sha256:abcdef" {
		t.Fatalf("runtime image should win over Bottlefile image, got %q", web.Image)
	}
	if len(web.Command) != 2 || web.Command[0] != "runtime-cmd" {
		t.Fatalf("runtime command should win, got %#v", web.Command)
	}
	if len(web.Ports) != 1 || web.Ports[0].HostPort != 18080 || web.Ports[0].ContainerPort != 8080 {
		t.Fatalf("runtime forwards should win over Bottlefile ports, got %#v", web.Ports)
	}
	if len(draft.Policies) != 1 || draft.Policies[0].DestPort != 3306 {
		t.Fatalf("expected runtime bottle policies, got %#v", draft.Policies)
	}
}

func TestBuildResourceDraftFromBottleDetailRejectsNonRunningBottleStates(t *testing.T) {
	base := bottlecore.BottleDetailModel{
		BottleName: "myapp",
		Services: map[string]bottlecore.BottleServiceModel{
			"web": {Image: "web:dev"},
		},
	}
	if _, err := BuildResourceDraftFromBottleDetail(base, BottleToResourcesOptions{}); err == nil {
		t.Fatalf("expected error when no runtime containers exist")
	}

	base.Containers = map[string]container.ContainerStateModel{
		"web": {ContainerId: "ctr-web", State: "stopped"},
	}
	if _, err := BuildResourceDraftFromBottleDetail(base, BottleToResourcesOptions{}); err == nil {
		t.Fatalf("expected error when runtime container is not running")
	}
}

func resourceFilesByName(files []ResourceFile) map[string][]byte {
	out := map[string][]byte{}
	for _, f := range files {
		out[f.Name] = f.Data
	}
	return out
}

func resourceFileNames(files []ResourceFile) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, f.Name)
	}
	return out
}

func TestBuildResourceDraftFromBottleDetailOmitsDefaultLibraryPrefixFromRuntimeImages(t *testing.T) {
	detail := bottlecore.BottleDetailModel{
		BottleName: "myapp",
		Services: map[string]bottlecore.BottleServiceModel{
			"mysql": {Image: "mysql:8.0"},
			"web":   {Image: "library/wordpress:latest"},
		},
		Containers: map[string]container.ContainerStateModel{
			"mysql": {ContainerId: "ctr-mysql", State: "running", Repository: "library/mysql", Reference: "8.0"},
			"web":   {ContainerId: "ctr-web", State: "running", Repository: "docker.io/library/wordpress", Reference: "latest"},
		},
	}

	draft, err := BuildResourceDraftFromBottleDetail(detail, BottleToResourcesOptions{})
	if err != nil {
		t.Fatal(err)
	}
	files, err := RenderResourceFiles(draft, RenderResourcesOptions{})
	if err != nil {
		t.Fatal(err)
	}
	deployments := string(resourceFilesByName(files)["04-deployments.yaml"])
	for _, want := range []string{`image: "mysql:8.0"`, `image: "wordpress:latest"`} {
		if !strings.Contains(deployments, want) {
			t.Fatalf("deployment manifest missing %q:\n%s", want, deployments)
		}
	}
	if strings.Contains(deployments, "library/mysql") || strings.Contains(deployments, "library/wordpress") {
		t.Fatalf("deployment manifest should not include default library prefix:\n%s", deployments)
	}
}

func TestBuildResourceDraftFromBottleOmitsDefaultLibraryPrefixFromSpecImages(t *testing.T) {
	body := []byte(`bottle:
  name: myapp
services:
  mysql:
    image: library/mysql:8.0
  wordpress:
    image: docker.io/library/wordpress:latest
  app:
    image: registry.example.com/library/app:v1
`)

	draft, err := BuildResourceDraftFromBottle(body, BottleToResourcesOptions{})
	if err != nil {
		t.Fatal(err)
	}
	files, err := RenderResourceFiles(draft, RenderResourcesOptions{})
	if err != nil {
		t.Fatal(err)
	}
	deployments := string(resourceFilesByName(files)["04-deployments.yaml"])
	for _, want := range []string{`image: "mysql:8.0"`, `image: "wordpress:latest"`, `image: "registry.example.com/library/app:v1"`} {
		if !strings.Contains(deployments, want) {
			t.Fatalf("deployment manifest missing %q:\n%s", want, deployments)
		}
	}
	if strings.Contains(deployments, "library/mysql") || strings.Contains(deployments, "library/wordpress") {
		t.Fatalf("deployment manifest should not include default library prefix:\n%s", deployments)
	}
}
