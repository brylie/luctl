//go:build ignore

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const threshold = 80.0

func main() {
	goExe := filepath.Join(runtime.GOROOT(), "bin", "go")

	cmd := exec.Command(goExe, "test", "./...", "-coverprofile=coverage.out", "-count=1")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}

	out, err := exec.Command(goExe, "tool", "cover", "-func=coverage.out").Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, "go tool cover:", err)
		os.Exit(1)
	}

	var pct float64

	for _, line := range strings.Split(string(out), "\n") {
		if !strings.HasPrefix(line, "total:") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			break
		}

		pct, err = strconv.ParseFloat(strings.TrimSuffix(fields[2], "%"), 64)
		if err != nil {
			fmt.Fprintln(os.Stderr, "parse coverage:", err)
			os.Exit(1)
		}

		break
	}

	fmt.Printf("Total coverage: %.1f%%\n", pct)

	if pct < threshold {
		fmt.Fprintf(os.Stderr, "Coverage %.1f%% is below the required %.0f%% threshold\n", pct, threshold)
		os.Exit(1)
	}
}
