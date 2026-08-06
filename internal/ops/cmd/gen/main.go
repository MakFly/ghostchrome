//go:build ignore

// gen writes contracts/commands.json from the canonical ops catalog.
// Invoked via: go generate ./internal/ops/...
// or directly:  go run ./internal/ops/cmd/gen/main.go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"

	"github.com/dev-toolings/ghostchrome/internal/ops"
)

func main() {
	catalog := ops.Catalog()

	// Ensure deterministic order (Catalog already returns sorted, but guard anyway).
	sort.Slice(catalog, func(i, j int) bool {
		return catalog[i].Name < catalog[j].Name
	})

	data, err := json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "gen: marshal: %v\n", err)
		os.Exit(1)
	}
	// Append trailing newline for VCS hygiene.
	data = append(data, '\n')

	// Resolve the contracts/ directory relative to this source file so the
	// generator works regardless of the caller's working directory.
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		fmt.Fprintf(os.Stderr, "gen: cannot resolve source path\n")
		os.Exit(1)
	}
	// file = .../internal/ops/cmd/gen/main.go
	// contracts/ is 4 levels up (gen/ → cmd/ → ops/ → internal/ → repo root)
	root := filepath.Join(filepath.Dir(file), "..", "..", "..", "..")
	out := filepath.Join(root, "contracts", "commands.json")

	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "gen: mkdir: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(out, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "gen: write %s: %v\n", out, err)
		os.Exit(1)
	}
	fmt.Printf("gen: wrote %s (%d ops)\n", out, len(catalog))
}
