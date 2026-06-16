package securityprofile

import "testing"

func TestServiceListPrint(t *testing.T) {
	service := NewServiceList()
	if err := service.print([]ProfileSummary{{
		Name:              "default",
		Type:              ProfileTypeBuiltIn,
		CapabilitiesCount: 14,
		SeccompEnabled:    true,
		AppArmorProfile:   "raind-default",
	}}); err != nil {
		t.Fatalf("print failed: %v", err)
	}
}
