package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/brylie/luctl/internal/contentdb"
	"github.com/spf13/cobra"
)

func newPkgUpdateCmd() *cobra.Command {
	var modsDir string

	cmd := &cobra.Command{
		Use:   "update [author/name]",
		Short: "Re-download the latest release of an installed mod.",
		Long:  "Re-downloads and reinstalls a mod from ContentDB.\nBulk update is not yet supported.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				fmt.Println("Specify a package to update: luctl package update <author/name>")

				return nil
			}

			parts := strings.SplitN(args[0], "/", 2)
			if len(parts) != 2 {
				return errors.New("argument must be in the form author/name")
			}

			author, name := parts[0], parts[1]

			dir := modsDir
			if dir == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("getting working directory: %w", err)
				}

				dir = filepath.Join(cwd, "mods")
			}

			existing := filepath.Join(dir, name)
			if _, err := os.Stat(existing); err == nil {
				fmt.Printf("Removing existing installation at %s ...\n", existing)

				if err := os.RemoveAll(existing); err != nil {
					return fmt.Errorf("removing existing mod: %w", err)
				}
			}

			fmt.Printf("Updating %s/%s ...\n", author, name)

			client := contentdb.New()

			dest, err := client.Install(cmd.Context(), author, name, dir)
			if err != nil {
				return fmt.Errorf("installing updated package: %w", err)
			}

			fmt.Printf("Updated: %s\n", dest)

			return nil
		},
	}

	cmd.Flags().StringVarP(&modsDir, "mods-dir", "d", "", "Path to mods directory (default: ./mods)")

	return cmd
}
