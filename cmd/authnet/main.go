// Package main provides the authnet executable entrypoint.
package main

import (
	"os"

	"github.com/lynyx/authnet-cli/internal/cli"
)

func main() {
	exitCode := cli.Execute(cli.NewRootCommand(cli.BuildInfo{}))
	if exitCode != 0 {
		os.Exit(int(exitCode))
	}
}
