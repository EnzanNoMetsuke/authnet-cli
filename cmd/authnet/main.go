// Package main provides the authnet executable entrypoint.
package main

import (
	"os"

	"github.com/lynyx/authnet-cli/internal/cli"
)

var (
	version        string
	commit         string
	date           string
	schemaVersion  string
	contractStatus string
)

func main() {
	exitCode := cli.Execute(cli.NewRootCommand(cli.BuildInfo{
		Version:        version,
		Commit:         commit,
		Date:           date,
		SchemaVersion:  schemaVersion,
		ContractStatus: contractStatus,
	}))
	if exitCode != 0 {
		os.Exit(int(exitCode))
	}
}
