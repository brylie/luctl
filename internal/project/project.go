// Package project defines the luanti.toml project manifest format.
package project

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// Filename is the standard project manifest file name.
const Filename = "luanti.toml"

// Project is the top-level structure of a luanti.toml manifest.
type Project struct {
	Server   ServerConfig   `toml:"server"`
	World    WorldConfig    `toml:"world"`
	Paths    PathsConfig    `toml:"paths"`
	Packages PackagesConfig `toml:"packages"`
	Config   map[string]any `toml:"config"`
}

// ServerConfig holds server identity and network settings.
type ServerConfig struct {
	Name        string   `toml:"name"`
	Description string   `toml:"description"`
	Port        int      `toml:"port"`
	Admins      []string `toml:"admins"`
}

// WorldConfig holds world generation settings.
type WorldConfig struct {
	Game   string `toml:"game"`
	Mapgen string `toml:"mapgen"`
}

// PathsConfig holds filesystem paths used by luctl commands.
type PathsConfig struct {
	ModsDir  string `toml:"mods_dir"`
	GamesDir string `toml:"games_dir"`
	WorldDir string `toml:"world_dir"`
	ConfFile string `toml:"conf_file"`
}

// PackagesConfig holds the declared package dependencies, split by type.
// IDs use ContentDB "author/name" format.
type PackagesConfig struct {
	Mods  []string `toml:"mods"`
	Games []string `toml:"games"`
}

// AllEntries returns every declared package as (id, type) pairs,
// convenient for iteration in install/status commands.
func (pc *PackagesConfig) AllEntries() []PackageEntry {
	entries := make([]PackageEntry, 0, len(pc.Mods)+len(pc.Games))

	for _, id := range pc.Mods {
		entries = append(entries, PackageEntry{ID: id, Type: "mod"})
	}

	for _, id := range pc.Games {
		entries = append(entries, PackageEntry{ID: id, Type: "game"})
	}

	return entries
}

// PackageEntry is a resolved (id, type) pair produced by AllEntries.
type PackageEntry struct {
	ID   string
	Type string
}

// Default returns a Project populated with sensible defaults.
func Default() *Project {
	return &Project{
		Server: ServerConfig{
			Name:        "My Luanti Server",
			Description: "",
			Port:        30000,
			Admins:      []string{},
		},
		World: WorldConfig{
			Game:   "minetest_game",
			Mapgen: "v7",
		},
		Paths: PathsConfig{
			ModsDir:  "./mods",
			GamesDir: "./games",
			WorldDir: "./data/worlds/world",
			ConfFile: "./data/main-config/minetest.conf",
		},
		Packages: PackagesConfig{},
		Config:   map[string]any{},
	}
}

// Load reads and parses a luanti.toml file from path.
func Load(path string) (*Project, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening project file: %w", err)
	}
	defer func() { _ = f.Close() }()

	var p Project
	if _, err := toml.NewDecoder(f).Decode(&p); err != nil {
		return nil, fmt.Errorf("parsing project file: %w", err)
	}

	return &p, nil
}

// Save writes the project to path as TOML.
// Mods and games lists are each sorted alphabetically before writing.
// String arrays with more than one element are written one item per line.
func Save(p *Project, path string) error {
	slices.Sort(p.Packages.Mods)
	slices.Sort(p.Packages.Games)

	var buf strings.Builder

	if err := toml.NewEncoder(&buf).Encode(p); err != nil {
		return fmt.Errorf("encoding project file: %w", err)
	}

	formatted := expandMultilineArrays(buf.String())

	if err := os.WriteFile(path, []byte(formatted), 0o600); err != nil {
		return fmt.Errorf("writing project file: %w", err)
	}

	return nil
}

// expandMultilineArrays post-processes TOML output so that string arrays
// with more than one element are rendered one item per line, e.g.:
//
//	mods = [
//	  "author/a",
//	  "author/b",
//	]
func expandMultilineArrays(src string) string {
	lines := strings.Split(src, "\n")
	out := make([]string, 0, len(lines))

	for _, line := range lines {
		out = append(out, expandArrayLine(line)...)
	}

	return strings.Join(out, "\n")
}

// expandArrayLine expands a single `key = ["a", "b", ...]` line into multiple
// lines.  Lines with zero or one element are left unchanged.
func expandArrayLine(line string) []string {
	const sep = " = ["

	key, tail, ok := strings.Cut(line, sep)
	if !ok || !strings.HasSuffix(strings.TrimRight(tail, " "), "]") {
		return []string{line}
	}

	tail = strings.TrimRight(tail, " ")
	inner := tail[:len(tail)-1] // strip trailing ]

	if strings.TrimSpace(inner) == "" {
		return []string{line} // keep `key = []` inline
	}

	rawItems := strings.Split(inner, ", ")
	if len(rawItems) <= 1 {
		return []string{line}
	}

	indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]

	result := []string{key + " = ["}

	for _, item := range rawItems {
		result = append(result, indent+"  "+strings.TrimSpace(item)+",")
	}

	result = append(result, indent+"]")

	return result
}

// ErrNotFound is returned when no luanti.toml exists in the current directory.
var ErrNotFound = errors.New("no luanti.toml found in current directory — run: luctl project init")

// SetModEnabled writes load_mod_<modName> = <enabled> into world.mt, updating
// an existing line in-place or appending if no entry exists yet.
func SetModEnabled(worldDir, modName string, enabled bool) error {
	path := filepath.Clean(filepath.Join(worldDir, "world.mt"))

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading world.mt: %w", err)
	}

	key := "load_mod_" + modName
	newLine := fmt.Sprintf("%s = %v", key, enabled)

	lines := strings.Split(string(data), "\n")
	found := false

	for i, line := range lines {
		if strings.HasPrefix(line, key) {
			lines[i] = newLine
			found = true

			break
		}
	}

	var content string

	if found {
		content = strings.Join(lines, "\n")
	} else {
		content = string(data)
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}

		content += newLine + "\n"
	}

	//nolint:gosec // path is constructed from config values, not user-controlled HTTP input
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return fmt.Errorf("writing world.mt: %w", err)
	}

	return nil
}

// AddPackage appends a package to the appropriate list if not already declared.
// Returns true if the entry was added, false if a duplicate was found.
func AddPackage(p *Project, id, pkgType string) bool {
	switch pkgType {
	case "game":
		if slices.Contains(p.Packages.Games, id) {
			return false
		}

		p.Packages.Games = append(p.Packages.Games, id)
	default:
		if slices.Contains(p.Packages.Mods, id) {
			return false
		}

		p.Packages.Mods = append(p.Packages.Mods, id)
	}

	return true
}

// LoadCurrent loads luanti.toml from the current working directory.
func LoadCurrent() (*Project, error) {
	p, err := Load(Filename)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}

	return p, err
}

// SyncConfig applies every key in config to confPath (a minetest.conf file),
// updating existing lines in-place and appending unknown keys at the end.
// Lines that begin with # are left untouched.
func SyncConfig(confPath string, config map[string]any) error {
	if len(config) == 0 {
		return nil
	}

	clean := filepath.Clean(confPath)

	data, err := os.ReadFile(clean)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading config file: %w", err)
	}

	lines := strings.Split(string(data), "\n")

	// Track which keys still need to be appended.
	pending := make(map[string]any, len(config))

	maps.Copy(pending, config)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || !strings.Contains(trimmed, "=") {
			continue
		}

		key := strings.TrimSpace(strings.SplitN(trimmed, "=", 2)[0])

		if val, ok := pending[key]; ok {
			lines[i] = fmt.Sprintf("%s = %v", key, val)
			delete(pending, key)
		}
	}

	result := strings.Join(lines, "\n")
	if !strings.HasSuffix(result, "\n") {
		result += "\n"
	}

	// Append remaining keys in sorted order for deterministic output.
	remaining := make([]string, 0, len(pending))

	for k := range pending {
		remaining = append(remaining, k)
	}

	sort.Strings(remaining)

	var sb strings.Builder

	for _, k := range remaining {
		fmt.Fprintf(&sb, "%s = %v\n", k, pending[k])
	}

	result += sb.String()

	//nolint:gosec // path is constructed from config values, not user-controlled HTTP input
	if err := os.WriteFile(clean, []byte(result), 0o600); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}
