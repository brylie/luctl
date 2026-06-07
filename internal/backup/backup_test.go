package backup

import (
	"testing"

	"github.com/brylie/luctl/internal/project"
)

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
