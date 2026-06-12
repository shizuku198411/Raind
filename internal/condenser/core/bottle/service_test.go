package bottle

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBottleServiceDecodeSpecNormalizesAliases(t *testing.T) {
	service := &BottleService{}

	spec, err := service.DecodeSpec([]byte(`
bottle:
  name: app
services:
  web:
    image: alpine:latest
    cap-add: ["CAP_NET_ADMIN"]
    capAdd: ["CAP_SYS_TIME", "CAP_NET_ADMIN"]
    devices: ["/dev/null"]
    device: ["/dev/zero"]
`))

	require.NoError(t, err)
	assert.Equal(t, "app", spec.Bottle.Name)
	assert.ElementsMatch(t, []string{"CAP_NET_ADMIN", "CAP_SYS_TIME"}, spec.Services["web"].CapAdd)
	assert.ElementsMatch(t, []string{"/dev/null", "/dev/zero"}, spec.Services["web"].Device)
}

func TestBottleServiceBuildStartOrderHonorsDependencies(t *testing.T) {
	service := &BottleService{}
	spec := &BottleSpec{Services: map[string]ServiceSpec{
		"web": {DependsOn: []string{"api"}},
		"api": {DependsOn: []string{"db"}},
		"db":  {},
	}}

	order, err := service.BuildStartOrder(spec)

	require.NoError(t, err)
	assert.Equal(t, []string{"db", "api", "web"}, order)
}

func TestBottleServiceBuildStartOrderRejectsUnknownDependency(t *testing.T) {
	service := &BottleService{}
	_, err := service.BuildStartOrder(&BottleSpec{Services: map[string]ServiceSpec{
		"web": {DependsOn: []string{"missing"}},
	}})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown service")
}

func TestBottleServiceBuildStartOrderRejectsCycle(t *testing.T) {
	service := &BottleService{}
	_, err := service.BuildStartOrder(&BottleSpec{Services: map[string]ServiceSpec{
		"a": {DependsOn: []string{"b"}},
		"b": {DependsOn: []string{"a"}},
	}})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "dependency cycle")
}
