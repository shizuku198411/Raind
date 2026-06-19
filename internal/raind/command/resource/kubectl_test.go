package resourcecommand

import (
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func TestNormalizeResourceKindAcceptsKubectlAliases(t *testing.T) {
	tests := []struct {
		raw  string
		want kubectlResourceKind
	}{
		{raw: "pods", want: resourcePod},
		{raw: "po", want: resourcePod},
		{raw: "deploy", want: resourceDeployment},
		{raw: "deployments", want: resourceDeployment},
		{raw: "rs", want: resourceReplicaSet},
		{raw: "svc", want: resourceService},
		{raw: "cm", want: resourceConfigMap},
		{raw: "secrets", want: resourceSecret},
		{raw: "ns", want: resourceNamespace},
		{raw: "netpol", want: resourceNetworkPolicy},
		{raw: "np", want: resourceNetworkPolicy},
		{raw: "pvc", want: resourcePVC},
		{raw: "ing", want: resourceIngress},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got, ok := normalizeResourceKind(tt.raw)

			require.True(t, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSplitKindNameAcceptsSlashForm(t *testing.T) {
	kind, name, err := splitKindName("deployment/web", "")

	require.NoError(t, err)
	assert.Equal(t, resourceDeployment, kind)
	assert.Equal(t, "web", name)
}

func TestSplitKindNameUsesExplicitNameOverSlashForm(t *testing.T) {
	kind, name, err := splitKindName("deployment/web", "override")

	require.NoError(t, err)
	assert.Equal(t, resourceDeployment, kind)
	assert.Equal(t, "override", name)
}

func TestRequireResourceName(t *testing.T) {
	require.NoError(t, requireResourceName(resourcePVC, "db-data"))

	err := requireResourceName(resourcePVC, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "persistentvolumeclaim name is required")
}

func TestKubectlArgsIgnoresTrailingWatchAndNamespaceFlags(t *testing.T) {
	set := flag.NewFlagSet("get", flag.ContinueOnError)
	ctx := cli.NewContext(nil, set, nil)
	require.NoError(t, set.Parse([]string{"deploy", "-n", "demo", "-w"}))

	assert.Equal(t, []string{"deploy"}, kubectlArgs(ctx))
}

func TestKubectlArgsIgnoresLongNamespaceAndWatchFlags(t *testing.T) {
	set := flag.NewFlagSet("get", flag.ContinueOnError)
	ctx := cli.NewContext(nil, set, nil)
	require.NoError(t, set.Parse([]string{"deployment/web", "--namespace=demo", "--watch"}))

	assert.Equal(t, []string{"deployment/web"}, kubectlArgs(ctx))
}
