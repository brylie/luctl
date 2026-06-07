package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/brylie/luctl/internal/project"
)

// ---------------------------------------------------------------------------
// resolvePackage
// ---------------------------------------------------------------------------

func TestResolvePackage_Mod(t *testing.T) {
	pkg := project.PackageEntry{ID: "alice/mod1", Type: "mod"}
	paths := project.PathsConfig{ModsDir: "./mods", GamesDir: "./games"}

	pkgType, name, dir := resolvePackage(pkg, paths)

	if pkgType != "mod" {
		t.Errorf("pkgType: want mod, got %q", pkgType)
	}
	if name != "mod1" {
		t.Errorf("name: want mod1, got %q", name)
	}
	if dir != "./mods" {
		t.Errorf("dir: want ./mods, got %q", dir)
	}
}

func TestResolvePackage_Game(t *testing.T) {
	pkg := project.PackageEntry{ID: "devteam/mygame", Type: "game"}
	paths := project.PathsConfig{ModsDir: "./mods", GamesDir: "./games"}

	pkgType, name, dir := resolvePackage(pkg, paths)

	if pkgType != "game" {
		t.Errorf("pkgType: want game, got %q", pkgType)
	}
	if name != "mygame" {
		t.Errorf("name: want mygame, got %q", name)
	}
	if dir != "./games" {
		t.Errorf("dir: want ./games, got %q", dir)
	}
}

func TestResolvePackage_EmptyTypeDefaultsMod(t *testing.T) {
	pkg := project.PackageEntry{ID: "alice/mod1", Type: ""}
	paths := project.PathsConfig{ModsDir: "./mods", GamesDir: "./games"}

	pkgType, _, dir := resolvePackage(pkg, paths)

	if pkgType != "mod" {
		t.Errorf("empty type should default to mod, got %q", pkgType)
	}
	if dir != "./mods" {
		t.Errorf("empty type should use mods dir, got %q", dir)
	}
}

func TestResolvePackage_InvalidID(t *testing.T) {
	pkg := project.PackageEntry{ID: "noauthor", Type: "mod"}
	paths := project.PathsConfig{ModsDir: "./mods"}

	_, name, _ := resolvePackage(pkg, paths)

	if name != "" {
		t.Errorf("invalid id (no slash): name should be empty, got %q", name)
	}
}

// ---------------------------------------------------------------------------
// renderPackageStatus
// ---------------------------------------------------------------------------

func TestRenderPackageStatus_AllInstalled(t *testing.T) {
	dir := t.TempDir()

	// Create fake mod directories.
	for _, mod := range []string{"mod1", "mod2"} {
		if err := os.Mkdir(filepath.Join(dir, mod), 0o750); err != nil {
			t.Fatalf("Mkdir: %v", err)
		}
	}

	p := project.Default()
	p.Paths.ModsDir = dir

	entries := []project.PackageEntry{
		{ID: "alice/mod1", Type: "mod"},
		{ID: "bob/mod2", Type: "mod"},
	}

	installed, missing := renderPackageStatus(p, entries)

	if installed != 2 {
		t.Errorf("installed: want 2, got %d", installed)
	}
	if missing != 0 {
		t.Errorf("missing: want 0, got %d", missing)
	}
}

func TestRenderPackageStatus_AllMissing(t *testing.T) {
	dir := t.TempDir()

	p := project.Default()
	p.Paths.ModsDir = dir

	entries := []project.PackageEntry{
		{ID: "alice/mod1", Type: "mod"},
		{ID: "bob/mod2", Type: "mod"},
	}

	installed, missing := renderPackageStatus(p, entries)

	if installed != 0 {
		t.Errorf("installed: want 0, got %d", installed)
	}
	if missing != 2 {
		t.Errorf("missing: want 2, got %d", missing)
	}
}

func TestRenderPackageStatus_Mixed(t *testing.T) {
	dir := t.TempDir()

	// Only mod1 is installed.
	if err := os.Mkdir(filepath.Join(dir, "mod1"), 0o750); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	p := project.Default()
	p.Paths.ModsDir = dir

	entries := []project.PackageEntry{
		{ID: "alice/mod1", Type: "mod"},
		{ID: "bob/mod2", Type: "mod"},
	}

	installed, missing := renderPackageStatus(p, entries)

	if installed != 1 {
		t.Errorf("installed: want 1, got %d", installed)
	}
	if missing != 1 {
		t.Errorf("missing: want 1, got %d", missing)
	}
}

func TestRenderPackageStatus_InvalidID(t *testing.T) {
	p := project.Default()

	entries := []project.PackageEntry{
		{ID: "noslash", Type: "mod"},
	}

	installed, missing := renderPackageStatus(p, entries)

	// Invalid IDs are printed as "invalid id" but not counted in either tally.
	if installed != 0 || missing != 0 {
		t.Errorf("invalid id should not count as installed or missing, got installed=%d missing=%d", installed, missing)
	}
}

func TestRenderPackageStatus_GameEntry(t *testing.T) {
	gamesDir := t.TempDir()

	// Create fake game directory.
	if err := os.Mkdir(filepath.Join(gamesDir, "mygame"), 0o750); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	p := project.Default()
	p.Paths.GamesDir = gamesDir

	entries := []project.PackageEntry{
		{ID: "devteam/mygame", Type: "game"},
	}

	installed, missing := renderPackageStatus(p, entries)

	if installed != 1 {
		t.Errorf("installed: want 1, got %d", installed)
	}
	if missing != 0 {
		t.Errorf("missing: want 0, got %d", missing)
	}
}
