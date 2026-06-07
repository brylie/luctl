package cmd

import (
	"fmt"

	"github.com/brylie/luctl/internal/contentdb"
	"github.com/spf13/cobra"
)

const pkgSearchFmt = "%-30s %-8s %s\n"

func newPkgSearchCmd() *cobra.Command {
	var searchType string
	var searchLimit int

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search ContentDB for packages.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := contentdb.New()

			packages, err := client.Search(cmd.Context(), args[0], searchType, searchLimit)
			if err != nil {
				return fmt.Errorf("searching ContentDB: %w", err)
			}

			if len(packages) == 0 {
				fmt.Println("No packages found.")

				return nil
			}

			fmt.Printf(pkgSearchFmt, "PACKAGE", "TYPE", "DESCRIPTION")
			fmt.Printf(pkgSearchFmt, "-------", "----", "-----------")

			for i := range packages {
				key := fmt.Sprintf("%s/%s", packages[i].Author, packages[i].Name)
				desc := packages[i].ShortDescription
				if len(desc) > 60 {
					desc = desc[:57] + "..."
				}

				fmt.Printf(pkgSearchFmt, key, packages[i].Type, desc)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&searchType, "type", "t", "", "Filter by type: mod, game, txp")
	cmd.Flags().IntVarP(&searchLimit, "limit", "n", 20, "Maximum results to return")

	return cmd
}
