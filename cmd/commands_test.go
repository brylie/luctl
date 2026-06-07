package cmd

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brylie/luctl/internal/contentdb"
	"github.com/brylie/luctl/internal/project"
)

// silenceCmd redirects cobra's own output so test logs stay clean.
func silenceCmd(cmd interface {
	SetOut(io.Writer)
	SetErr(io.Writer)
}) {
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
}

// ---------------------------------------------------------------------------
// pkg list
// ---------------------------------------------------------------------------

func TestPkgListCmd_ExplicitDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "mod1"), 0o750); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "mod2"), 0o750); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	// Regular file should be ignored.
	_ = os.WriteFile(filepath.Join(dir, "readme.txt"), []byte(""), 0o600)

	cmd := newPkgListCmd()
	silenceCmd(cmd)
	cmd.SetArgs([]string{"--mods-dir", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestPkgListCmd_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	cmd := newPkgListCmd()
	silenceCmd(cmd)
	cmd.SetArgs([]string{"--mods-dir", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute on empty dir: %v", err)
	}
}

func TestPkgListCmd_MissingDir(t *testing.T) {
	cmd := newPkgListCmd()
	silenceCmd(cmd)
	cmd.SetArgs([]string{"--mods-dir", "/nonexistent/path/to/mods"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for missing directory, got nil")
	}
}

// ---------------------------------------------------------------------------
// pkg enable / disable
// ---------------------------------------------------------------------------

func TestPkgEnableCmd(t *testing.T) {
	worldDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(worldDir, "world.mt"), []byte(""), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	cmd := newPkgEnableCmd()
	silenceCmd(cmd)
	cmd.SetArgs([]string{"--world-dir", worldDir, "mymod"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(worldDir, "world.mt"))
	if !strings.Contains(string(data), "load_mod_mymod = true") {
		t.Errorf("expected load_mod_mymod = true, got:\n%s", data)
	}
}

func TestPkgDisableCmd(t *testing.T) {
	worldDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(worldDir, "world.mt"), []byte("load_mod_mymod = true\n"), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	cmd := newPkgDisableCmd()
	silenceCmd(cmd)
	cmd.SetArgs([]string{"--world-dir", worldDir, "mymod"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(worldDir, "world.mt"))
	if !strings.Contains(string(data), "load_mod_mymod = false") {
		t.Errorf("expected load_mod_mymod = false, got:\n%s", data)
	}
}

func TestPkgEnableCmd_MissingWorldMT(t *testing.T) {
	worldDir := t.TempDir()
	// No world.mt created.
	cmd := newPkgEnableCmd()
	silenceCmd(cmd)
	cmd.SetArgs([]string{"--world-dir", worldDir, "mymod"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error when world.mt is missing, got nil")
	}
}

// ---------------------------------------------------------------------------
// project init
// ---------------------------------------------------------------------------

func TestProjectInitCmd(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	cmd := newProjectInitCmd()
	silenceCmd(cmd)
	cmd.SetArgs([]string{"--name", "TestServer", "--admin", "testadmin", "--game", "mygame"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	p, err := project.Load(filepath.Join(dir, project.Filename))
	if err != nil {
		t.Fatalf("Load after init: %v", err)
	}
	if p.Server.Name != "TestServer" {
		t.Errorf("server.name: want TestServer, got %q", p.Server.Name)
	}
	if len(p.Server.Admins) != 1 || p.Server.Admins[0] != "testadmin" {
		t.Errorf("admins: want [testadmin], got %v", p.Server.Admins)
	}
	if p.World.Game != "mygame" {
		t.Errorf("world.game: want mygame, got %q", p.World.Game)
	}
}

func TestProjectInitCmd_AlreadyExists(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	// Pre-create the manifest.
	if err := os.WriteFile(project.Filename, []byte("[server]\nname=\"existing\"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cmd := newProjectInitCmd()
	silenceCmd(cmd)
	if err := cmd.Execute(); err == nil {
		t.Error("expected error when luanti.toml already exists, got nil")
	}
}

// ---------------------------------------------------------------------------
// project fmt
// ---------------------------------------------------------------------------

func TestProjectFmtCmd(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	// Write unsorted manifest.
	p := project.Default()
	p.Packages.Mods = []string{"zzz/last", "aaa/first", "mmm/middle"}
	if err := project.Save(p, filepath.Join(dir, project.Filename)); err != nil {
		t.Fatalf("Save: %v", err)
	}

	cmd := newProjectFmtCmd()
	silenceCmd(cmd)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	loaded, err := project.Load(filepath.Join(dir, project.Filename))
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

func TestProjectFmtCmd_NoManifest(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	cmd := newProjectFmtCmd()
	silenceCmd(cmd)
	if err := cmd.Execute(); err == nil {
		t.Error("expected error when no luanti.toml, got nil")
	}
}

// ---------------------------------------------------------------------------
// project sync
// ---------------------------------------------------------------------------

func TestProjectSyncCmd_NoConfig(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	p := project.Default()
	if err := project.Save(p, filepath.Join(dir, project.Filename)); err != nil {
		t.Fatalf("Save: %v", err)
	}

	cmd := newProjectSyncCmd()
	silenceCmd(cmd)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute with no config: %v", err)
	}
}

func TestProjectSyncCmd_WithConfig(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	confPath := filepath.Join(dir, "minetest.conf")

	p := project.Default()
	p.Paths.ConfFile = confPath
	p.Config = map[string]any{"server_name": "TestServer", "max_users": 20}
	if err := project.Save(p, filepath.Join(dir, project.Filename)); err != nil {
		t.Fatalf("Save: %v", err)
	}

	cmd := newProjectSyncCmd()
	silenceCmd(cmd)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute with config: %v", err)
	}

	data, _ := os.ReadFile(confPath)
	if !strings.Contains(string(data), "server_name = TestServer") {
		t.Errorf("server_name not in conf:\n%s", data)
	}
}

func TestProjectSyncCmd_NoManifest(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	cmd := newProjectSyncCmd()
	silenceCmd(cmd)
	if err := cmd.Execute(); err == nil {
		t.Error("expected error when no luanti.toml, got nil")
	}
}

func TestProjectSyncCmd_ConfigWithNoConfFile(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	// Config present but paths.conf_file is empty.
	p := project.Default()
	p.Paths.ConfFile = ""
	p.Config = map[string]any{"server_name": "Test"}
	if err := project.Save(p, filepath.Join(dir, project.Filename)); err != nil {
		t.Fatalf("Save: %v", err)
	}

	cmd := newProjectSyncCmd()
	silenceCmd(cmd)
	if err := cmd.Execute(); err == nil {
		t.Error("expected error when conf_file path is empty, got nil")
	}
}

// ---------------------------------------------------------------------------
// newRootCmd / newPackageCmd / newProjectCmd — construction covers all
// subcommand constructor code paths in one call.
// ---------------------------------------------------------------------------

func TestNewRootCmd(t *testing.T) {
	root := newRootCmd()
	if root == nil {
		t.Fatal("newRootCmd returned nil")
	}
	if root.Use != "luctl" {
		t.Errorf("root.Use: want luctl, got %q", root.Use)
	}
	// Root should have exactly 3 subcommands: package, project, and server.
	if len(root.Commands()) != 3 {
		t.Errorf("want 3 subcommands, got %d", len(root.Commands()))
	}
}

func TestNewPackageCmd(t *testing.T) {
	cmd := newPackageCmd()
	if cmd == nil {
		t.Fatal("newPackageCmd returned nil")
	}
	names := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		names[sub.Name()] = true
	}
	for _, want := range []string{"search", "info", "install", "enable", "disable", "list", "update"} {
		if !names[want] {
			t.Errorf("subcommand %q not registered", want)
		}
	}
}

func TestNewProjectCmd(t *testing.T) {
	cmd := newProjectCmd()
	if cmd == nil {
		t.Fatal("newProjectCmd returned nil")
	}
	names := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		names[sub.Name()] = true
	}
	for _, want := range []string{"init", "install", "status", "fmt", "sync"} {
		if !names[want] {
			t.Errorf("subcommand %q not registered", want)
		}
	}
}

// ---------------------------------------------------------------------------
// backupSources (server backup)
// ---------------------------------------------------------------------------

func TestBackupSources_BothPresent(t *testing.T) {
	dir := t.TempDir()
	worldDir := filepath.Join(dir, "world")

	if err := os.Mkdir(worldDir, 0o750); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	confFile := filepath.Join(dir, "minetest.conf")

	if err := os.WriteFile(confFile, []byte(""), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	p := &project.Project{}
	p.Paths.WorldDir = worldDir
	p.Paths.ConfFile = confFile

	sources, err := backupSources(p)
	if err != nil {
		t.Fatalf("backupSources: %v", err)
	}

	if len(sources) != 2 {
		t.Errorf("want 2 sources, got %d", len(sources))
	}
}

func TestBackupSources_OnlyWorldDir(t *testing.T) {
	dir := t.TempDir()
	worldDir := filepath.Join(dir, "world")

	if err := os.Mkdir(worldDir, 0o750); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	p := &project.Project{}
	p.Paths.WorldDir = worldDir

	sources, err := backupSources(p)
	if err != nil {
		t.Fatalf("backupSources: %v", err)
	}

	if len(sources) != 1 || sources[0] != worldDir {
		t.Errorf("want [%s], got %v", worldDir, sources)
	}
}

func TestBackupSources_MissingPath(t *testing.T) {
	p := &project.Project{}
	p.Paths.WorldDir = "/nonexistent/world"

	if _, err := backupSources(p); err == nil {
		t.Error("expected error for missing world_dir, got nil")
	}
}

func TestBackupSources_NoSources(t *testing.T) {
	p := &project.Project{}

	if _, err := backupSources(p); err == nil {
		t.Error("expected error when no sources configured, got nil")
	}
}

func TestNewServerCmd(t *testing.T) {
	cmd := newServerCmd()
	if cmd == nil {
		t.Fatal("newServerCmd returned nil")
	}
	names := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		names[sub.Name()] = true
	}
	for _, want := range []string{"backup", "restore"} {
		if !names[want] {
			t.Errorf("subcommand %q not registered", want)
		}
	}
}

// ---------------------------------------------------------------------------
// project status command
// ---------------------------------------------------------------------------

func TestProjectStatusCmd_NoPackages(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	p := project.Default()
	if err := project.Save(p, filepath.Join(dir, project.Filename)); err != nil {
		t.Fatalf("Save: %v", err)
	}

	cmd := newProjectStatusCmd()
	silenceCmd(cmd)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestProjectStatusCmd_WithPackages(t *testing.T) {
	dir := t.TempDir()
	modsDir := filepath.Join(dir, "mods")
	if err := os.Mkdir(modsDir, 0o750); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	// One installed, one missing.
	if err := os.Mkdir(filepath.Join(modsDir, "mod1"), 0o750); err != nil {
		t.Fatalf("Mkdir mod1: %v", err)
	}

	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	p := project.Default()
	p.Paths.ModsDir = modsDir
	p.Packages.Mods = []string{"alice/mod1", "bob/mod2"}
	if err := project.Save(p, filepath.Join(dir, project.Filename)); err != nil {
		t.Fatalf("Save: %v", err)
	}

	cmd := newProjectStatusCmd()
	silenceCmd(cmd)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestProjectStatusCmd_NoManifest(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	cmd := newProjectStatusCmd()
	silenceCmd(cmd)
	if err := cmd.Execute(); err == nil {
		t.Error("expected error when no luanti.toml, got nil")
	}
}

// ---------------------------------------------------------------------------
// pkg search  (uses HTTP via contentdb.BaseURL)
// ---------------------------------------------------------------------------

// withCmdBaseURL overrides the cmd-package client factory for the duration of
// the test so that commands point at a local httptest.Server instead of
// the production ContentDB.
func withCmdBaseURL(t *testing.T, url string) {
	t.Helper()
	orig := newClient
	newClient = func() *contentdb.Client {
		return contentdb.NewWithClient(url, &http.Client{})
	}
	t.Cleanup(func() { newClient = orig })
}

func TestPkgSearchCmd(t *testing.T) {
	packages := []contentdb.Package{
		{Author: "alice", Name: "mod1", Title: "Mod One", Type: "mod", ShortDescription: "A test mod"},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(packages)
	}))
	defer ts.Close()
	withCmdBaseURL(t, ts.URL)

	cmd := newPkgSearchCmd()
	silenceCmd(cmd)
	cmd.SetArgs([]string{"--limit", "5", "mod"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestPkgSearchCmd_EmptyResults(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]contentdb.Package{})
	}))
	defer ts.Close()
	withCmdBaseURL(t, ts.URL)

	cmd := newPkgSearchCmd()
	silenceCmd(cmd)
	cmd.SetArgs([]string{"nonexistent_mod"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute on empty results: %v", err)
	}
}

func TestPkgSearchCmd_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()
	withCmdBaseURL(t, ts.URL)

	cmd := newPkgSearchCmd()
	silenceCmd(cmd)
	cmd.SetArgs([]string{"query"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for API failure, got nil")
	}
}

// ---------------------------------------------------------------------------
// pkg info
// ---------------------------------------------------------------------------

func TestPkgInfoCmd(t *testing.T) {
	pkg := contentdb.Package{
		Author: "alice", Name: "mod1", Title: "Mod One", Type: "mod",
		License: "MIT", Downloads: 100, Tags: []string{"survival"},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(pkg)
	}))
	defer ts.Close()
	withCmdBaseURL(t, ts.URL)

	cmd := newPkgInfoCmd()
	silenceCmd(cmd)
	cmd.SetArgs([]string{"alice/mod1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestPkgInfoCmd_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()
	withCmdBaseURL(t, ts.URL)

	cmd := newPkgInfoCmd()
	silenceCmd(cmd)
	cmd.SetArgs([]string{"alice/nonexistent"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for 404, got nil")
	}
}

func TestPkgInfoCmd_InvalidArg(t *testing.T) {
	cmd := newPkgInfoCmd()
	silenceCmd(cmd)
	cmd.SetArgs([]string{"noslash"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for invalid author/name format, got nil")
	}
}

// ---------------------------------------------------------------------------
// project install (covers installPackages)
// ---------------------------------------------------------------------------

// buildCmdZip returns a minimal in-memory zip with a single top-dir entry,
// matching ContentDB archive layout used in cmd-level install tests.
func buildCmdZip(t *testing.T, modName string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, name := range []string{modName + "-v1.0/init.lua"} {
		f, err := w.Create(name)
		if err != nil {
			t.Fatalf("zip.Create: %v", err)
		}
		if _, err := f.Write([]byte("-- mod")); err != nil {
			t.Fatalf("zip write: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("zip.Close: %v", err)
	}

	return buf.Bytes()
}

func TestProjectInstallCmd_SingleMod(t *testing.T) {
	modZip := buildCmdZip(t, "mod1")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(modZip)
	}))
	defer ts.Close()
	withCmdBaseURL(t, ts.URL)

	dir := t.TempDir()
	modsDir := filepath.Join(dir, "mods")
	if err := os.Mkdir(modsDir, 0o750); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	p := project.Default()
	p.Paths.ModsDir = modsDir
	p.Paths.WorldDir = ""
	p.Packages.Mods = []string{"alice/mod1"}
	if err := project.Save(p, filepath.Join(dir, project.Filename)); err != nil {
		t.Fatalf("Save: %v", err)
	}

	cmd := newProjectInstallCmd()
	silenceCmd(cmd)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if _, err := os.Stat(filepath.Join(modsDir, "mod1")); err != nil {
		t.Errorf("mod1 should be extracted to mods dir: %v", err)
	}
}

func TestProjectInstallCmd_NoPackages(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	p := project.Default()
	// No packages declared.
	if err := project.Save(p, filepath.Join(dir, project.Filename)); err != nil {
		t.Fatalf("Save: %v", err)
	}

	cmd := newProjectInstallCmd()
	silenceCmd(cmd)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute with no packages: %v", err)
	}
}

func TestProjectInstallCmd_NoManifest(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	cmd := newProjectInstallCmd()
	silenceCmd(cmd)
	if err := cmd.Execute(); err == nil {
		t.Error("expected error when no luanti.toml, got nil")
	}
}

// ---------------------------------------------------------------------------
// server backup list
// ---------------------------------------------------------------------------

const fakeServerListXML = `<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>test-bucket</Name>
  <IsTruncated>false</IsTruncated>
  <Contents>
    <Key>luanti/backup-2026-06-07T15-33-51Z.tar.gz</Key>
    <LastModified>2026-06-07T15:33:53.000Z</LastModified>
    <Size>12345</Size>
  </Contents>
</ListBucketResult>`

const fakeServerEmptyXML = `<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>test-bucket</Name>
  <IsTruncated>false</IsTruncated>
</ListBucketResult>`

// setupBackupManifest writes a minimal manifest pointing at ts and changes
// into dir for the duration of the test.
func setupBackupManifest(t *testing.T, dir, tsURL string) {
	t.Helper()

	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	t.Cleanup(func() { _ = os.Chdir(orig) })

	p := project.Default()
	p.Backup.Bucket = "test-bucket"
	p.Backup.Endpoint = tsURL
	p.Backup.Region = "us-east-1"
	p.Backup.Prefix = "luanti/"

	if err := project.Save(p, filepath.Join(dir, project.Filename)); err != nil {
		t.Fatalf("Save manifest: %v", err)
	}

	t.Setenv("LUCTL_S3_ACCESS_KEY", "test-key")
	t.Setenv("LUCTL_S3_SECRET_KEY", "test-secret")
}

func TestServerBackupListCmd_WithResults(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(fakeServerListXML))
	}))
	defer ts.Close()

	setupBackupManifest(t, t.TempDir(), ts.URL)

	cmd := newServerBackupListCmd()
	silenceCmd(cmd)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestServerBackupListCmd_NoBackups(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(fakeServerEmptyXML))
	}))
	defer ts.Close()

	setupBackupManifest(t, t.TempDir(), ts.URL)

	cmd := newServerBackupListCmd()
	silenceCmd(cmd)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute (no backups): %v", err)
	}
}

func TestServerBackupListCmd_NoManifest(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	cmd := newServerBackupListCmd()
	silenceCmd(cmd)

	if err := cmd.Execute(); err == nil {
		t.Error("expected error when no luanti.toml, got nil")
	}
}

// ---------------------------------------------------------------------------
// server backup create
// ---------------------------------------------------------------------------

func TestServerBackupCreateCmd(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("ETag", `"abc123"`)
	}))
	defer ts.Close()

	dir := t.TempDir()
	worldDir := filepath.Join(dir, "world")

	if err := os.Mkdir(worldDir, 0o750); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	confFile := filepath.Join(dir, "minetest.conf")

	if err := os.WriteFile(confFile, []byte(""), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	setupBackupManifest(t, dir, ts.URL)

	p, err := project.Load(filepath.Join(dir, project.Filename))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	p.Paths.WorldDir = worldDir
	p.Paths.ConfFile = confFile

	if err := project.Save(p, filepath.Join(dir, project.Filename)); err != nil {
		t.Fatalf("Save: %v", err)
	}

	cmd := newServerBackupCreateCmd()
	silenceCmd(cmd)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

// ---------------------------------------------------------------------------
// server restore
// ---------------------------------------------------------------------------

// emptyTarGz builds a valid but empty tar.gz archive.
func emptyTarGz(t *testing.T) []byte {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	_ = tw.Close()
	_ = gz.Close()

	return buf.Bytes()
}

func TestServerRestoreCmd_NoManifest(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	cmd := newServerRestoreCmd()
	silenceCmd(cmd)

	if err := cmd.Execute(); err == nil {
		t.Error("expected error when no luanti.toml, got nil")
	}
}

func TestServerRestoreCmd_NoWorldDir(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	defer ts.Close()

	dir := t.TempDir()
	setupBackupManifest(t, dir, ts.URL)

	// project.Default sets a non-empty WorldDir; clear it explicitly.
	p, err := project.Load(filepath.Join(dir, project.Filename))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	p.Paths.WorldDir = ""

	if err := project.Save(p, filepath.Join(dir, project.Filename)); err != nil {
		t.Fatalf("Save: %v", err)
	}

	cmd := newServerRestoreCmd()
	silenceCmd(cmd)
	cmd.SetArgs([]string{"backup.tar.gz"})

	if err := cmd.Execute(); err == nil {
		t.Error("expected error when paths.world_dir is empty, got nil")
	}
}

func TestServerRestoreCmd_Cancelled(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	defer ts.Close()

	dir := t.TempDir()
	worldDir := filepath.Join(dir, "world")

	if err := os.Mkdir(worldDir, 0o750); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	setupBackupManifest(t, dir, ts.URL)

	p, err := project.Load(filepath.Join(dir, project.Filename))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	p.Paths.WorldDir = worldDir

	if err := project.Save(p, filepath.Join(dir, project.Filename)); err != nil {
		t.Fatalf("Save: %v", err)
	}

	cmd := newServerRestoreCmd()
	silenceCmd(cmd)
	cmd.SetArgs([]string{"backup.tar.gz"})
	cmd.SetIn(strings.NewReader("n\n"))

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute (cancelled): %v", err)
	}
}

func TestServerRestoreCmd_Force(t *testing.T) {
	archive := emptyTarGz(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			return
		}
		_, _ = w.Write(archive)
	}))
	defer ts.Close()

	dir := t.TempDir()
	worldDir := filepath.Join(dir, "world")

	if err := os.Mkdir(worldDir, 0o750); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	setupBackupManifest(t, dir, ts.URL)

	p, err := project.Load(filepath.Join(dir, project.Filename))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	p.Paths.WorldDir = worldDir

	if err := project.Save(p, filepath.Join(dir, project.Filename)); err != nil {
		t.Fatalf("Save: %v", err)
	}

	cmd := newServerRestoreCmd()
	silenceCmd(cmd)
	cmd.SetArgs([]string{"--force", "backup.tar.gz"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute (force): %v", err)
	}
}
