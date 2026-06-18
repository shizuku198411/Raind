package buildinfo

import "fmt"

var (
	Version = "dev"
	Commit  = "unknown"
	BuiltAt = "unknown"
)

func VersionString() string {
	return fmt.Sprintf("%s (commit: %s)", Version, Commit)
}
