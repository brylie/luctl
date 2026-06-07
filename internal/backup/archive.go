package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// createArchive writes a tar.gz of all paths in sources into w.
// Each source path is walked recursively; files are stored relative to their parent directory.
func createArchive(ctx context.Context, w io.Writer, sources []string) error {
	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)

	for _, src := range sources {
		info, err := os.Stat(src)
		if err != nil {
			return fmt.Errorf("stat %s: %w", src, err)
		}

		// base is the directory to strip so archive paths stay relative.
		base := src
		if !info.IsDir() {
			base = filepath.Dir(src)
		}

		if err := walkAndAdd(ctx, tw, src, base); err != nil {
			return err
		}
	}

	if err := tw.Close(); err != nil {
		return fmt.Errorf("closing tar writer: %w", err)
	}

	if err := gz.Close(); err != nil {
		return fmt.Errorf("closing gzip writer: %w", err)
	}

	return nil
}

// isSQLiteAuxFile reports whether name is a SQLite auxiliary file that must be excluded
// from a backup. Auxiliary files (-journal, -wal, -shm) are transient and not needed
// when the databases are captured via VACUUM INTO.
func isSQLiteAuxFile(name string) bool {
	return strings.HasSuffix(name, "-journal") ||
		strings.HasSuffix(name, "-wal") ||
		strings.HasSuffix(name, "-shm")
}

// addSQLiteEntry snapshots a live SQLite database via VACUUM INTO and writes the
// consistent copy into the tar archive under relPath.
func addSQLiteEntry(ctx context.Context, tw *tar.Writer, srcPath, relPath string, info os.FileInfo) error {
	dir, err := os.MkdirTemp("", "luctl-sqlite-*")
	if err != nil {
		return fmt.Errorf("temp dir for sqlite backup: %w", err)
	}

	defer func() { _ = os.RemoveAll(dir) }()

	tmpPath := filepath.Join(dir, "backup.db")

	if err := sqliteBackup(ctx, srcPath, tmpPath); err != nil {
		return err
	}

	tmpInfo, err := os.Stat(tmpPath)
	if err != nil {
		return fmt.Errorf("stat sqlite backup: %w", err)
	}

	hdr, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return fmt.Errorf("creating tar header for %s: %w", relPath, err)
	}

	hdr.Name = relPath
	hdr.Size = tmpInfo.Size()

	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("writing tar header for %s: %w", relPath, err)
	}

	f, err := os.Open(tmpPath)
	if err != nil {
		return fmt.Errorf("opening sqlite backup: %w", err)
	}

	defer func() { _ = f.Close() }()

	if _, err := io.Copy(tw, f); err != nil {
		return fmt.Errorf("writing sqlite backup to archive: %w", err)
	}

	return nil
}

// addRegularEntry writes a non-SQLite regular file directly into the tar archive.
func addRegularEntry(tw *tar.Writer, path string, hdr *tar.Header, info os.FileInfo) error {
	hdr.Size = info.Size()

	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("writing tar header for %s: %w", path, err)
	}

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening %s: %w", path, err)
	}

	defer func() { _ = f.Close() }()

	if _, err := io.Copy(tw, f); err != nil {
		return fmt.Errorf("writing %s to archive: %w", path, err)
	}

	return nil
}

// walkAndAdd adds src (file or directory tree) to tw, stripping base from all paths.
// SQLite databases are captured via VACUUM INTO for a consistent live snapshot;
// their auxiliary files (-journal, -wal, -shm) are skipped.
func walkAndAdd(ctx context.Context, tw *tar.Writer, src, base string) error {
	err := filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if isSQLiteAuxFile(filepath.Base(path)) {
			return nil
		}

		rel, err := filepath.Rel(base, path)
		if err != nil {
			return fmt.Errorf("computing relative path: %w", err)
		}

		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("creating tar header for %s: %w", path, err)
		}

		hdr.Name = rel

		if info.IsDir() {
			hdr.Name += "/"

			return tw.WriteHeader(hdr)
		}

		if filepath.Ext(path) == ".sqlite" {
			return addSQLiteEntry(ctx, tw, path, rel, info)
		}

		return addRegularEntry(tw, path, hdr, info)
	})
	if err != nil {
		return fmt.Errorf("walking %s: %w", src, err)
	}

	return nil
}

// extractArchive reads a tar.gz from r and extracts it under destDir.
// All paths are treated as relative to destDir.
func extractArchive(r io.Reader, destDir string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("opening gzip reader: %w", err)
	}

	defer func() { _ = gz.Close() }()

	tw := tar.NewReader(gz)

	for {
		hdr, err := tw.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return fmt.Errorf("reading tar entry: %w", err)
		}

		if err := extractEntry(hdr, tw, destDir); err != nil {
			return err
		}
	}

	return nil
}

// extractEntry handles a single tar entry, writing it to destDir.
func extractEntry(hdr *tar.Header, r io.Reader, destDir string) error {
	// Guard against path traversal by cleaning the joined path.
	target := filepath.Join(destDir, filepath.Clean("/"+hdr.Name))
	if target == destDir {
		return nil
	}

	switch hdr.Typeflag {
	case tar.TypeDir:
		if err := os.MkdirAll(target, 0o750); err != nil {
			return fmt.Errorf("mkdir %s: %w", target, err)
		}

	case tar.TypeReg:
		return writeRegularFile(target, hdr, r)
	}

	return nil
}

// writeRegularFile creates target and copies r into it using the mode from hdr.
func writeRegularFile(target string, hdr *tar.Header, r io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return fmt.Errorf("mkdir parent %s: %w", target, err)
	}

	f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, hdr.FileInfo().Mode())
	if err != nil {
		return fmt.Errorf("creating %s: %w", target, err)
	}

	defer func() { _ = f.Close() }()

	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("writing %s: %w", target, err)
	}

	return nil
}
