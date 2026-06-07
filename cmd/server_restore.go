package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/brylie/luctl/internal/backup"
	"github.com/brylie/luctl/internal/project"
	"github.com/spf13/cobra"
)

func newServerRestoreCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore [backup-name]",
		Short: "Download and extract a backup into the world directory.",
		Long: `Restores a backup from S3-compatible storage into the world directory
configured in luanti.toml (paths.world_dir).

WARNING: this overwrites the existing world directory. Back up your current
world before restoring if you may need to recover it.

If backup-name is omitted the most recent backup is used.

Use --force to skip the confirmation prompt (e.g. in scripts or cron jobs).

Credentials are read from two environment variables:

  LUCTL_S3_ACCESS_KEY   your access key / spaces key
  LUCTL_S3_SECRET_KEY   your secret key / spaces secret

Set them safely to avoid storing secrets in shell history:

  source .env            # recommended — see .env.example for safe setup
  read -rs LUCTL_S3_ACCESS_KEY && export LUCTL_S3_ACCESS_KEY  # interactive`,
		Args: cobra.MaximumNArgs(1),
		RunE: runServerRestore,
	}

	cmd.Flags().Bool("force", false, "skip the overwrite confirmation prompt")

	return cmd
}

func runServerRestore(cmd *cobra.Command, args []string) error {
	p, err := project.LoadCurrent()
	if err != nil {
		return err
	}

	if p.Paths.WorldDir == "" {
		return errors.New("paths.world_dir is not set in luanti.toml")
	}

	client, err := backup.NewS3Client(p.Backup)
	if err != nil {
		return err
	}

	ctx := context.Background()

	var name string

	if len(args) == 1 {
		name = args[0]
	} else {
		name, err = backup.LatestName(ctx, client, p.Backup)
		if err != nil {
			return err
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Using latest backup: %s\n", name)
	}

	force, _ := cmd.Flags().GetBool("force")

	if !force {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(),
			"This will overwrite %s with %s. Continue? [y/N] ", p.Paths.WorldDir, name)

		scanner := bufio.NewScanner(cmd.InOrStdin())
		scanner.Scan()

		if strings.ToLower(strings.TrimSpace(scanner.Text())) != "y" {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Restore cancelled.")
			return nil
		}
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Restoring %s → %s\n", name, p.Paths.WorldDir)

	if err := backup.Restore(ctx, client, p.Backup, name, p.Paths.WorldDir); err != nil {
		return err
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Restore complete.")

	return nil
}
