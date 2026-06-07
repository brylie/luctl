package cmd

import (
	"fmt"

	"github.com/brylie/luctl/internal/project"
	"github.com/spf13/cobra"
)

func newPkgEnableCmd() *cobra.Command {
	var worldDir string

	cmd := &cobra.Command{
		Use:   "enable <mod_name>",
		Short: "Enable a mod in world.mt (sets load_mod_<name> = true).",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			modName := args[0]

			dir, err := resolveWorldDir(worldDir)
			if err != nil {
				return err
			}

			if err := project.SetModEnabled(dir, modName, true); err != nil {
				return fmt.Errorf("enabling mod: %w", err)
			}

			fmt.Printf("Enabled %s in world.mt\n", modName)

			return nil
		},
	}

	cmd.Flags().StringVar(&worldDir, "world-dir", "", "Path to world directory (default: from luanti.toml)")

	return cmd
}
