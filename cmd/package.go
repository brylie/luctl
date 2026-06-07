package cmd

import "github.com/spf13/cobra"

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
