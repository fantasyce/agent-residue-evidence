package versioninfo

import "fmt"

var (
	Version = "0.2.0"
	Commit  = "unknown"
)

func String() string {
	return fmt.Sprintf("agent-residue-evidence %s (%s)", Version, Commit)
}
