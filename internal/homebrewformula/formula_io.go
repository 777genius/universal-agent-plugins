package homebrewformula

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func writeFormula(outputPath string, body []byte) error {
	if strings.TrimSpace(outputPath) == "" {
		return fmt.Errorf("homebrew formula output path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(outputPath, body, 0o644)
}

func parseChecksums(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return nil, fmt.Errorf("invalid checksums.txt line %q", line)
		}
		sum := strings.TrimSpace(fields[0])
		name := strings.TrimPrefix(strings.TrimSpace(fields[len(fields)-1]), "*")
		out[name] = sum
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
