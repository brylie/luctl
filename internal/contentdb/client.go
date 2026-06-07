// Package contentdb provides a minimal client for the ContentDB REST API.
package contentdb

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// BaseURL is the root of the ContentDB REST API.  Declared as a var so that
// tests can redirect requests to a local httptest.Server.
var BaseURL = "https://content.luanti.org"

const (
	// maxDecompressedSize caps zip extraction to 500 MB to prevent decompression bombs.
	maxDecompressedSize = 500 * 1024 * 1024
	apiErrFmt           = "API error: %s"
)

// Client is a minimal ContentDB API client.
type Client struct {
	http *http.Client
}

// New returns a Client with default settings.
func New() *Client {
	return &Client{http: &http.Client{}}
}

// NewWithClient returns a Client that uses the provided *http.Client.
// Intended for testing so that callers can redirect requests to an
// httptest.Server without changing the package-level BaseURL.
func NewWithClient(c *http.Client) *Client {
	return &Client{http: c}
}

// drainAndClose drains and closes an HTTP response body to allow connection reuse.
func drainAndClose(resp *http.Response) {
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

// doGet performs a GET request with the given context and returns the response.
func (c *Client) doGet(ctx context.Context, endpoint string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}

	return resp, nil
}

// Search queries packages. pkgType may be "mod", "game", "txp", or "" for all.
func (c *Client) Search(ctx context.Context, query, pkgType string, limit int) ([]Package, error) {
	params := url.Values{}
	params.Set("q", query)

	if pkgType != "" {
		params.Set("type", pkgType)
	}

	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}

	resp, err := c.doGet(ctx, fmt.Sprintf("%s/api/packages/?%s", BaseURL, params.Encode()))
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(apiErrFmt, resp.Status)
	}

	// The list endpoint returns a plain JSON array; with pagination it's an object.
	// Try array first, fall back to paginated wrapper.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	var packages []Package
	if err := json.Unmarshal(body, &packages); err == nil {
		return packages, nil
	}

	var paginated PackageListResponse
	if err := json.Unmarshal(body, &paginated); err != nil {
		return nil, fmt.Errorf("unexpected response format: %w", err)
	}

	return paginated.Items, nil
}

// GetPackage fetches metadata for a single package by author/name.
func (c *Client) GetPackage(ctx context.Context, author, name string) (*Package, error) {
	resp, err := c.doGet(ctx, fmt.Sprintf("%s/api/packages/%s/%s/", BaseURL, author, name))
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp)

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("package %s/%s not found", author, name)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(apiErrFmt, resp.Status)
	}

	var pkg Package
	if err := json.NewDecoder(resp.Body).Decode(&pkg); err != nil {
		return nil, fmt.Errorf("decoding package response: %w", err)
	}

	return &pkg, nil
}

// GetReleases returns the release list for a package.
func (c *Client) GetReleases(ctx context.Context, author, name string) ([]Release, error) {
	resp, err := c.doGet(ctx, fmt.Sprintf("%s/api/packages/%s/%s/releases/", BaseURL, author, name))
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(apiErrFmt, resp.Status)
	}

	var releases []Release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("decoding releases response: %w", err)
	}

	return releases, nil
}

// Install downloads the latest release of author/name and unpacks it into destDir.
// Returns the path to the extracted mod directory.
func (c *Client) Install(ctx context.Context, author, name, destDir string) (string, error) {
	downloadURL := fmt.Sprintf("%s/packages/%s/%s/download/", BaseURL, author, name)

	resp, err := c.doGet(ctx, downloadURL)
	if err != nil {
		return "", err
	}
	defer drainAndClose(resp)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: %s", resp.Status)
	}

	tmp, err := os.CreateTemp("", "luctl-mod-*.zip")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}

	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		_ = tmp.Close()

		return "", fmt.Errorf("writing download to temp file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("closing temp file: %w", err)
	}

	return unzip(tmpName, destDir, name)
}

// GetDependencies returns the full transitive dependency graph for a package.
// The map key is "author/name"; the value is that package's dependency list.
func (c *Client) GetDependencies(ctx context.Context, author, name string) (DependenciesResponse, error) {
	resp, err := c.doGet(ctx, fmt.Sprintf("%s/api/packages/%s/%s/dependencies/", BaseURL, author, name))
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(apiErrFmt, resp.Status)
	}

	var deps DependenciesResponse
	if err := json.NewDecoder(resp.Body).Decode(&deps); err != nil {
		return nil, fmt.Errorf("decoding dependencies response: %w", err)
	}

	return deps, nil
}

// unzip extracts a zip archive into destDir/modName, stripping the top-level
// directory that ContentDB adds to every archive.
func unzip(src, destDir, modName string) (string, error) {
	r, err := zip.OpenReader(src)
	if err != nil {
		return "", fmt.Errorf("opening zip archive: %w", err)
	}
	defer func() { _ = r.Close() }()

	outDir := filepath.Join(destDir, modName)
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		return "", fmt.Errorf("creating output directory: %w", err)
	}

	for _, f := range r.File {
		if err := extractEntry(f, outDir); err != nil {
			return "", err
		}
	}

	return outDir, nil
}

// extractEntry extracts a single zip entry into outDir, stripping the
// archive's top-level directory prefix.
func extractEntry(f *zip.File, outDir string) error {
	rel := stripTopDir(filepath.ToSlash(f.Name))
	if rel == "" {
		return nil
	}

	dest := filepath.Join(outDir, filepath.FromSlash(rel))

	// Guard against path traversal.
	if !strings.HasPrefix(dest, filepath.Clean(outDir)+string(os.PathSeparator)) {
		return fmt.Errorf("illegal path in archive: %s", f.Name)
	}

	if f.FileInfo().IsDir() {
		if err := os.MkdirAll(dest, 0o750); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}

		return nil
	}

	return writeZipEntry(f, dest)
}

// writeZipEntry writes the contents of a zip file entry to dest.
func writeZipEntry(f *zip.File, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		return fmt.Errorf("creating parent directory: %w", err)
	}

	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("opening zip entry: %w", err)
	}
	defer func() { _ = rc.Close() }()

	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, io.LimitReader(rc, maxDecompressedSize)); err != nil {
		return fmt.Errorf("extracting file: %w", err)
	}

	return nil
}

// stripTopDir removes the first path component from a zip entry name.
// Returns an empty string if the entry is the top-level directory itself.
func stripTopDir(name string) string {
	_, after, found := strings.Cut(name, "/")
	if !found {
		return ""
	}

	return after
}
