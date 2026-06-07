package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/brylie/luctl/internal/contentdb"
	"github.com/brylie/luctl/internal/project"
	"github.com/spf13/cobra"
)

func newPkgInstallCmd() *cobra.Command {
	var modsDir string
	var noSave bool
	var noEnable bool
	var pkgType string

	cmd := &cobra.Command{
		Use:   "install <author/name>",
		Short: "Download and install a package into the mods directory.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			author, name, err := splitAuthorName(args[0])
			if err != nil {
				return err
			}

			proj, _ := project.LoadCurrent()
			dir := resolveInstallDir(modsDir, pkgType, proj)
			client := contentdb.New()

			pkg, err := client.GetPackage(cmd.Context(), author, name)
			if err != nil {
				return fmt.Errorf("fetching package metadata: %w", err)
			}

			installRequiredDeps(cmd, client, pkg, dir, proj, noSave, noEnable)

			fmt.Printf("Installing %s/%s into %s ...\n", author, name, dir)

			dest, err := client.Install(cmd.Context(), author, name, dir)
			if err != nil {
				return fmt.Errorf("installing package: %w", err)
			}

			fmt.Printf("Installed: %s\n", dest)

			return saveToManifest(proj, noSave, noEnable, author, name, pkgType, dest)
		},
	}

	cmd.Flags().StringVarP(&modsDir, "mods-dir", "d", "", "Path to mods directory (default: from luanti.toml or ./mods)")
	cmd.Flags().BoolVar(&noSave, "no-save", false, "Skip saving the package to luanti.toml")
	cmd.Flags().BoolVar(&noEnable, "no-enable", false, "Skip enabling the mod in world.mt")
	cmd.Flags().StringVar(&pkgType, "type", "mod", `Package type: "mod", "game", or "txp"`)

	return cmd
}

// installRequiredDeps fetches the transitive dependency graph for pkg and
// installs any required packages not already present in dir.
func installRequiredDeps(
	cmd *cobra.Command, client *contentdb.Client, pkg *contentdb.Package,
	dir string, proj *project.Project, noSave, noEnable bool,
) {
	depsResp, err := client.GetDependencies(cmd.Context(), pkg.Author, pkg.Name)
	if err != nil {
		fmt.Printf("Warning: could not fetch dependencies for %s/%s: %v\n", pkg.Author, pkg.Name, err)
		return
	}

	provided := make(map[string]bool, len(pkg.Provides))
	for _, p := range pkg.Provides {
		provided[p] = true
	}

	mainID := pkg.Author + "/" + pkg.Name
	needed := collectRequiredDeps(depsResp, mainID, provided)
	installDepsFromList(cmd, client, needed, dir, proj, noSave, noEnable)
}

// collectRequiredDeps does a BFS over the dependency graph and returns the
// ordered list of "author/name" package IDs that must be installed.
// mainID itself is excluded; names already in provided are skipped.
func collectRequiredDeps(depsResp contentdb.DependenciesResponse, mainID string, provided map[string]bool) []string {
	queue := []string{mainID}
	visited := map[string]bool{mainID: true}

	var needed []string

	for len(queue) > 0 {
		pkgID := queue[0]
		queue = queue[1:]

		for _, dep := range depsResp[pkgID] {
			providerID := resolveDepProvider(dep, provided)
			if providerID == "" || visited[providerID] {
				continue
			}

			visited[providerID] = true
			needed = append(needed, providerID)

			if _, hasDeps := depsResp[providerID]; hasDeps {
				queue = append(queue, providerID)
			}
		}
	}

	return needed
}

// resolveDepProvider picks the best ContentDB package ID that satisfies dep.
// Returns "" when the dep is optional, has no candidates, or is fully provided
// by the modpack being installed.
func resolveDepProvider(dep contentdb.Dependency, provided map[string]bool) string {
	if dep.IsOptional || len(dep.Packages) == 0 {
		return ""
	}

	// Prefer the candidate whose package-name matches dep.Name exactly.
	for _, pkgID := range dep.Packages {
		if strings.HasSuffix(pkgID, "/"+dep.Name) && !depNameProvided(pkgID, provided) {
			return pkgID
		}
	}

	// Fall back to the first non-provided candidate.
	for _, pkgID := range dep.Packages {
		if !depNameProvided(pkgID, provided) {
			return pkgID
		}
	}

	return ""
}

func depNameProvided(pkgID string, provided map[string]bool) bool {
	_, name, err := splitAuthorName(pkgID)
	return err == nil && provided[name]
}

// installDepsFromList downloads and wires each package ID that is not yet on disk.
func installDepsFromList(
	cmd *cobra.Command, client *contentdb.Client, needed []string,
	dir string, proj *project.Project, noSave, noEnable bool,
) {
	for _, depID := range needed {
		depAuthor, depName, err := splitAuthorName(depID)
		if err != nil {
			continue
		}

		if isModInstalled(dir, depName) {
			fmt.Printf("Dependency %s already installed.\n", depID)
			continue
		}

		fmt.Printf("Installing dependency %s into %s ...\n", depID, dir)

		dest, err := client.Install(cmd.Context(), depAuthor, depName, dir)
		if err != nil {
			fmt.Printf("Warning: could not install dependency %s: %v\n", depID, err)
			continue
		}

		fmt.Printf("Installed: %s\n", dest)

		if err := saveToManifest(proj, noSave, noEnable, depAuthor, depName, "mod", dest); err != nil {
			fmt.Printf("Warning: could not save dependency %s to manifest: %v\n", depID, err)
		}
	}
}

// isModInstalled reports whether a mod directory already exists under modsDir.
func isModInstalled(modsDir, name string) bool {
	_, err := os.Stat(filepath.Join(modsDir, name))
	return err == nil
}

// saveToManifest records the package in luanti.toml and enables it in world.mt.
// destPath is the directory where the package was extracted (used for modpack detection).
func saveToManifest(proj *project.Project, noSave, noEnable bool, author, name, pkgType, destPath string) error {
	if proj != nil && !noSave {
		id := author + "/" + name

		if project.AddPackage(proj, id, pkgType) {
			if err := project.Save(proj, project.Filename); err != nil {
				return fmt.Errorf("saving project manifest: %w", err)
			}

			fmt.Printf("\nAdded %s to luanti.toml\n", id)
		} else {
			fmt.Printf("\n%s is already declared in luanti.toml\n", id)
		}
	}

	if pkgType == "mod" && !noEnable && proj != nil && proj.Paths.WorldDir != "" {
		enableInstalledMod(proj.Paths.WorldDir, destPath, name)
	}

	return nil
}

// enableInstalledMod enables a mod in world.mt.  For modpacks (directories that
// contain modpack.conf) it enables every sub-mod that has an init.lua instead.
func enableInstalledMod(worldDir, destPath, name string) {
	if _, err := os.Stat(filepath.Join(destPath, "modpack.conf")); err == nil {
		enableModpackSubMods(worldDir, destPath)
		return
	}

	if err := project.SetModEnabled(worldDir, name, true); err != nil {
		// Non-fatal: world.mt may not exist yet (world not yet generated).
		fmt.Printf("Note: could not update world.mt for %s: %v\n", name, err)
	} else {
		fmt.Printf("Updated world.mt: load_mod_%s = true\n", name)
	}
}

// enableModpackSubMods iterates a modpack directory and enables every
// sub-directory that contains an init.lua as an individual mod.
func enableModpackSubMods(worldDir, modpackDir string) {
	entries, err := os.ReadDir(modpackDir)
	if err != nil {
		fmt.Printf("Note: could not read modpack directory %s: %v\n", modpackDir, err)
		return
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		subName := e.Name()
		if _, err := os.Stat(filepath.Join(modpackDir, subName, "init.lua")); err != nil {
			continue
		}

		if err := project.SetModEnabled(worldDir, subName, true); err != nil {
			fmt.Printf("Note: could not update world.mt for %s: %v\n", subName, err)
		} else {
			fmt.Printf("Updated world.mt: load_mod_%s = true\n", subName)
		}
	}
}

func splitAuthorName(arg string) (author, name string, err error) {
	parts := strings.SplitN(arg, "/", 2)

	if len(parts) != 2 {
		return "", "", errors.New("argument must be in the form author/name")
	}

	return parts[0], parts[1], nil
}

func resolveInstallDir(modsDir, pkgType string, proj *project.Project) string {
	if modsDir != "" {
		return modsDir
	}

	if proj != nil {
		if pkgType == "game" {
			return proj.Paths.GamesDir
		}

		return proj.Paths.ModsDir
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "mods"
	}

	return filepath.Join(cwd, "mods")
}

// resolveWorldDir returns the world directory from the flag or from luanti.toml.
func resolveWorldDir(flagVal string) (string, error) {
	if flagVal != "" {
		return flagVal, nil
	}

	proj, err := project.LoadCurrent()
	if err != nil {
		return "", fmt.Errorf("no --world-dir given and %w", err)
	}

	if proj.Paths.WorldDir == "" {
		return "", errors.New("world_dir is not set in luanti.toml [paths]")
	}

	return proj.Paths.WorldDir, nil
}
