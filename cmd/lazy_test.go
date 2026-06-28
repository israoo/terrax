package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestLazyCmd_IsRegistered verifies that the lazy subcommand is registered on the root command.
func TestLazyCmd_IsRegistered(t *testing.T) {
	found := false
	for _, sub := range rootCmd.Commands() {
		if sub.Use == "lazy" {
			found = true
			break
		}
	}
	assert.True(t, found, "lazy subcommand should be registered")
}
