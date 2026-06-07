package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newPkgListCmd() *cobra.Command {
	var modsDir string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List locally installed mods.",
		RunE: func(_ *cobra.Command, _ []string) error {
			dir, err := defaultModsDir(modsDir)
			if err != nil {
				return err
			}

			entries, err := os.ReadDir(dir)
			if err != nil {
				return fmt.Errorf("reading mods directory: %w", err)
			}

			var mods []string
			for _, e := range entries {
				if e.IsDir() {
					mods = append(mods, e.Name())
				}
			}

			if len(mods) == 0 {
				fmt.Println("No mods installed.")

				return nil
			}

			fmt.Printf("Installed mods in %s:\n", dir)

			for _, m := range mods {
				fmt.Printf("  %s\n", m)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&modsDir, "mods-dir", "d", "", "Path to mods directory (default: ./mods)")

	return cmd
}
