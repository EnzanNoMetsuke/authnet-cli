package main

import (
	"fmt"
	"os"

	"github.com/lynyx/authnet-cli/internal/cli"
)

func main() {
	if err := cli.NewRootCommand(cli.BuildInfo{}).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
