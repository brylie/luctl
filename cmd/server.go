package cmd

import "github.com/spf13/cobra"

func newServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Manage the Luanti server (backup, restore).",
	}

	cmd.AddCommand(newServerBackupCmd())
	cmd.AddCommand(newServerRestoreCmd())

	return cmd
}
