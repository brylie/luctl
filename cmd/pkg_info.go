package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newPkgInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info <author/name>",
		Short: "Show details for a ContentDB package.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			parts := strings.SplitN(args[0], "/", 2)
			if len(parts) != 2 {
				return errors.New("argument must be in the form author/name")
			}

			author, name := parts[0], parts[1]
			client := newClient()

			pkg, err := client.GetPackage(cmd.Context(), author, name)
			if err != nil {
				return fmt.Errorf("fetching package info: %w", err)
			}

			fmt.Printf("Title:       %s\n", pkg.Title)
			fmt.Printf("Author:      %s\n", pkg.Author)
			fmt.Printf("Name:        %s\n", pkg.Name)
			fmt.Printf("Type:        %s\n", pkg.Type)
			fmt.Printf("License:     %s\n", pkg.License)
			fmt.Printf("Downloads:   %d\n", pkg.Downloads)
			fmt.Printf("Score:       %.1f\n", pkg.Score)

			if pkg.Repo != "" {
				fmt.Printf("Repository:  %s\n", pkg.Repo)
			}

			if len(pkg.Tags) > 0 {
				fmt.Printf("Tags:        %s\n", strings.Join(pkg.Tags, ", "))
			}

			fmt.Printf("\n%s\n", pkg.ShortDescription)

			return nil
		},
	}
}
