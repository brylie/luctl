package cmd

import (
	"fmt"

	"github.com/brylie/luctl/internal/project"
	"github.com/spf13/cobra"
)

func newProjectFmtCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "fmt",
		Short: "Format and sort luanti.toml in-place.",
		RunE: func(_ *cobra.Command, _ []string) error {
			p, err := project.LoadCurrent()
			if err != nil {
				return fmt.Errorf("loading project: %w", err)
			}

			if err := project.Save(p, project.Filename); err != nil {
				return fmt.Errorf("saving project: %w", err)
			}

			fmt.Println("luanti.toml formatted.")

			return nil
		},
	}
}
