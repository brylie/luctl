package backup

import (
	"errors"
	"testing"

	"github.com/brylie/luctl/internal/project"
)

// ---------------------------------------------------------------------------
// NewS3Client
// ---------------------------------------------------------------------------

func TestNewS3Client_MissingCredentials(t *testing.T) {
	t.Setenv("LUCTL_S3_ACCESS_KEY", "")
	t.Setenv("LUCTL_S3_SECRET_KEY", "")

	cfg := project.BackupConfig{Bucket: "test-bucket", Endpoint: "https://example.com", Region: "us-east-1"}

	_, err := NewS3Client(cfg)
	if !errors.Is(err, ErrMissingCredentials) {
		t.Errorf("want ErrMissingCredentials, got %v", err)
	}
}

func TestNewS3Client_MissingBucket(t *testing.T) {
	t.Setenv("LUCTL_S3_ACCESS_KEY", "key")
	t.Setenv("LUCTL_S3_SECRET_KEY", "secret")

	cfg := project.BackupConfig{Endpoint: "https://example.com"}

	_, err := NewS3Client(cfg)
	if err == nil {
		t.Error("expected error for missing bucket, got nil")
	}
}

func TestNewS3Client_ValidConfig(t *testing.T) {
	t.Setenv("LUCTL_S3_ACCESS_KEY", "test-key")
	t.Setenv("LUCTL_S3_SECRET_KEY", "test-secret")

	cfg := project.BackupConfig{
		Bucket:   "test-bucket",
		Endpoint: "https://example.com",
		Region:   "us-east-1",
	}

	client, err := NewS3Client(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client == nil {
		t.Error("expected non-nil client")
	}
}

func TestNewS3Client_DefaultsRegion(t *testing.T) {
	t.Setenv("LUCTL_S3_ACCESS_KEY", "test-key")
	t.Setenv("LUCTL_S3_SECRET_KEY", "test-secret")

	// Region is intentionally empty; NewS3Client should default to us-east-1.
	cfg := project.BackupConfig{
		Bucket:   "test-bucket",
		Endpoint: "https://example.com",
	}

	client, err := NewS3Client(cfg)
	if err != nil {
		t.Fatalf("unexpected error with empty region: %v", err)
	}

	if client == nil {
		t.Error("expected non-nil client")
	}
}
