package cmd

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brylie/luctl/internal/contentdb"
	"github.com/brylie/luctl/internal/project"
	"github.com/spf13/cobra"
)

// buildCmdTestZip builds a minimal in-memory zip with ContentDB top-dir layout.
func buildCmdTestZip(t *testing.T, modName string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, name := range []string{modName + "-v1.0/init.lua", modName + "-v1.0/README.md"} {
		f, err := w.Create(name)
		if err != nil {
			t.Fatalf("zip.Create(%q): %v", name, err)
		}
		if _, err := f.Write([]byte("-- content")); err != nil {
			t.Fatalf("zip write: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("zip.Close: %v", err)
	}

	return buf.Bytes()
}

// newCmdWithContext creates a bare cobra.Command suitable for passing to
// functions that call cmd.Context() (e.g. installRequiredDeps).
func newCmdWithContext() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	return cmd
}

// ---------------------------------------------------------------------------
// splitAuthorName
// ---------------------------------------------------------------------------

func TestSplitAuthorName_Valid(t *testing.T) {
	author, name, err := splitAuthorName("alice/mod1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if author != "alice" {
		t.Errorf("author: want alice, got %q", author)
	}
	if name != "mod1" {
		t.Errorf("name: want mod1, got %q", name)
	}
}

func TestSplitAuthorName_NoSlash(t *testing.T) {
	_, _, err := splitAuthorName("justname")
	if err == nil {
		t.Error("expected error for input without slash, got nil")
	}
}

func TestSplitAuthorName_TwoSlashes(t *testing.T) {
	// SplitN(..., 2) means everything after the first slash is the name.
	author, name, err := splitAuthorName("alice/mod1/extra")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if author != "alice" {
		t.Errorf("author: want alice, got %q", author)
	}
	if name != "mod1/extra" {
		t.Errorf("name: want mod1/extra, got %q", name)
	}
}

func TestSplitAuthorName_Empty(t *testing.T) {
	_, _, err := splitAuthorName("")
	if err == nil {
		t.Error("expected error for empty input, got nil")
	}
}

// ---------------------------------------------------------------------------
// resolveInstallDir
// ---------------------------------------------------------------------------

func TestResolveInstallDir_FlagWins(t *testing.T) {
	p := project.Default()
	p.Paths.ModsDir = "./project-mods"

	got := resolveInstallDir("/explicit/path", "mod", p)
	if got != "/explicit/path" {
		t.Errorf("flag should win, got %q", got)
	}
}

func TestResolveInstallDir_ModFromProject(t *testing.T) {
	p := project.Default()
	p.Paths.ModsDir = "./custom-mods"

	got := resolveInstallDir("", "mod", p)
	if got != "./custom-mods" {
		t.Errorf("want ./custom-mods, got %q", got)
	}
}

func TestResolveInstallDir_GameFromProject(t *testing.T) {
	p := project.Default()
	p.Paths.GamesDir = "./custom-games"

	got := resolveInstallDir("", "game", p)
	if got != "./custom-games" {
		t.Errorf("want ./custom-games, got %q", got)
	}
}

func TestResolveInstallDir_NilProjFallback(t *testing.T) {
	cwd, _ := os.Getwd()
	got := resolveInstallDir("", "mod", nil)
	want := filepath.Join(cwd, "mods")
	if got != want {
		t.Errorf("nil proj: want %q, got %q", want, got)
	}
}

// ---------------------------------------------------------------------------
// isModInstalled
// ---------------------------------------------------------------------------

func TestIsModInstalled_Exists(t *testing.T) {
	dir := t.TempDir()
	modDir := filepath.Join(dir, "mymod")
	if err := os.Mkdir(modDir, 0o750); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if !isModInstalled(dir, "mymod") {
		t.Error("expected isModInstalled to return true for existing directory")
	}
}

func TestIsModInstalled_Missing(t *testing.T) {
	dir := t.TempDir()
	if isModInstalled(dir, "nonexistent") {
		t.Error("expected isModInstalled to return false for missing directory")
	}
}

// ---------------------------------------------------------------------------
// resolveDepProvider
// ---------------------------------------------------------------------------

func TestResolveDepProvider_Optional(t *testing.T) {
	dep := contentdb.Dependency{
		IsOptional: true,
		Name:       "default",
		Packages:   []string{"mt-mods/default"},
	}
	got := resolveDepProvider(dep, map[string]bool{})
	if got != "" {
		t.Errorf("optional dep should return empty, got %q", got)
	}
}

func TestResolveDepProvider_NoPackages(t *testing.T) {
	dep := contentdb.Dependency{
		IsOptional: false,
		Name:       "default",
		Packages:   []string{},
	}
	got := resolveDepProvider(dep, map[string]bool{})
	if got != "" {
		t.Errorf("no packages should return empty, got %q", got)
	}
}

func TestResolveDepProvider_ExactNamePreferred(t *testing.T) {
	dep := contentdb.Dependency{
		IsOptional: false,
		Name:       "default",
		Packages:   []string{"other/somemod", "mt-mods/default"},
	}
	got := resolveDepProvider(dep, map[string]bool{})
	if got != "mt-mods/default" {
		t.Errorf("exact name match should be preferred, got %q", got)
	}
}

func TestResolveDepProvider_FallbackToFirst(t *testing.T) {
	dep := contentdb.Dependency{
		IsOptional: false,
		Name:       "specialmod",
		Packages:   []string{"alice/modpack", "bob/other"},
	}
	got := resolveDepProvider(dep, map[string]bool{})
	if got != "alice/modpack" {
		t.Errorf("should fall back to first non-provided, got %q", got)
	}
}

func TestResolveDepProvider_AllProvided(t *testing.T) {
	dep := contentdb.Dependency{
		IsOptional: false,
		Name:       "default",
		Packages:   []string{"mt-mods/default"},
	}
	// "default" is already provided by the modpack being installed.
	provided := map[string]bool{"default": true}
	got := resolveDepProvider(dep, provided)
	if got != "" {
		t.Errorf("all provided should return empty, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// collectRequiredDeps
// ---------------------------------------------------------------------------

func TestCollectRequiredDeps_NoDeps(t *testing.T) {
	depsResp := contentdb.DependenciesResponse{
		"alice/mod1": {},
	}
	needed := collectRequiredDeps(depsResp, "alice/mod1", map[string]bool{})
	if len(needed) != 0 {
		t.Errorf("no deps: want empty, got %v", needed)
	}
}

func TestCollectRequiredDeps_DirectDep(t *testing.T) {
	depsResp := contentdb.DependenciesResponse{
		"alice/mod1": {
			{IsOptional: false, Name: "default", Packages: []string{"mt-mods/default"}},
		},
		"mt-mods/default": {},
	}
	needed := collectRequiredDeps(depsResp, "alice/mod1", map[string]bool{})
	if len(needed) != 1 || needed[0] != "mt-mods/default" {
		t.Errorf("want [mt-mods/default], got %v", needed)
	}
}

func TestCollectRequiredDeps_TransitiveDep(t *testing.T) {
	depsResp := contentdb.DependenciesResponse{
		"alice/mod1": {
			{IsOptional: false, Name: "base", Packages: []string{"mt-mods/base"}},
		},
		"mt-mods/base": {
			{IsOptional: false, Name: "core", Packages: []string{"mt-mods/core"}},
		},
		"mt-mods/core": {},
	}
	needed := collectRequiredDeps(depsResp, "alice/mod1", map[string]bool{})
	if len(needed) != 2 {
		t.Fatalf("want 2 transitive deps, got %v", needed)
	}
}

func TestCollectRequiredDeps_OptionalSkipped(t *testing.T) {
	depsResp := contentdb.DependenciesResponse{
		"alice/mod1": {
			{IsOptional: true, Name: "optmod", Packages: []string{"someone/optmod"}},
		},
	}
	needed := collectRequiredDeps(depsResp, "alice/mod1", map[string]bool{})
	if len(needed) != 0 {
		t.Errorf("optional dep should be skipped, got %v", needed)
	}
}

func TestCollectRequiredDeps_NoCycleOnVisited(t *testing.T) {
	// Circular dependency: A → B → A
	depsResp := contentdb.DependenciesResponse{
		"alice/a": {
			{IsOptional: false, Name: "b", Packages: []string{"alice/b"}},
		},
		"alice/b": {
			{IsOptional: false, Name: "a", Packages: []string{"alice/a"}},
		},
	}
	// Should not loop forever; alice/a is the main ID and already visited.
	needed := collectRequiredDeps(depsResp, "alice/a", map[string]bool{})
	if len(needed) != 1 || needed[0] != "alice/b" {
		t.Errorf("cycle: want [alice/b], got %v", needed)
	}
}

func TestCollectRequiredDeps_ProvidedSkipped(t *testing.T) {
	// dep.Name is already provided by the modpack.
	depsResp := contentdb.DependenciesResponse{
		"alice/modpack": {
			{IsOptional: false, Name: "submod", Packages: []string{"alice/submod"}},
		},
	}
	provided := map[string]bool{"submod": true}
	needed := collectRequiredDeps(depsResp, "alice/modpack", provided)
	if len(needed) != 0 {
		t.Errorf("provided name should be skipped, got %v", needed)
	}
}

// ---------------------------------------------------------------------------
// saveToManifest
// ---------------------------------------------------------------------------

func TestSaveToManifest_NoSaveFlag(t *testing.T) {
	p := project.Default()
	err := saveToManifest(p, true, true, "alice", "mod1", "mod", t.TempDir())
	if err != nil {
		t.Fatalf("saveToManifest with noSave=true: %v", err)
	}
	if len(p.Packages.Mods) != 0 {
		t.Error("noSave=true should not modify the manifest")
	}
}

func TestSaveToManifest_NilProject(t *testing.T) {
	// nil proj should be a no-op and not panic.
	err := saveToManifest(nil, false, false, "alice", "mod1", "mod", t.TempDir())
	if err != nil {
		t.Fatalf("saveToManifest with nil project: %v", err)
	}
}

func TestSaveToManifest_SavesTOML(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	p := project.Default()
	// Clear WorldDir so enableInstalledMod is not called.
	p.Paths.WorldDir = ""

	if err := saveToManifest(p, false, false, "alice", "mod1", "mod", dir); err != nil {
		t.Fatalf("saveToManifest: %v", err)
	}

	loaded, err := project.Load(filepath.Join(dir, project.Filename))
	if err != nil {
		t.Fatalf("Load after saveToManifest: %v", err)
	}
	if len(loaded.Packages.Mods) != 1 || loaded.Packages.Mods[0] != "alice/mod1" {
		t.Errorf("expected alice/mod1 in mods, got %v", loaded.Packages.Mods)
	}
}

func TestSaveToManifest_DuplicateSkipped(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	p := project.Default()
	p.Packages.Mods = []string{"alice/mod1"}
	p.Paths.WorldDir = ""
	if err := project.Save(p, filepath.Join(dir, project.Filename)); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Should silently skip (already declared) without error.
	if err := saveToManifest(p, false, false, "alice", "mod1", "mod", dir); err != nil {
		t.Fatalf("saveToManifest for duplicate: %v", err)
	}
}

func TestSaveToManifest_WithEnableModInWorldMT(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	worldDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(worldDir, "world.mt"), []byte(""), 0o600); err != nil {
		t.Fatalf("setup world.mt: %v", err)
	}

	p := project.Default()
	p.Paths.WorldDir = worldDir

	destDir := t.TempDir() // no modpack.conf → treated as regular mod

	if err := saveToManifest(p, false, false, "alice", "mod1", "mod", destDir); err != nil {
		t.Fatalf("saveToManifest: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(worldDir, "world.mt"))
	if !strings.Contains(string(data), "load_mod_mod1 = true") {
		t.Errorf("expected load_mod_mod1 = true in world.mt, got:\n%s", data)
	}
}

// ---------------------------------------------------------------------------
// enableInstalledMod / enableModpackSubMods
// ---------------------------------------------------------------------------

func TestEnableInstalledMod_RegularMod(t *testing.T) {
	worldDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(worldDir, "world.mt"), []byte(""), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}
	destDir := t.TempDir() // no modpack.conf

	enableInstalledMod(worldDir, destDir, "mymod")

	data, _ := os.ReadFile(filepath.Join(worldDir, "world.mt"))
	if !strings.Contains(string(data), "load_mod_mymod = true") {
		t.Errorf("expected load_mod_mymod = true, got:\n%s", data)
	}
}

func TestEnableInstalledMod_Modpack(t *testing.T) {
	worldDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(worldDir, "world.mt"), []byte(""), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	modpackDir := t.TempDir()
	// Mark as modpack.
	if err := os.WriteFile(filepath.Join(modpackDir, "modpack.conf"), []byte(""), 0o600); err != nil {
		t.Fatalf("setup modpack.conf: %v", err)
	}
	// Sub-mod with init.lua.
	subMod := filepath.Join(modpackDir, "submod1")
	if err := os.Mkdir(subMod, 0o750); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subMod, "init.lua"), []byte("-- sub"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	enableInstalledMod(worldDir, modpackDir, "mymodpack")

	data, _ := os.ReadFile(filepath.Join(worldDir, "world.mt"))
	if !strings.Contains(string(data), "load_mod_submod1 = true") {
		t.Errorf("expected load_mod_submod1 = true, got:\n%s", data)
	}
}

func TestEnableModpackSubMods_SkipsNoInitLua(t *testing.T) {
	worldDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(worldDir, "world.mt"), []byte(""), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	modpackDir := t.TempDir()
	// Directory without init.lua — should be skipped.
	if err := os.Mkdir(filepath.Join(modpackDir, "notamod"), 0o750); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	// Directory with init.lua — should be enabled.
	realMod := filepath.Join(modpackDir, "realmod")
	if err := os.Mkdir(realMod, 0o750); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(realMod, "init.lua"), []byte("-- real"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	enableModpackSubMods(worldDir, modpackDir)

	data, _ := os.ReadFile(filepath.Join(worldDir, "world.mt"))
	content := string(data)
	if strings.Contains(content, "load_mod_notamod") {
		t.Error("notamod (no init.lua) should not appear in world.mt")
	}
	if !strings.Contains(content, "load_mod_realmod = true") {
		t.Errorf("realmod should be enabled, got:\n%s", content)
	}
}

// ---------------------------------------------------------------------------
// resolveWorldDir
// ---------------------------------------------------------------------------

func TestResolveWorldDir_FlagProvided(t *testing.T) {
	dir, err := resolveWorldDir("/explicit/world/dir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dir != "/explicit/world/dir" {
		t.Errorf("want /explicit/world/dir, got %q", dir)
	}
}

func TestResolveWorldDir_NoFlagNoProject(t *testing.T) {
	tmpDir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	_, err := resolveWorldDir("")
	if err == nil {
		t.Error("expected error when no flag and no luanti.toml, got nil")
	}
}

func TestResolveWorldDir_FromProject(t *testing.T) {
	tmpDir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	p := project.Default()
	p.Paths.WorldDir = "/data/worlds/myworld"
	if err := project.Save(p, filepath.Join(tmpDir, project.Filename)); err != nil {
		t.Fatalf("Save: %v", err)
	}

	dir, err := resolveWorldDir("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dir != "/data/worlds/myworld" {
		t.Errorf("want /data/worlds/myworld, got %q", dir)
	}
}

func TestResolveWorldDir_EmptyWorldDirInProject(t *testing.T) {
	tmpDir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	p := project.Default()
	p.Paths.WorldDir = "" // explicitly empty
	if err := project.Save(p, filepath.Join(tmpDir, project.Filename)); err != nil {
		t.Fatalf("Save: %v", err)
	}

	_, err := resolveWorldDir("")
	if err == nil {
		t.Error("expected error when world_dir is empty in project, got nil")
	}
}

// ---------------------------------------------------------------------------
// installRequiredDeps
// ---------------------------------------------------------------------------

func TestInstallRequiredDeps_NoDeps(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(contentdb.DependenciesResponse{})
	}))
	defer ts.Close()

	client := contentdb.NewWithClient(ts.URL, ts.Client())
	pkg := &contentdb.Package{Author: "alice", Name: "mod1"}

	// Should complete without error — no deps to install.
	installRequiredDeps(newCmdWithContext(), client, pkg, t.TempDir(), nil, true, true)
}

func TestInstallRequiredDeps_FetchError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := contentdb.NewWithClient(ts.URL, ts.Client())
	pkg := &contentdb.Package{Author: "alice", Name: "mod1"}

	// Fetch error should be reported as warning, not panic.
	installRequiredDeps(newCmdWithContext(), client, pkg, t.TempDir(), nil, true, true)
}

func TestInstallRequiredDeps_WithDep(t *testing.T) {
	depZip := buildCmdTestZip(t, "dep1")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/dependencies/"):
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(contentdb.DependenciesResponse{
				"alice/mod1": {
					{IsOptional: false, Name: "dep1", Packages: []string{"bob/dep1"}},
				},
				"bob/dep1": {},
			})
		default:
			// Package download.
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(depZip)
		}
	}))
	defer ts.Close()

	client := contentdb.NewWithClient(ts.URL, ts.Client())
	pkg := &contentdb.Package{Author: "alice", Name: "mod1"}

	dir := t.TempDir()
	installRequiredDeps(newCmdWithContext(), client, pkg, dir, nil, true, true)

	// dep1 should have been extracted.
	if _, err := os.Stat(filepath.Join(dir, "dep1")); err != nil {
		t.Errorf("dep1 should be installed after installRequiredDeps: %v", err)
	}
}

// ---------------------------------------------------------------------------
// installDepsFromList
// ---------------------------------------------------------------------------

func TestInstallDepsFromList_AlreadyInstalled(t *testing.T) {
	dir := t.TempDir()
	// Pre-create the mod directory so it looks already installed.
	if err := os.Mkdir(filepath.Join(dir, "dep1"), 0o750); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	client := contentdb.New()
	// Should print "already installed" and skip — no network call needed.
	installDepsFromList(newCmdWithContext(), client, []string{"alice/dep1"}, dir, nil, true, true)
}

func TestInstallDepsFromList_InstallsDep(t *testing.T) {
	depZip := buildCmdTestZip(t, "dep1")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(depZip)
	}))
	defer ts.Close()

	client := contentdb.NewWithClient(ts.URL, ts.Client())
	dir := t.TempDir()

	installDepsFromList(newCmdWithContext(), client, []string{"alice/dep1"}, dir, nil, true, true)

	if _, err := os.Stat(filepath.Join(dir, "dep1")); err != nil {
		t.Errorf("dep1 should be installed: %v", err)
	}
}

func TestInstallDepsFromList_InvalidID(t *testing.T) {
	client := contentdb.New()
	// Invalid ID (no slash) should be skipped, not panic.
	installDepsFromList(newCmdWithContext(), client, []string{"noslash"}, t.TempDir(), nil, true, true)
}

func TestInstallDepsFromList_InstallFails(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	client := contentdb.NewWithClient(ts.URL, ts.Client())
	// Install failure should print a warning, not error out.
	installDepsFromList(newCmdWithContext(), client, []string{"alice/dep1"}, t.TempDir(), nil, true, true)
}
