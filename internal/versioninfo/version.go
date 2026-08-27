package versioninfo

import "fmt"

var (
	Version = "0.1.0-dev"
	Commit  = "unknown"
)

func String() string {
	return fmt.Sprintf("agent-residue-evidence %s (%s)", Version, Commit)
}
