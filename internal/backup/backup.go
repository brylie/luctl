package backup

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/brylie/luctl/internal/project"
)

// Entry describes a single backup object in S3.
type Entry struct {
	Name         string
	Key          string
	LastModified time.Time
	SizeBytes    int64
}

// Create archives sources as a tar.gz, uploads it to S3, and returns the object key.
// sources is the list of filesystem paths to include (world dir + conf file).
func Create(ctx context.Context, client *s3.Client, cfg project.BackupConfig, sources []string) (string, error) {
	if err := ValidateConfig(cfg); err != nil {
		return "", err
	}

	name := fmt.Sprintf("backup-%s.tar.gz", time.Now().UTC().Format("2006-01-02T15-04-05Z"))
	key := objectKey(cfg.Prefix, name)

	var buf bytes.Buffer

	if err := createArchive(ctx, &buf, sources); err != nil {
		return "", fmt.Errorf("creating archive: %w", err)
	}

	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(cfg.Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(buf.Bytes()),
		ContentType: aws.String("application/gzip"),
	})
	if err != nil {
		return "", fmt.Errorf("uploading backup to s3://%s/%s: %w", cfg.Bucket, key, err)
	}

	return key, nil
}

// List returns all backup objects stored under the configured prefix.
func List(ctx context.Context, client *s3.Client, cfg project.BackupConfig) ([]Entry, error) {
	if err := ValidateConfig(cfg); err != nil {
		return nil, err
	}

	out, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(cfg.Bucket),
		Prefix: aws.String(cfg.Prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("listing backups in s3://%s/%s: %w", cfg.Bucket, cfg.Prefix, err)
	}

	entries := make([]Entry, 0, len(out.Contents))

	for _, obj := range out.Contents {
		key := aws.ToString(obj.Key)
		name := strings.TrimPrefix(key, cfg.Prefix)

		var mod time.Time
		if obj.LastModified != nil {
			mod = *obj.LastModified
		}

		var size int64
		if obj.Size != nil {
			size = *obj.Size
		}

		entries = append(entries, Entry{
			Name:         name,
			Key:          key,
			LastModified: mod,
			SizeBytes:    size,
		})
	}

	return entries, nil
}

// Restore downloads the backup identified by name (without prefix) and extracts it to destDir.
func Restore(ctx context.Context, client *s3.Client, cfg project.BackupConfig, name, destDir string) error {
	if err := ValidateConfig(cfg); err != nil {
		return err
	}

	key := objectKey(cfg.Prefix, name)

	// Verify the object exists before downloading.
	if _, err := headObject(ctx, client, cfg.Bucket, key); err != nil {
		return fmt.Errorf("backup %q not found: %w", name, err)
	}

	out, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("downloading s3://%s/%s: %w", cfg.Bucket, key, err)
	}
	defer func() { _ = out.Body.Close() }()

	if err := extractArchive(out.Body, destDir); err != nil {
		return fmt.Errorf("extracting backup: %w", err)
	}

	return nil
}

// LatestName returns the name (without prefix) of the most recently modified backup,
// or an error if no backups exist.
func LatestName(ctx context.Context, client *s3.Client, cfg project.BackupConfig) (string, error) {
	entries, err := List(ctx, client, cfg)
	if err != nil {
		return "", err
	}

	if len(entries) == 0 {
		return "", fmt.Errorf("no backups found in s3://%s/%s", cfg.Bucket, cfg.Prefix)
	}

	latest := entries[0]

	for _, e := range entries[1:] {
		if e.LastModified.After(latest.LastModified) {
			latest = e
		}
	}

	return latest.Name, nil
}

// FormatSize returns a human-readable size string (KB / MB / GB).
func FormatSize(b int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)

	switch {
	case b >= gb:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// DownloadToWriter downloads a backup object and writes it to w.
// Exposed for restore-to-stdout use cases.
func DownloadToWriter(ctx context.Context, client *s3.Client, bucket, key string, w io.Writer) error {
	out, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("GetObject s3://%s/%s: %w", bucket, key, err)
	}
	defer func() { _ = out.Body.Close() }()

	if _, err := io.Copy(w, out.Body); err != nil {
		return fmt.Errorf("reading object body: %w", err)
	}

	return nil
}
