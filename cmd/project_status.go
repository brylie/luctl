package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/brylie/luctl/internal/project"
	"github.com/spf13/cobra"
)

const pkgStatusFmt = "%-35s %-8s %s\n"

func newProjectStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show which packages declared in luanti.toml are installed.",
		RunE: func(_ *cobra.Command, _ []string) error {
			p, err := project.LoadCurrent()
			if err != nil {
				return fmt.Errorf("loading project: %w", err)
			}

			entries := p.Packages.AllEntries()

			if len(entries) == 0 {
				fmt.Println("No packages declared in luanti.toml.")

				return nil
			}

			installed, missing := renderPackageStatus(p, entries)

			fmt.Printf("\n%d installed, %d missing\n", installed, missing)

			if missing > 0 {
				fmt.Println("Run `luctl project install` to install missing packages.")
			}

			return nil
		},
	}
}

func renderPackageStatus(p *project.Project, entries []project.PackageEntry) (installed, missing int) {
	fmt.Printf(pkgStatusFmt, "PACKAGE", "TYPE", "STATUS")
	fmt.Printf(pkgStatusFmt, "-------", "----", "------")

	for _, pkg := range entries {
		pkgType, name, dir := resolvePackage(pkg, p.Paths)

		if name == "" {
			fmt.Printf("%-35s %-8s invalid id\n", pkg.ID, pkgType)

			continue
		}

		status := "missing"

		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			status = "installed"
			installed++
		} else {
			missing++
		}

		fmt.Printf(pkgStatusFmt, pkg.ID, pkgType, status)
	}

	return installed, missing
}

// resolvePackage returns the effective type, mod name, and install directory for a package entry.
func resolvePackage(pkg project.PackageEntry, paths project.PathsConfig) (pkgType, name, dir string) {
	pkgType = pkg.Type
	if pkgType == "" {
		pkgType = "mod"
	}

	parts := strings.SplitN(pkg.ID, "/", 2)
	if len(parts) != 2 {
		return pkgType, "", ""
	}

	dir = paths.ModsDir
	if pkgType == "game" {
		dir = paths.GamesDir
	}

	return pkgType, parts[1], dir
}
