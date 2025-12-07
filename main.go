package main

import (
	"fmt"
	"os"

	"github.com/israoo/terrax/cmd"
)

func main() {
	// Set the version in the cmd package (injected by GoReleaser via ldflags)
	cmd.Version = version

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
