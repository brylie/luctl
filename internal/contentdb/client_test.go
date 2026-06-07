package contentdb

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// buildTestZip creates an in-memory zip with entries in the form:
//
//	files = map[archivePath]content
//
// Every entry is placed under a top-level "pkg-1.0/" prefix that stripTopDir
// is expected to remove, matching ContentDB archive layout.
func buildTestZip(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	for name, content := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatalf("zip.Create(%q): %v", name, err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatalf("zip write(%q): %v", name, err)
		}
	}

	if err := w.Close(); err != nil {
		t.Fatalf("zip.Close: %v", err)
	}
	return buf.Bytes()
}

// withTestServer starts an httptest.Server, overrides BaseURL, and returns the
// server. The caller is responsible for calling ts.Close() (via t.Cleanup).
func withTestServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	orig := BaseURL
	BaseURL = ts.URL
	t.Cleanup(func() { BaseURL = orig })

	return ts
}

// newTestClient creates a Client whose http.Client is pre-configured to trust
// the test server's TLS (for plain HTTP servers this is just a default client).
func newTestClient(ts *httptest.Server) *Client {
	return &Client{http: ts.Client()}
}

// ---------------------------------------------------------------------------
// stripTopDir
// ---------------------------------------------------------------------------

func TestStripTopDir(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"topdir/file.lua", "file.lua"},
		{"topdir/subdir/file.lua", "subdir/file.lua"},
		{"topdir/", ""},
		{"noslash", ""},
		{"a/b/c/d", "b/c/d"},
	}

	for _, tc := range cases {
		got := stripTopDir(tc.input)
		if got != tc.want {
			t.Errorf("stripTopDir(%q): want %q, got %q", tc.input, tc.want, got)
		}
	}
}

// ---------------------------------------------------------------------------
// Search
// ---------------------------------------------------------------------------

func TestSearch_ArrayResponse(t *testing.T) {
	packages := []Package{
		{Author: "alice", Name: "mod1", Title: "Mod One", Type: "mod"},
		{Author: "bob", Name: "mod2", Title: "Mod Two", Type: "mod"},
	}

	ts := withTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(packages)
	}))
	client := newTestClient(ts)

	results, err := client.Search(context.Background(), "mod", "mod", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("want 2 results, got %d", len(results))
	}
	if results[0].Author != "alice" {
		t.Errorf("first result author: want alice, got %q", results[0].Author)
	}
}

func TestSearch_PaginatedResponse(t *testing.T) {
	resp := PackageListResponse{
		Page: 1, PerPage: 10, PageCount: 1, Total: 1,
		Items: []Package{{Author: "alice", Name: "mod1", Type: "mod"}},
	}

	ts := withTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	client := newTestClient(ts)

	results, err := client.Search(context.Background(), "mod", "", 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("want 1 result, got %d", len(results))
	}
}

func TestSearch_NonOK(t *testing.T) {
	ts := withTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	client := newTestClient(ts)

	_, err := client.Search(context.Background(), "x", "", 0)
	if err == nil {
		t.Error("expected error for non-200 response, got nil")
	}
}

// ---------------------------------------------------------------------------
// GetPackage
// ---------------------------------------------------------------------------

func TestGetPackage_OK(t *testing.T) {
	pkg := Package{Author: "alice", Name: "mod1", Title: "Mod One", Type: "mod", Downloads: 42}

	ts := withTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(pkg)
	}))
	client := newTestClient(ts)

	result, err := client.GetPackage(context.Background(), "alice", "mod1")
	if err != nil {
		t.Fatalf("GetPackage: %v", err)
	}
	if result.Author != "alice" {
		t.Errorf("author: want alice, got %q", result.Author)
	}
	if result.Downloads != 42 {
		t.Errorf("downloads: want 42, got %d", result.Downloads)
	}
}

func TestGetPackage_NotFound(t *testing.T) {
	ts := withTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	client := newTestClient(ts)

	_, err := client.GetPackage(context.Background(), "alice", "nonexistent")
	if err == nil {
		t.Error("expected error for 404, got nil")
	}
}

func TestGetPackage_NonOK(t *testing.T) {
	ts := withTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	client := newTestClient(ts)

	_, err := client.GetPackage(context.Background(), "alice", "mod1")
	if err == nil {
		t.Error("expected error for non-200 response, got nil")
	}
}

// ---------------------------------------------------------------------------
// GetReleases
// ---------------------------------------------------------------------------

func TestGetReleases_OK(t *testing.T) {
	releases := []Release{
		{ID: 1, Name: "v1.0", Title: "Version 1.0"},
		{ID: 2, Name: "v2.0", Title: "Version 2.0"},
	}

	ts := withTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(releases)
	}))
	client := newTestClient(ts)

	results, err := client.GetReleases(context.Background(), "alice", "mod1")
	if err != nil {
		t.Fatalf("GetReleases: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("want 2 releases, got %d", len(results))
	}
	if results[0].Name != "v1.0" {
		t.Errorf("release[0].Name: want v1.0, got %q", results[0].Name)
	}
}

func TestGetReleases_NonOK(t *testing.T) {
	ts := withTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	client := newTestClient(ts)

	_, err := client.GetReleases(context.Background(), "alice", "mod1")
	if err == nil {
		t.Error("expected error for non-200, got nil")
	}
}

// ---------------------------------------------------------------------------
// GetDependencies
// ---------------------------------------------------------------------------

func TestGetDependencies_OK(t *testing.T) {
	deps := DependenciesResponse{
		"alice/mod1": {
			{IsOptional: false, Name: "default", Packages: []string{"mt-mods/default"}},
		},
	}

	ts := withTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(deps)
	}))
	client := newTestClient(ts)

	result, err := client.GetDependencies(context.Background(), "alice", "mod1")
	if err != nil {
		t.Fatalf("GetDependencies: %v", err)
	}
	if _, ok := result["alice/mod1"]; !ok {
		t.Error("expected alice/mod1 key in response")
	}
	if result["alice/mod1"][0].Name != "default" {
		t.Errorf("dep name: want default, got %q", result["alice/mod1"][0].Name)
	}
}

func TestGetDependencies_NonOK(t *testing.T) {
	ts := withTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	client := newTestClient(ts)

	_, err := client.GetDependencies(context.Background(), "alice", "mod1")
	if err == nil {
		t.Error("expected error for non-200, got nil")
	}
}

// ---------------------------------------------------------------------------
// Install
// ---------------------------------------------------------------------------

func TestInstall_OK(t *testing.T) {
	// Build a zip with ContentDB's top-dir layout: "mod1-v1.0/init.lua"
	zipData := buildTestZip(t, map[string]string{
		"mod1-v1.0/init.lua":       "-- main",
		"mod1-v1.0/README.md":      "readme",
		"mod1-v1.0/sub/helper.lua": "-- helper",
	})

	ts := withTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(zipData)
	}))
	client := newTestClient(ts)

	destDir := t.TempDir()
	outDir, err := client.Install(context.Background(), "alice", "mod1", destDir)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	// Output directory should be destDir/mod1
	expectedDir := filepath.Join(destDir, "mod1")
	if outDir != expectedDir {
		t.Errorf("outDir: want %q, got %q", expectedDir, outDir)
	}

	// Files should have been extracted (top-dir prefix stripped).
	if _, err := os.Stat(filepath.Join(expectedDir, "init.lua")); err != nil {
		t.Errorf("init.lua should exist after extraction: %v", err)
	}
	if _, err := os.Stat(filepath.Join(expectedDir, "sub", "helper.lua")); err != nil {
		t.Errorf("sub/helper.lua should exist after extraction: %v", err)
	}
}

func TestInstall_BadStatus(t *testing.T) {
	ts := withTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	client := newTestClient(ts)

	_, err := client.Install(context.Background(), "alice", "mod1", t.TempDir())
	if err == nil {
		t.Error("expected error for non-200 download response, got nil")
	}
}

// ---------------------------------------------------------------------------
// unzip / extractEntry  (filesystem-level, no HTTP)
// ---------------------------------------------------------------------------

func TestUnzip_Basic(t *testing.T) {
	zipData := buildTestZip(t, map[string]string{
		"topdir/init.lua":       "-- mod",
		"topdir/sub/helper.lua": "-- help",
	})

	// Write zip to a temp file.
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "test.zip")
	if err := os.WriteFile(zipPath, zipData, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	destDir := t.TempDir()
	outDir, err := unzip(zipPath, destDir, "mymod")
	if err != nil {
		t.Fatalf("unzip: %v", err)
	}

	if _, err := os.Stat(filepath.Join(outDir, "init.lua")); err != nil {
		t.Errorf("init.lua should be extracted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "sub", "helper.lua")); err != nil {
		t.Errorf("sub/helper.lua should be extracted: %v", err)
	}
}

func TestUnzip_PathTraversal(t *testing.T) {
	// Craft a zip with a path-traversal entry.
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.Create("topdir/../../../etc/passwd")
	if err != nil {
		t.Fatalf("zip.Create: %v", err)
	}
	_, _ = f.Write([]byte("malicious"))
	_ = w.Close()

	dir := t.TempDir()
	zipPath := filepath.Join(dir, "evil.zip")
	if err := os.WriteFile(zipPath, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err = unzip(zipPath, dir, "mymod")
	if err == nil {
		t.Error("expected path traversal error, got nil")
	}
}

// ---------------------------------------------------------------------------
// New
// ---------------------------------------------------------------------------

func TestNew(t *testing.T) {
	c := New()
	if c == nil {
		t.Fatal("New() should return non-nil client")
	}
	if c.http == nil {
		t.Error("New() should initialize the http.Client field")
	}
}

// ---------------------------------------------------------------------------
// extractEntry — directory case
// ---------------------------------------------------------------------------

func TestExtractEntry_Directory(t *testing.T) {
	// Build a zip that contains a directory entry after the top-dir prefix.
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	header := &zip.FileHeader{
		Name:   "topdir/subdir/",
		Method: zip.Store,
	}
	header.SetMode(os.ModeDir | 0o750)
	if _, err := w.CreateHeader(header); err != nil {
		t.Fatalf("CreateHeader: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("zip.Close: %v", err)
	}

	dir := t.TempDir()
	zipPath := filepath.Join(dir, "test.zip")
	if err := os.WriteFile(zipPath, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	outDir := t.TempDir()
	modDir, err := unzip(zipPath, outDir, "mymod")
	if err != nil {
		t.Fatalf("unzip: %v", err)
	}

	// "subdir" should have been created inside the mod output directory.
	if _, err := os.Stat(filepath.Join(modDir, "subdir")); err != nil {
		t.Errorf("subdir should be created after extracting directory entry: %v", err)
	}
}
