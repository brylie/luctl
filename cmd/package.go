package cmd

import (
	"github.com/brylie/luctl/internal/contentdb"
	"github.com/spf13/cobra"
)

// newClient is the factory used by all package commands to create a ContentDB
// client. Tests replace it to redirect requests to a local httptest.Server.
var newClient = contentdb.New //nolint:gochecknoglobals // replaced in tests

func newPackageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "package",
		Short: "Manage ContentDB packages (mods, games, texture packs).",
	}

	cmd.AddCommand(newPkgSearchCmd())
	cmd.AddCommand(newPkgInfoCmd())
	cmd.AddCommand(newPkgInstallCmd())
	cmd.AddCommand(newPkgEnableCmd())
	cmd.AddCommand(newPkgDisableCmd())
	cmd.AddCommand(newPkgListCmd())
	cmd.AddCommand(newPkgUpdateCmd())

	return cmd
}
