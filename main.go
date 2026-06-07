// Command luctl is a CLI for managing a Luanti co-op server and its packages.
package main

import (
	"fmt"
	"os"

	"github.com/brylie/luctl/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
