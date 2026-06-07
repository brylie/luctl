package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/brylie/luctl/internal/backup"
	"github.com/brylie/luctl/internal/project"
	"github.com/spf13/cobra"
)

func newServerBackupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Create or list server backups stored in S3-compatible storage.",
	}

	cmd.AddCommand(newServerBackupCreateCmd())
	cmd.AddCommand(newServerBackupListCmd())

	return cmd
}

func newServerBackupCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create",
		Short: "Archive the world directory and minetest.conf, then upload to S3.",
		Long: `Creates a timestamped tar.gz backup of:
  - The world directory (paths.world_dir in luanti.toml)
  - The server config file (paths.conf_file in luanti.toml)

The backup is uploaded to the S3-compatible bucket configured under [backup]
in luanti.toml. Credentials are read from two environment variables:

  LUCTL_S3_ACCESS_KEY   your access key / spaces key
  LUCTL_S3_SECRET_KEY   your secret key / spaces secret

Set them safely to avoid storing secrets in shell history:

  source .env            # recommended — see .env.example for safe setup
  read -rs LUCTL_S3_ACCESS_KEY && export LUCTL_S3_ACCESS_KEY  # interactive`,
		RunE: runServerBackupCreate,
	}
}

func runServerBackupCreate(cmd *cobra.Command, _ []string) error {
	p, err := project.LoadCurrent()
	if err != nil {
		return err
	}

	sources, err := backupSources(p)
	if err != nil {
		return err
	}

	client, err := backup.NewS3Client(p.Backup)
	if err != nil {
		return err
	}

	ctx := context.Background()

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Creating backup archive…")

	key, err := backup.Create(ctx, client, p.Backup, sources)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Backup uploaded: s3://%s/%s\n", p.Backup.Bucket, key)

	return nil
}

// backupSources returns the paths that should be included in a backup,
// validating that each path exists.
func backupSources(p *project.Project) ([]string, error) {
	candidates := []struct {
		label string
		path  string
	}{
		{"world_dir", p.Paths.WorldDir},
		{"conf_file", p.Paths.ConfFile},
	}

	var sources []string

	for _, c := range candidates {
		if c.path == "" {
			continue
		}

		if _, err := os.Stat(c.path); err != nil {
			return nil, fmt.Errorf("paths.%s %q not found: %w", c.label, c.path, err)
		}

		sources = append(sources, c.path)
	}

	if len(sources) == 0 {
		return nil, errors.New("no backup sources configured — set paths.world_dir and paths.conf_file in luanti.toml")
	}

	return sources, nil
}

func newServerBackupListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available backups in the configured S3 bucket.",
		RunE:  runServerBackupList,
	}
}

func runServerBackupList(cmd *cobra.Command, _ []string) error {
	p, err := project.LoadCurrent()
	if err != nil {
		return err
	}

	client, err := backup.NewS3Client(p.Backup)
	if err != nil {
		return err
	}

	ctx := context.Background()

	entries, err := backup.List(ctx, client, p.Backup)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No backups found.")

		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "NAME\tSIZE\tDATE")

	for _, e := range entries {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n",
			e.Name,
			backup.FormatSize(e.SizeBytes),
			e.LastModified.UTC().Format(time.RFC3339),
		)
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("flushing table output: %w", err)
	}

	return nil
}
