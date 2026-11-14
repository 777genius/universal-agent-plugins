package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const packageDir = "install/integrationctl/agentplugins/internal/archtest"

func main() {
	update := flag.Bool("update", false, "rewrite the committed ClientID budget baseline")
	flag.Parse()

	root, err := repoRoot()
	if err != nil {
		fail(err)
	}
	counts, err := clientIDBudget(root)
	if err != nil {
		fail(err)
	}
	if *update {
		if err := writeBudget(root, counts); err != nil {
			fail(err)
		}
	}
	for _, name := range sortedKeys(counts) {
		fmt.Printf("%6d  %s\n", counts[name], name)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "archtest:", err)
	os.Exit(1)
}

func budgetPath(root string) string {
	return filepath.Join(root, filepath.FromSlash(packageDir), filepath.FromSlash(budgetFile))
}

func writeBudget(root string, counts map[string]int) error {
	encoded, err := json.MarshalIndent(budget{Packages: counts}, "", "  ")
	if err != nil {
		return err
	}
	path := budgetPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(encoded, '\n'), 0o644)
}

func readBudget(root string) (map[string]int, error) {
	contents, err := os.ReadFile(budgetPath(root))
	if err != nil {
		return nil, err
	}
	var baseline budget
	if err := json.Unmarshal(contents, &baseline); err != nil {
		return nil, err
	}
	return baseline.Packages, nil
}

func sortedKeys(counts map[string]int) []string {
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
