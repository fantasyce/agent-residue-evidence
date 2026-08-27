package main

import (
	"io"
	"os"

	"github.com/fantasyce/agent-residue-evidence/internal/cli"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	return cli.Run(args, os.Stdin, stdout, stderr)
}
