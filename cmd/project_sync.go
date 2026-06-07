package cmd

import (
	"errors"
	"fmt"

	"github.com/brylie/luctl/internal/project"
	"github.com/spf13/cobra"
)

func newProjectSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Apply [config] keys from luanti.toml to minetest.conf.",
		Long: `Reads every key under [config] in luanti.toml and writes it to the
server's minetest.conf (paths.conf_file). Existing lines are updated
in-place; unknown keys are appended. Lines beginning with # are never
modified. Run this after editing luanti.toml to push changes to the server.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			p, err := project.LoadCurrent()
			if err != nil {
				return fmt.Errorf("loading project: %w", err)
			}

			if len(p.Config) == 0 {
				fmt.Println("No [config] keys declared in luanti.toml.")

				return nil
			}

			if p.Paths.ConfFile == "" {
				return errors.New("paths.conf_file is not set in luanti.toml")
			}

			if err := project.SyncConfig(p.Paths.ConfFile, p.Config); err != nil {
				return fmt.Errorf("syncing config: %w", err)
			}

			fmt.Printf("Synced %d config key(s) to %s\n", len(p.Config), p.Paths.ConfFile)

			return nil
		},
	}
}
