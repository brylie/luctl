package cmd

import (
	"fmt"
	"strings"

	"github.com/brylie/luctl/internal/project"
	"github.com/spf13/cobra"
)

func newProjectInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install all packages declared in luanti.toml.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := project.LoadCurrent()
			if err != nil {
				return fmt.Errorf("loading project: %w", err)
			}

			entries := p.Packages.AllEntries()

			if len(entries) == 0 {
				fmt.Println("No packages declared in luanti.toml.")

				return nil
			}

			failed := installPackages(cmd, p, entries)

			fmt.Printf("\n%d installed", len(entries)-len(failed))

			if len(failed) > 0 {
				return fmt.Errorf("%d package(s) failed to install: %s", len(failed), strings.Join(failed, ", "))
			}

			fmt.Println()

			return nil
		},
	}
}

func installPackages(cmd *cobra.Command, p *project.Project, entries []project.PackageEntry) []string {
	client := newClient()

	var failed []string

	for _, pkg := range entries {
		parts := strings.SplitN(pkg.ID, "/", 2)
		if len(parts) != 2 {
			fmt.Printf("  SKIP  %s  (invalid id, expected author/name)\n", pkg.ID)

			continue
		}

		author, name := parts[0], parts[1]

		destDir := p.Paths.ModsDir
		if pkg.Type == "game" {
			destDir = p.Paths.GamesDir
		}

		fmt.Printf("  installing %s ...\n", pkg.ID)

		dest, err := client.Install(cmd.Context(), author, name, destDir)
		if err != nil {
			fmt.Printf("  FAIL  %s: %v\n", pkg.ID, err)
			failed = append(failed, pkg.ID)

			continue
		}

		fmt.Printf("  OK    %s\n", pkg.ID)

		if pkg.Type != "game" && p.Paths.WorldDir != "" {
			enableInstalledMod(p.Paths.WorldDir, dest, name)
		}
	}

	return failed
}
