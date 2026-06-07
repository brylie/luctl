package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// makeTestDB creates a minimal valid SQLite database at path.
func makeTestDB(t *testing.T, path string) {
	t.Helper()

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	defer func() { _ = db.Close() }()

	if _, err := db.ExecContext(context.Background(), "CREATE TABLE t (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
}

// archiveNames opens a tar.gz from r and returns all entry names.
func archiveNames(t *testing.T, r io.Reader) map[string]bool {
	t.Helper()

	gz, err := gzip.NewReader(r)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}

	defer func() { _ = gz.Close() }()

	names := make(map[string]bool)
	tr := tar.NewReader(gz)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			t.Fatalf("tar.Next: %v", err)
		}

		names[hdr.Name] = true
	}

	return names
}

// ---------------------------------------------------------------------------
// isSQLiteAuxFile
// ---------------------------------------------------------------------------

func TestIsSQLiteAuxFile(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"map.sqlite-journal", true},
		{"mod_storage.sqlite-wal", true},
		{"players.sqlite-shm", true},
		{"map.sqlite", false},
		{"world.mt", false},
		{"minetest.conf", false},
		{"-journal", true},
	}

	for _, c := range cases {
		if got := isSQLiteAuxFile(c.name); got != c.want {
			t.Errorf("isSQLiteAuxFile(%q): want %v, got %v", c.name, c.want, got)
		}
	}
}

// ---------------------------------------------------------------------------
// createArchive / extractArchive round-trip
// ---------------------------------------------------------------------------

func TestCreateExtractRoundtrip(t *testing.T) {
	src := t.TempDir()

	if err := os.WriteFile(filepath.Join(src, "world.mt"), []byte("game_id = minetest\n"), 0o600); err != nil {
		t.Fatalf("WriteFile world.mt: %v", err)
	}

	makeTestDB(t, filepath.Join(src, "map.sqlite"))

	var buf bytes.Buffer

	if err := createArchive(context.Background(), &buf, []string{src}); err != nil {
		t.Fatalf("createArchive: %v", err)
	}

	names := archiveNames(t, bytes.NewReader(buf.Bytes()))

	if !names["world.mt"] {
		t.Error("world.mt should be in archive")
	}

	if !names["map.sqlite"] {
		t.Error("map.sqlite should be in archive")
	}

	for name := range names {
		if isSQLiteAuxFile(name) {
			t.Errorf("auxiliary file %q must not be in archive", name)
		}
	}

	// Extract and verify each file is present.
	dst := t.TempDir()

	if err := extractArchive(bytes.NewReader(buf.Bytes()), dst); err != nil {
		t.Fatalf("extractArchive: %v", err)
	}

	for _, want := range []string{"world.mt", "map.sqlite"} {
		if _, err := os.Stat(filepath.Join(dst, want)); err != nil {
			t.Errorf("%s not found after extract: %v", want, err)
		}
	}
}

// TestCreateArchive_ExcludesAuxFiles verifies that files with -journal, -wal,
// and -shm suffixes are excluded. The aux files are created without a
// corresponding SQLite database so that the SQLite engine does not interfere.
func TestCreateArchive_ExcludesAuxFiles(t *testing.T) {
	src := t.TempDir()

	if err := os.WriteFile(filepath.Join(src, "world.mt"), []byte(""), 0o600); err != nil {
		t.Fatalf("WriteFile world.mt: %v", err)
	}

	auxFiles := []string{"orphan.sqlite-journal", "orphan.sqlite-wal", "orphan.sqlite-shm"}

	for _, aux := range auxFiles {
		if err := os.WriteFile(filepath.Join(src, aux), []byte(""), 0o600); err != nil {
			t.Fatalf("WriteFile %s: %v", aux, err)
		}
	}

	var buf bytes.Buffer

	if err := createArchive(context.Background(), &buf, []string{src}); err != nil {
		t.Fatalf("createArchive: %v", err)
	}

	names := archiveNames(t, &buf)

	for _, aux := range auxFiles {
		if names[aux] {
			t.Errorf("auxiliary file %q must be excluded from archive", aux)
		}
	}

	if !names["world.mt"] {
		t.Error("world.mt should be in archive")
	}
}

// ---------------------------------------------------------------------------
// extractArchive — subdirectory handling
// ---------------------------------------------------------------------------

func TestExtractArchive_WithSubdir(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	dirHdr := &tar.Header{Name: "subdir/", Typeflag: tar.TypeDir, Mode: 0o750}
	if err := tw.WriteHeader(dirHdr); err != nil {
		t.Fatalf("WriteHeader dir: %v", err)
	}

	content := []byte("hello")
	fileHdr := &tar.Header{Name: "subdir/file.txt", Typeflag: tar.TypeReg, Size: int64(len(content)), Mode: 0o600}
	if err := tw.WriteHeader(fileHdr); err != nil {
		t.Fatalf("WriteHeader file: %v", err)
	}

	if _, err := tw.Write(content); err != nil {
		t.Fatalf("Write: %v", err)
	}

	_ = tw.Close()
	_ = gz.Close()

	dst := t.TempDir()

	if err := extractArchive(&buf, dst); err != nil {
		t.Fatalf("extractArchive: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dst, "subdir")); err != nil {
		t.Errorf("subdir not created: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dst, "subdir", "file.txt"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if string(data) != "hello" {
		t.Errorf("want hello, got %q", string(data))
	}
}

// ---------------------------------------------------------------------------
// extractArchive — security
// ---------------------------------------------------------------------------

func TestExtractArchive_PathTraversal(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	hdr := &tar.Header{
		Name:     "../escape.txt",
		Typeflag: tar.TypeReg,
		Size:     5,
		Mode:     0o600,
	}

	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("WriteHeader: %v", err)
	}

	if _, err := tw.Write([]byte("evil!")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	_ = tw.Close()
	_ = gz.Close()

	dst := t.TempDir()

	if err := extractArchive(&buf, dst); err != nil {
		t.Fatalf("extractArchive with traversal entry returned unexpected error: %v", err)
	}

	// The file must not appear outside dst.
	escaped := filepath.Join(filepath.Dir(dst), "escape.txt")
	if _, err := os.Stat(escaped); err == nil {
		t.Error("path traversal: file was written outside the destination directory")
	}
}
