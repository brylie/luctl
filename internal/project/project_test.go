package project

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Default
// ---------------------------------------------------------------------------

func TestDefault(t *testing.T) {
	p := Default()

	if p.Server.Port != 30000 {
		t.Errorf("port: want 30000, got %d", p.Server.Port)
	}
	if p.Server.Name != "My Luanti Server" {
		t.Errorf("server.name: want %q, got %q", "My Luanti Server", p.Server.Name)
	}
	if p.World.Game != "minetest_game" {
		t.Errorf("world.game: want %q, got %q", "minetest_game", p.World.Game)
	}
	if p.World.Mapgen != "v7" {
		t.Errorf("world.mapgen: want %q, got %q", "v7", p.World.Mapgen)
	}
	if p.Paths.ModsDir != "./mods" {
		t.Errorf("paths.mods_dir: want %q, got %q", "./mods", p.Paths.ModsDir)
	}
	if p.Paths.GamesDir != "./games" {
		t.Errorf("paths.games_dir: want %q, got %q", "./games", p.Paths.GamesDir)
	}
	if p.Paths.WorldDir != "./data/worlds/world" {
		t.Errorf("paths.world_dir: want %q, got %q", "./data/worlds/world", p.Paths.WorldDir)
	}
	if p.Config == nil {
		t.Error("config map should not be nil")
	}
	if len(p.Server.Admins) != 0 {
		t.Errorf("admins: want empty slice, got %v", p.Server.Admins)
	}
}

// ---------------------------------------------------------------------------
// AllEntries
// ---------------------------------------------------------------------------

func TestAllEntries_Empty(t *testing.T) {
	pc := PackagesConfig{}
	if got := pc.AllEntries(); len(got) != 0 {
		t.Errorf("want 0 entries, got %d", len(got))
	}
}

func TestAllEntries_ModsOnly(t *testing.T) {
	pc := PackagesConfig{Mods: []string{"alice/mod1", "bob/mod2"}}
	entries := pc.AllEntries()

	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}
	for _, e := range entries {
		if e.Type != "mod" {
			t.Errorf("expected type mod, got %q for id %q", e.Type, e.ID)
		}
	}
}

func TestAllEntries_GamesOnly(t *testing.T) {
	pc := PackagesConfig{Games: []string{"devteam/mygame"}}
	entries := pc.AllEntries()

	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	if entries[0].Type != "game" {
		t.Errorf("expected type game, got %q", entries[0].Type)
	}
	if entries[0].ID != "devteam/mygame" {
		t.Errorf("expected id devteam/mygame, got %q", entries[0].ID)
	}
}

func TestAllEntries_Mixed(t *testing.T) {
	pc := PackagesConfig{
		Mods:  []string{"alice/mod1"},
		Games: []string{"devteam/mygame"},
	}
	entries := pc.AllEntries()

	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}
	// Mods come first.
	if entries[0].Type != "mod" {
		t.Errorf("first entry should be mod, got %q", entries[0].Type)
	}
	if entries[1].Type != "game" {
		t.Errorf("second entry should be game, got %q", entries[1].Type)
	}
}

// ---------------------------------------------------------------------------
// AddPackage
// ---------------------------------------------------------------------------

func TestAddPackage_AddMod(t *testing.T) {
	p := Default()
	added := AddPackage(p, "alice/mod1", "mod")
	if !added {
		t.Error("expected AddPackage to return true for new mod")
	}
	if len(p.Packages.Mods) != 1 || p.Packages.Mods[0] != "alice/mod1" {
		t.Errorf("unexpected mods list: %v", p.Packages.Mods)
	}
}

func TestAddPackage_DuplicateMod(t *testing.T) {
	p := Default()
	AddPackage(p, "alice/mod1", "mod")
	added := AddPackage(p, "alice/mod1", "mod")
	if added {
		t.Error("expected AddPackage to return false for duplicate mod")
	}
	if len(p.Packages.Mods) != 1 {
		t.Errorf("expected 1 mod after duplicate add, got %d", len(p.Packages.Mods))
	}
}

func TestAddPackage_AddGame(t *testing.T) {
	p := Default()
	added := AddPackage(p, "devteam/mygame", "game")
	if !added {
		t.Error("expected AddPackage to return true for new game")
	}
	if len(p.Packages.Games) != 1 || p.Packages.Games[0] != "devteam/mygame" {
		t.Errorf("unexpected games list: %v", p.Packages.Games)
	}
}

func TestAddPackage_DuplicateGame(t *testing.T) {
	p := Default()
	AddPackage(p, "devteam/mygame", "game")
	added := AddPackage(p, "devteam/mygame", "game")
	if added {
		t.Error("expected AddPackage to return false for duplicate game")
	}
	if len(p.Packages.Games) != 1 {
		t.Errorf("expected 1 game after duplicate add, got %d", len(p.Packages.Games))
	}
}

func TestAddPackage_UnknownTypeGoesToMods(t *testing.T) {
	p := Default()
	AddPackage(p, "alice/txp", "txp")
	if len(p.Packages.Mods) != 1 {
		t.Errorf("unknown type should fall into mods, got mods=%v", p.Packages.Mods)
	}
}

// ---------------------------------------------------------------------------
// expandArrayLine
// ---------------------------------------------------------------------------

func TestExpandArrayLine_NotAnArray(t *testing.T) {
	line := `name = "my server"`
	got := expandArrayLine(line)
	if len(got) != 1 || got[0] != line {
		t.Errorf("non-array line should be returned unchanged, got %v", got)
	}
}

func TestExpandArrayLine_EmptyArray(t *testing.T) {
	line := `admins = []`
	got := expandArrayLine(line)
	if len(got) != 1 || got[0] != line {
		t.Errorf("empty array should stay inline, got %v", got)
	}
}

func TestExpandArrayLine_SingleElement(t *testing.T) {
	line := `mods = ["alice/mod1"]`
	got := expandArrayLine(line)
	// Single element — no expansion.
	if len(got) != 1 || got[0] != line {
		t.Errorf("single-element array should stay inline, got %v", got)
	}
}

func TestExpandArrayLine_MultiElement(t *testing.T) {
	line := `mods = ["alice/mod1", "bob/mod2", "carol/mod3"]`
	got := expandArrayLine(line)

	// Expect: header, 3 items, closing bracket = 5 lines.
	if len(got) != 5 {
		t.Fatalf("want 5 lines for 3-element array, got %d: %v", len(got), got)
	}
	if got[0] != `mods = [` {
		t.Errorf("first line: want `mods = [`, got %q", got[0])
	}
	if !strings.Contains(got[1], `"alice/mod1"`) {
		t.Errorf("second line should contain alice/mod1, got %q", got[1])
	}
	if got[4] != `]` {
		t.Errorf("last line: want `]`, got %q", got[4])
	}
}

// ---------------------------------------------------------------------------
// expandMultilineArrays
// ---------------------------------------------------------------------------

func TestExpandMultilineArrays_NoArrays(t *testing.T) {
	src := "name = \"server\"\nport = 30000\n"
	got := expandMultilineArrays(src)
	if got != src {
		t.Errorf("string with no arrays should be unchanged\ngot: %q", got)
	}
}

func TestExpandMultilineArrays_MultiElementExpanded(t *testing.T) {
	src := `mods = ["alice/mod1", "bob/mod2"]` + "\n"
	got := expandMultilineArrays(src)
	if !strings.Contains(got, "\"alice/mod1\",") {
		t.Errorf("expanded output should contain alice/mod1 on its own line, got:\n%s", got)
	}
	if !strings.Contains(got, "\"bob/mod2\",") {
		t.Errorf("expanded output should contain bob/mod2 on its own line, got:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// Load / Save round-trip
// ---------------------------------------------------------------------------

func TestLoadSaveRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, Filename)

	p := Default()
	p.Server.Name = "Test Server"
	p.Server.Port = 12345
	AddPackage(p, "alice/mod1", "mod")
	AddPackage(p, "bob/mod2", "mod")
	AddPackage(p, "devteam/mygame", "game")

	if err := Save(p, path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.Server.Name != p.Server.Name {
		t.Errorf("server.name: want %q, got %q", p.Server.Name, loaded.Server.Name)
	}
	if loaded.Server.Port != p.Server.Port {
		t.Errorf("server.port: want %d, got %d", p.Server.Port, loaded.Server.Port)
	}
	if len(loaded.Packages.Mods) != 2 {
		t.Errorf("mods: want 2, got %d", len(loaded.Packages.Mods))
	}
	if len(loaded.Packages.Games) != 1 {
		t.Errorf("games: want 1, got %d", len(loaded.Packages.Games))
	}
}

func TestSave_SortsMods(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, Filename)

	p := Default()
	p.Packages.Mods = []string{"zzz/last", "aaa/first", "mmm/middle"}

	if err := Save(p, path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.Packages.Mods[0] != "aaa/first" {
		t.Errorf("mods[0]: want aaa/first, got %q", loaded.Packages.Mods[0])
	}
	if loaded.Packages.Mods[2] != "zzz/last" {
		t.Errorf("mods[2]: want zzz/last, got %q", loaded.Packages.Mods[2])
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path/luanti.toml")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestLoad_MalformedTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, Filename)

	if err := os.WriteFile(path, []byte("[[[ not valid toml"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Error("expected error for malformed TOML, got nil")
	}
}

// ---------------------------------------------------------------------------
// SetModEnabled
// ---------------------------------------------------------------------------

func TestSetModEnabled_AppendToEmptyFile(t *testing.T) {
	worldDir := t.TempDir()
	worldMT := filepath.Join(worldDir, "world.mt")

	if err := os.WriteFile(worldMT, []byte(""), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := SetModEnabled(worldDir, "mymod", true); err != nil {
		t.Fatalf("SetModEnabled: %v", err)
	}

	data, _ := os.ReadFile(worldMT)
	if !strings.Contains(string(data), "load_mod_mymod = true") {
		t.Errorf("expected load_mod_mymod = true in world.mt, got:\n%s", data)
	}
}

func TestSetModEnabled_AppendToExistingFile(t *testing.T) {
	worldDir := t.TempDir()
	worldMT := filepath.Join(worldDir, "world.mt")

	initial := "load_mod_othermod = true\n"
	if err := os.WriteFile(worldMT, []byte(initial), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := SetModEnabled(worldDir, "newmod", true); err != nil {
		t.Fatalf("SetModEnabled: %v", err)
	}

	data, _ := os.ReadFile(worldMT)
	content := string(data)
	if !strings.Contains(content, "load_mod_othermod = true") {
		t.Error("existing entry should be preserved")
	}
	if !strings.Contains(content, "load_mod_newmod = true") {
		t.Errorf("new entry should be appended, got:\n%s", content)
	}
}

func TestSetModEnabled_UpdateExisting(t *testing.T) {
	worldDir := t.TempDir()
	worldMT := filepath.Join(worldDir, "world.mt")

	initial := "load_mod_mymod = false\nload_mod_other = true\n"
	if err := os.WriteFile(worldMT, []byte(initial), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := SetModEnabled(worldDir, "mymod", true); err != nil {
		t.Fatalf("SetModEnabled: %v", err)
	}

	data, _ := os.ReadFile(worldMT)
	content := string(data)
	if strings.Contains(content, "load_mod_mymod = false") {
		t.Error("old value should have been replaced")
	}
	if !strings.Contains(content, "load_mod_mymod = true") {
		t.Errorf("updated value not found, got:\n%s", content)
	}
}

func TestSetModEnabled_Disable(t *testing.T) {
	worldDir := t.TempDir()
	worldMT := filepath.Join(worldDir, "world.mt")

	initial := "load_mod_mymod = true\n"
	if err := os.WriteFile(worldMT, []byte(initial), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := SetModEnabled(worldDir, "mymod", false); err != nil {
		t.Fatalf("SetModEnabled: %v", err)
	}

	data, _ := os.ReadFile(worldMT)
	if !strings.Contains(string(data), "load_mod_mymod = false") {
		t.Errorf("expected load_mod_mymod = false, got:\n%s", data)
	}
}

func TestSetModEnabled_MissingWorldMT(t *testing.T) {
	worldDir := t.TempDir()
	err := SetModEnabled(worldDir, "mymod", true)
	if err == nil {
		t.Error("expected error when world.mt does not exist, got nil")
	}
}

func TestSetModEnabled_NoPrefixCollision(t *testing.T) {
	worldDir := t.TempDir()
	worldMT := filepath.Join(worldDir, "world.mt")

	// load_mod_mymod_extended must not be touched when enabling load_mod_mymod.
	initial := "load_mod_mymod_extended = false\n"
	if err := os.WriteFile(worldMT, []byte(initial), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := SetModEnabled(worldDir, "mymod", true); err != nil {
		t.Fatalf("SetModEnabled: %v", err)
	}

	data, _ := os.ReadFile(worldMT)
	content := string(data)
	if !strings.Contains(content, "load_mod_mymod_extended = false") {
		t.Errorf("load_mod_mymod_extended should be unchanged, got:\n%s", content)
	}
	if !strings.Contains(content, "load_mod_mymod = true") {
		t.Errorf("load_mod_mymod should be appended, got:\n%s", content)
	}
}

// ---------------------------------------------------------------------------
// SyncConfig
// ---------------------------------------------------------------------------

func TestSyncConfig_EmptyConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "minetest.conf")
	if err := SyncConfig(path, map[string]any{}); err != nil {
		t.Errorf("empty config should return nil, got: %v", err)
	}
}

func TestSyncConfig_CreatesFileWithNewKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "minetest.conf")

	cfg := map[string]any{"server_name": "MyServer", "max_users": 10}
	if err := SyncConfig(path, cfg); err != nil {
		t.Fatalf("SyncConfig: %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, "server_name = MyServer") {
		t.Errorf("server_name not found in:\n%s", content)
	}
	if !strings.Contains(content, "max_users = 10") {
		t.Errorf("max_users not found in:\n%s", content)
	}
}

func TestSyncConfig_UpdatesExistingKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "minetest.conf")

	initial := "server_name = OldName\nmax_users = 5\n"
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := SyncConfig(path, map[string]any{"server_name": "NewName"}); err != nil {
		t.Fatalf("SyncConfig: %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)
	if strings.Contains(content, "server_name = OldName") {
		t.Error("old value should have been replaced")
	}
	if !strings.Contains(content, "server_name = NewName") {
		t.Errorf("updated value not found, got:\n%s", content)
	}
	// Untouched key should remain.
	if !strings.Contains(content, "max_users = 5") {
		t.Errorf("unrelated key should be preserved, got:\n%s", content)
	}
}

func TestSyncConfig_SkipsCommentLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "minetest.conf")

	initial := "# server_name = commented\nserver_name = real\n"
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := SyncConfig(path, map[string]any{"server_name": "Updated"}); err != nil {
		t.Fatalf("SyncConfig: %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, "# server_name = commented") {
		t.Error("comment line should be left untouched")
	}
	if !strings.Contains(content, "server_name = Updated") {
		t.Errorf("active key should be updated, got:\n%s", content)
	}
}

func TestSyncConfig_AppendsSortedNewKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "minetest.conf")

	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	cfg := map[string]any{"zzz_key": "z", "aaa_key": "a", "mmm_key": "m"}
	if err := SyncConfig(path, cfg); err != nil {
		t.Fatalf("SyncConfig: %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)
	lines := strings.Split(strings.TrimSpace(content), "\n")

	// All three keys should appear; order should be sorted (aaa, mmm, zzz).
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d: %s", len(lines), content)
	}
	if !strings.HasPrefix(lines[0], "aaa_key") {
		t.Errorf("first appended key should be aaa_key, got %q", lines[0])
	}
	if !strings.HasPrefix(lines[2], "zzz_key") {
		t.Errorf("last appended key should be zzz_key, got %q", lines[2])
	}
}

// ---------------------------------------------------------------------------
// LoadCurrent
// ---------------------------------------------------------------------------

func TestLoadCurrent_WithFile(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	p := Default()
	p.Server.Name = "LoadCurrent Test"
	if err := Save(p, Filename); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := LoadCurrent()
	if err != nil {
		t.Fatalf("LoadCurrent: %v", err)
	}
	if loaded.Server.Name != "LoadCurrent Test" {
		t.Errorf("server.name: want LoadCurrent Test, got %q", loaded.Server.Name)
	}
}

func TestLoadCurrent_NotFound(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	_, err := LoadCurrent()
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
