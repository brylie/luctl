package cmd

import (
	"fmt"

	"github.com/brylie/luctl/internal/project"
	"github.com/spf13/cobra"
)

func newPkgDisableCmd() *cobra.Command {
	var worldDir string

	cmd := &cobra.Command{
		Use:   "disable <mod_name>",
		Short: "Disable a mod in world.mt (sets load_mod_<name> = false).",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			modName := args[0]

			dir, err := resolveWorldDir(worldDir)
			if err != nil {
				return err
			}

			if err := project.SetModEnabled(dir, modName, false); err != nil {
				return fmt.Errorf("disabling mod: %w", err)
			}

			fmt.Printf("Disabled %s in world.mt\n", modName)

			return nil
		},
	}

	cmd.Flags().StringVar(&worldDir, "world-dir", "", "Path to world directory (default: from luanti.toml)")

	return cmd
}
