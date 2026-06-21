package promote

import (
	"raind/internal/raind/core/container"
	policycore "raind/internal/raind/core/policy"
	"strings"
	"testing"
)

func TestConvertSecurityPoliciesFiltersPromotedContainers(t *testing.T) {
	policyList := []policycore.PolicyModel{
		{
			Source:      policycore.HostInfoModel{ContainerName: "web"},
			Destination: policycore.HostInfoModel{ContainerName: "db"},
			Protocol:    "tcp",
			DestPort:    3306,
			Comment:     "allow web to db",
		},
		{
			Source:      policycore.HostInfoModel{ContainerName: "web"},
			Destination: policycore.HostInfoModel{ContainerName: "cache"},
			Protocol:    "tcp",
			DestPort:    6379,
		},
	}
	inspects := []container.ContainerInspectModel{
		{Name: "db", ContainerId: "db1"},
		{Name: "web", ContainerId: "web1"},
	}
	services := []ServiceDraft{{Name: "db"}, {Name: "web"}}

	policies := ConvertSecurityPolicies(inspects, services, policyList)
	if len(policies) != 1 {
		t.Fatalf("expected one matching policy, got %#v", policies)
	}
	got := policies[0]
	if got.Type != "east-west" || got.Source != "web" || got.Destination != "db" || got.Protocol != "tcp" || got.DestPort != 3306 || got.Comment != "allow web to db" {
		t.Fatalf("unexpected policy: %#v", got)
	}
}

func TestAttachSecurityPoliciesFromPolicyListRendersBottlePolicies(t *testing.T) {
	policyList := []policycore.PolicyModel{
		{
			Source:      policycore.HostInfoModel{ContainerName: "web"},
			Destination: policycore.HostInfoModel{ContainerName: "db"},
			Protocol:    "tcp",
			DestPort:    3306,
			Comment:     "manual allow",
		},
	}
	inspects := []container.ContainerInspectModel{
		{Name: "db", ImageRepository: "mysql", ImageReference: "latest"},
		{Name: "web", ImageRepository: "myapp", ImageReference: "latest"},
	}
	draft, err := BuildBottleDraftFromContainers(inspects, ContainerToBottleOptions{BottleName: "stack"})
	if err != nil {
		t.Fatal(err)
	}
	AttachSecurityPoliciesFromPolicyList(&draft, inspects, policyList)
	out, err := RenderBottlefile(draft)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	for _, want := range []string{
		`policies:`,
		`  - type: "east-west"`,
		`    source: "web"`,
		`    destination: "db"`,
		`    protocol: "tcp"`,
		`    dest_port: 3306`,
		`    comment: "manual allow"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered bottle yaml missing %q:\n%s", want, text)
		}
	}
}

func TestConvertSecurityPoliciesSupportsSingleContainerCustomServiceName(t *testing.T) {
	policyList := []policycore.PolicyModel{
		{
			Source:      policycore.HostInfoModel{ContainerName: "myapp"},
			Destination: policycore.HostInfoModel{ContainerName: "myapp"},
			Protocol:    "icmp",
		},
	}
	inspects := []container.ContainerInspectModel{{Name: "myapp", ContainerId: "ctr1"}}
	services := []ServiceDraft{{Name: "app"}}

	policies := ConvertSecurityPolicies(inspects, services, policyList)
	if len(policies) != 1 {
		t.Fatalf("expected one policy, got %#v", policies)
	}
	if policies[0].Source != "app" || policies[0].Destination != "app" {
		t.Fatalf("expected custom service name mapping, got %#v", policies[0])
	}
}

func TestConvertSecurityPoliciesSkipsRemovedPolicies(t *testing.T) {
	policyList := []policycore.PolicyModel{
		{
			Status:      "remove_next_commit",
			Source:      policycore.HostInfoModel{ContainerName: "web"},
			Destination: policycore.HostInfoModel{ContainerName: "db"},
			Protocol:    "tcp",
			DestPort:    3306,
		},
	}
	inspects := []container.ContainerInspectModel{{Name: "db"}, {Name: "web"}}
	services := []ServiceDraft{{Name: "db"}, {Name: "web"}}

	policies := ConvertSecurityPolicies(inspects, services, policyList)
	if len(policies) != 0 {
		t.Fatalf("expected removed policy to be skipped, got %#v", policies)
	}
}
