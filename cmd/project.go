package cmd

import "github.com/spf13/cobra"

func newProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage a luanti.toml server project manifest.",
	}

	cmd.AddCommand(newProjectInitCmd())
	cmd.AddCommand(newProjectInstallCmd())
	cmd.AddCommand(newProjectStatusCmd())
	cmd.AddCommand(newProjectFmtCmd())
	cmd.AddCommand(newProjectSyncCmd())

	return cmd
}
