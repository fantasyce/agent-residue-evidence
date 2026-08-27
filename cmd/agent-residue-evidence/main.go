package main

import (
	"fmt"
	"io"
	"os"

	"github.com/fantasyce/agent-residue-evidence/internal/versioninfo"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "--version" {
		fmt.Fprintln(stdout, versioninfo.String())
		return 0
	}
	fmt.Fprintln(stderr, "usage: agent-residue-evidence --version")
	return 2
}
