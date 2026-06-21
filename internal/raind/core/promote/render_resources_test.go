package promote

import (
	"bytes"
	"testing"
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
