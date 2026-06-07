// Package cmd implements the luctl CLI commands.
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:          "luctl",
		Short:        "luctl – Luanti server management CLI",
		Long:         "Manage your Luanti co-op server: install packages, control the server, and more.",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	root.AddCommand(newPackageCmd())
	root.AddCommand(newProjectCmd())
	root.AddCommand(newServerCmd())

	return root
}

// Execute is the entry point called from main.
func Execute() error {
	if err := newRootCmd().Execute(); err != nil {
		return fmt.Errorf("luctl: %w", err)
	}

	return nil
}
