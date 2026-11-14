package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	required := []string{
		filepath.Join("skills", "lint-repo", "SKILL.md"),
		filepath.Join("generated", "skills", "claude", "lint-repo", "SKILL.md"),
		filepath.Join("generated", "skills", "codex", "lint-repo", "SKILL.md"),
		filepath.Join("commands", "lint-repo.md"),
	}
	var missing []string
	for _, rel := range required {
		if _, err := os.Stat(rel); err != nil {
			missing = append(missing, rel)
		}
	}
	if len(missing) > 0 {
		for _, rel := range missing {
			fmt.Fprintf(os.Stderr, "lint-repo: missing %s\n", rel)
		}
		os.Exit(1)
	}
	fmt.Fprintln(os.Stdout, "lint-repo: example skill package looks internally consistent")
}
