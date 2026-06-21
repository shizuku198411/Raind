package container

import (
	"testing"

	"raind/internal/condenser/store/psm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContainerServiceParseImageRef(t *testing.T) {
	service := &ContainerService{}

	tests := []struct {
		name string
		in   string
		repo string
		ref  string
	}{
		{name: "implicit library latest", in: "alpine", repo: "library/alpine", ref: "latest"},
		{name: "implicit library tag", in: "alpine:3.20", repo: "library/alpine", ref: "3.20"},
		{name: "explicit library", in: "library/ubuntu:24.04", repo: "library/ubuntu", ref: "24.04"},
		{name: "registry normalized", in: "docker.io/library/busybox:latest", repo: "library/busybox", ref: "latest"},
		{name: "custom registry", in: "example.com/ns/app:v1", repo: "example.com/ns/app", ref: "v1"},
		{name: "digest", in: "nginx@sha256:abc", repo: "library/nginx", ref: "sha256:abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, ref, err := service.parseImageRef(tt.in)
			require.NoError(t, err)
			assert.Equal(t, tt.repo, repo)
			assert.Equal(t, tt.ref, ref)
		})
	}
}

func TestContainerServiceBuildDeviceMounts(t *testing.T) {
	service := &ContainerService{}

	got, err := service.buildDeviceMounts([]string{"/dev/null", "/dev/fuse:/dev/fuse:r", "/dev/kvm:/dev/kvm:rwm"})

	require.NoError(t, err)
	assert.Equal(t, []string{
		"/dev/null:/dev/null:bind,rprivate,dev",
		"/dev/fuse:/dev/fuse:bind,rprivate,dev,ro",
		"/dev/kvm:/dev/kvm:bind,rprivate,dev",
	}, got)
}

func TestContainerServiceBuildDeviceMountsRejectsInvalid(t *testing.T) {
	service := &ContainerService{}

	_, err := service.buildDeviceMounts([]string{"relative:/dev/null"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "absolute path required")
}

func TestContainerServiceBuildCommandQuotesArguments(t *testing.T) {
	service := &ContainerService{}

	got := service.buildCommand([]string{"/bin/sh", "-c"}, []string{"echo hello"})

	assert.Equal(t, "/bin/sh -c 'echo hello'", got)
}

func TestContainerServiceResolvSearchDomainsForPlainContainer(t *testing.T) {
	service := &ContainerService{}

	got := service.resolvSearchDomains("raind0", "")

	assert.Equal(t, []string{"raind0.raind"}, got)
}

func TestContainerServiceResolvSearchDomainsForPodContainer(t *testing.T) {
	service := &ContainerService{
		psmHandler: &fakePsmHandler{pods: map[string]psm.PodInfo{
			"pod-1": {PodId: "pod-1", Namespace: "default"},
		}},
	}

	got := service.resolvSearchDomains("raind0", "pod-1")

	assert.Equal(t, []string{
		"default.svc.cluster.local",
		"svc.cluster.local",
		"cluster.local",
		"raind0.raind",
	}, got)
}
