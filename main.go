// Package main is the entry point for the TerraX CLI application.
//
// It initializes the root command and executes it, handling any top-level errors
// that may occur during the process.
package main

import (
	"fmt"
	"os"

	"github.com/israoo/terrax/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
