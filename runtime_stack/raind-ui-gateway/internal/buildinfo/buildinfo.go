package buildinfo

import "fmt"

var (
	Version = "dev"
	Commit  = "unknown"
	BuiltAt = "unknown"
)

func String() string {
	return fmt.Sprintf("version=%s commit=%s built_at=%s", Version, Commit, BuiltAt)
}
