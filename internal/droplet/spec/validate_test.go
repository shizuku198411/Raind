package spec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateBasicAcceptsMinimalSpec(t *testing.T) {
	major := int64(1)
	minor := int64(3)
	err := ValidateBasic(Spec{
		OciVersion: "1.3.0",
		Root:       RootObject{Path: "/rootfs"},
		Process:    ProcessObject{Args: []string{"/bin/sh"}, ConsoleSize: &ConsoleSizeObject{Height: 24, Width: 80}},
		LinuxSpec: LinuxSpecObject{
			Namespaces:  []NamespaceObject{{Type: "mount"}},
			UIDMappings: []IDMappingObject{{ContainerID: 0, HostID: 1000, Size: 1}},
			GIDMappings: []IDMappingObject{{ContainerID: 0, HostID: 1000, Size: 1}},
			Resources: ResourceObject{
				Devices: []DeviceCgroupObject{
					{Allow: true, Type: "c", Major: &major, Minor: &minor, Access: "rwm"},
					{Allow: false, Type: "a"},
				},
			},
		},
	})

	require.NoError(t, err)
}

func TestValidateBasicRejectsMissingRequiredFields(t *testing.T) {
	tests := []struct {
		name          string
		containerSpec Spec
		want          string
	}{
		{name: "missing version", containerSpec: Spec{Root: RootObject{Path: "/rootfs"}, Process: ProcessObject{Args: []string{"/bin/sh"}}}, want: "ociVersion is required"},
		{name: "missing root", containerSpec: Spec{OciVersion: "1.3.0", Process: ProcessObject{Args: []string{"/bin/sh"}}}, want: "root.path is required"},
		{name: "missing args", containerSpec: Spec{OciVersion: "1.3.0", Root: RootObject{Path: "/rootfs"}}, want: "process.args[0] is required"},
		{name: "bad console size zero", containerSpec: Spec{OciVersion: "1.3.0", Root: RootObject{Path: "/rootfs"}, Process: ProcessObject{Args: []string{"/bin/sh"}, ConsoleSize: &ConsoleSizeObject{Height: 0, Width: 80}}}, want: "process.consoleSize"},
		{name: "bad console size large", containerSpec: Spec{OciVersion: "1.3.0", Root: RootObject{Path: "/rootfs"}, Process: ProcessObject{Args: []string{"/bin/sh"}, ConsoleSize: &ConsoleSizeObject{Height: 24, Width: MaxConsoleSize + 1}}}, want: "process.consoleSize"},
		{name: "bad namespace", containerSpec: Spec{OciVersion: "1.3.0", Root: RootObject{Path: "/rootfs"}, Process: ProcessObject{Args: []string{"/bin/sh"}}, LinuxSpec: LinuxSpecObject{Namespaces: []NamespaceObject{{Type: "bad"}}}}, want: "unsupported linux namespace type"},
		{name: "bad mapping", containerSpec: Spec{OciVersion: "1.3.0", Root: RootObject{Path: "/rootfs"}, Process: ProcessObject{Args: []string{"/bin/sh"}}, LinuxSpec: LinuxSpecObject{UIDMappings: []IDMappingObject{{ContainerID: 0, HostID: 1000}}}}, want: "mapping size"},
		{name: "bad device", containerSpec: Spec{OciVersion: "1.3.0", Root: RootObject{Path: "/rootfs"}, Process: ProcessObject{Args: []string{"/bin/sh"}}, LinuxSpec: LinuxSpecObject{Devices: []DeviceObject{{Path: "/dev/test", Type: "x"}}}}, want: "unsupported linux device type"},
		{name: "bad resources device type", containerSpec: Spec{OciVersion: "1.3.0", Root: RootObject{Path: "/rootfs"}, Process: ProcessObject{Args: []string{"/bin/sh"}}, LinuxSpec: LinuxSpecObject{Resources: ResourceObject{Devices: []DeviceCgroupObject{{Allow: true, Type: "x"}}}}}, want: "unsupported linux resources device type"},
		{name: "bad resources device access", containerSpec: Spec{OciVersion: "1.3.0", Root: RootObject{Path: "/rootfs"}, Process: ProcessObject{Args: []string{"/bin/sh"}}, LinuxSpec: LinuxSpecObject{Resources: ResourceObject{Devices: []DeviceCgroupObject{{Allow: true, Type: "c", Access: "rx"}}}}}, want: "unsupported linux resources device access"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBasic(tt.containerSpec)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}
