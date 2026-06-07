package backup

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/brylie/luctl/internal/project"
)

// fakeS3Client creates an S3 client pointed at ts and sets credentials via t.Setenv.
func fakeS3Client(t *testing.T, ts *httptest.Server) *s3.Client {
	t.Helper()
	t.Setenv("LUCTL_S3_ACCESS_KEY", "test-key")
	t.Setenv("LUCTL_S3_SECRET_KEY", "test-secret")

	cfg := project.BackupConfig{
		Bucket:   "test-bucket",
		Endpoint: ts.URL,
		Region:   "us-east-1",
		Prefix:   "luanti/",
	}

	client, err := NewS3Client(cfg)
	if err != nil {
		t.Fatalf("NewS3Client: %v", err)
	}

	return client
}

// fakeListResponse is a minimal ListObjectsV2 XML response with one object.
const fakeListResponse = `<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>test-bucket</Name>
  <IsTruncated>false</IsTruncated>
  <Contents>
    <Key>luanti/backup-2026-06-07T15-33-51Z.tar.gz</Key>
    <LastModified>2026-06-07T15:33:53.000Z</LastModified>
    <Size>12345</Size>
  </Contents>
</ListBucketResult>`

// fakeMultiListResponse is a ListObjectsV2 response with two objects at different timestamps.
const fakeMultiListResponse = `<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>test-bucket</Name>
  <IsTruncated>false</IsTruncated>
  <Contents>
    <Key>luanti/backup-old.tar.gz</Key>
    <LastModified>2026-01-01T00:00:00.000Z</LastModified>
    <Size>100</Size>
  </Contents>
  <Contents>
    <Key>luanti/backup-new.tar.gz</Key>
    <LastModified>2026-06-07T15:33:53.000Z</LastModified>
    <Size>200</Size>
  </Contents>
</ListBucketResult>`

// fakeEmptyListResponse is a ListObjectsV2 response with no objects.
const fakeEmptyListResponse = `<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>test-bucket</Name>
  <IsTruncated>false</IsTruncated>
</ListBucketResult>`

// ---------------------------------------------------------------------------
// objectKey
// ---------------------------------------------------------------------------

func TestObjectKey(t *testing.T) {
	if got := objectKey("luanti/", "backup.tar.gz"); got != "luanti/backup.tar.gz" {
		t.Errorf("want luanti/backup.tar.gz, got %q", got)
	}

	if got := objectKey("", "backup.tar.gz"); got != "backup.tar.gz" {
		t.Errorf("empty prefix: want backup.tar.gz, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// FormatSize
// ---------------------------------------------------------------------------

func TestFormatSize(t *testing.T) {
	cases := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{2048, "2.0 KB"},
		{5 * 1024 * 1024, "5.0 MB"},
		{2 * 1024 * 1024 * 1024, "2.0 GB"},
	}

	for _, c := range cases {
		if got := FormatSize(c.bytes); got != c.want {
			t.Errorf("FormatSize(%d): want %q, got %q", c.bytes, c.want, got)
		}
	}
}

// ---------------------------------------------------------------------------
// ValidateConfig
// ---------------------------------------------------------------------------

func TestValidateConfig_MissingBucket(t *testing.T) {
	cfg := project.BackupConfig{Endpoint: "https://example.com"}

	if err := ValidateConfig(cfg); err == nil {
		t.Error("expected error for missing bucket, got nil")
	}
}

func TestValidateConfig_MissingEndpoint(t *testing.T) {
	cfg := project.BackupConfig{Bucket: "my-bucket"}

	if err := ValidateConfig(cfg); err == nil {
		t.Error("expected error for missing endpoint, got nil")
	}
}

func TestValidateConfig_Valid(t *testing.T) {
	cfg := project.BackupConfig{
		Bucket:   "my-bucket",
		Endpoint: "https://example.com",
	}

	if err := ValidateConfig(cfg); err != nil {
		t.Errorf("expected nil for valid config, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

func TestList(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = fmt.Fprint(w, fakeListResponse)
	}))
	defer ts.Close()

	cfg := project.BackupConfig{Bucket: "test-bucket", Endpoint: ts.URL, Prefix: "luanti/"}
	entries, err := List(context.Background(), fakeS3Client(t, ts), cfg)

	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}

	if entries[0].Name != "backup-2026-06-07T15-33-51Z.tar.gz" {
		t.Errorf("unexpected Name: %q", entries[0].Name)
	}

	if entries[0].SizeBytes != 12345 {
		t.Errorf("want SizeBytes 12345, got %d", entries[0].SizeBytes)
	}
}

func TestList_Empty(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = fmt.Fprint(w, fakeEmptyListResponse)
	}))
	defer ts.Close()

	cfg := project.BackupConfig{Bucket: "test-bucket", Endpoint: ts.URL, Prefix: "luanti/"}
	entries, err := List(context.Background(), fakeS3Client(t, ts), cfg)

	if err != nil {
		t.Fatalf("List on empty bucket: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("want 0 entries, got %d", len(entries))
	}
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func TestCreate(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("ETag", `"abc123"`)
	}))
	defer ts.Close()

	src := t.TempDir()

	if err := os.WriteFile(filepath.Join(src, "world.mt"), []byte("game_id = test\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg := project.BackupConfig{Bucket: "test-bucket", Endpoint: ts.URL, Prefix: "luanti/"}
	key, err := Create(context.Background(), fakeS3Client(t, ts), cfg, []string{src})

	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if !strings.HasPrefix(key, "luanti/backup-") {
		t.Errorf("key should start with luanti/backup-, got %q", key)
	}

	if !strings.HasSuffix(key, ".tar.gz") {
		t.Errorf("key should end with .tar.gz, got %q", key)
	}
}

// ---------------------------------------------------------------------------
// LatestName
// ---------------------------------------------------------------------------

func TestLatestName(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = fmt.Fprint(w, fakeMultiListResponse)
	}))
	defer ts.Close()

	cfg := project.BackupConfig{Bucket: "test-bucket", Endpoint: ts.URL, Prefix: "luanti/"}
	name, err := LatestName(context.Background(), fakeS3Client(t, ts), cfg)

	if err != nil {
		t.Fatalf("LatestName: %v", err)
	}

	if name != "backup-new.tar.gz" {
		t.Errorf("want backup-new.tar.gz, got %q", name)
	}
}

func TestLatestName_Empty(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = fmt.Fprint(w, fakeEmptyListResponse)
	}))
	defer ts.Close()

	cfg := project.BackupConfig{Bucket: "test-bucket", Endpoint: ts.URL, Prefix: "luanti/"}
	_, err := LatestName(context.Background(), fakeS3Client(t, ts), cfg)

	if err == nil {
		t.Error("expected error for empty bucket, got nil")
	}
}

// ---------------------------------------------------------------------------
// headObject
// ---------------------------------------------------------------------------

func TestHeadObject(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		w.Header().Set("ETag", `"abc123"`)
	}))
	defer ts.Close()

	out, err := headObject(context.Background(), fakeS3Client(t, ts), "test-bucket", "luanti/backup.tar.gz")
	if err != nil {
		t.Fatalf("headObject: %v", err)
	}

	if out == nil {
		t.Error("expected non-nil output")
	}
}

// ---------------------------------------------------------------------------
// DownloadToWriter
// ---------------------------------------------------------------------------

func TestDownloadToWriter(t *testing.T) {
	body := []byte("fake archive content")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	defer ts.Close()

	var buf bytes.Buffer

	err := DownloadToWriter(
		context.Background(),
		fakeS3Client(t, ts),
		"test-bucket",
		"luanti/backup.tar.gz",
		&buf,
	)
	if err != nil {
		t.Fatalf("DownloadToWriter: %v", err)
	}

	if buf.String() != string(body) {
		t.Errorf("want %q, got %q", string(body), buf.String())
	}
}
