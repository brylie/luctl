package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/brylie/luctl/internal/project"
	"github.com/spf13/cobra"
)

func newProjectInitCmd() *cobra.Command {
	var serverName string
	var adminName string
	var gameName string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a luanti.toml project manifest in the current directory.",
		RunE: func(_ *cobra.Command, _ []string) error {
			if _, err := os.Stat(project.Filename); err == nil {
				return errors.New("luanti.toml already exists in this directory")
			}

			p := project.Default()

			if serverName != "" {
				p.Server.Name = serverName
			}

			if adminName != "" {
				p.Server.Admins = []string{adminName}
			}

			if gameName != "" {
				p.World.Game = gameName
			}

			if err := project.Save(p, project.Filename); err != nil {
				return fmt.Errorf("writing project file: %w", err)
			}

			fmt.Printf("Created %s\n", project.Filename)
			fmt.Println("Edit it to declare your packages, then run: luctl project install")

			return nil
		},
	}

	cmd.Flags().StringVar(&serverName, "name", "", "Server name")
	cmd.Flags().StringVar(&adminName, "admin", "", "Admin username")
	cmd.Flags().StringVar(&gameName, "game", "", "Base game (default: minetest_game)")

	return cmd
}
