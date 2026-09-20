package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type devSnapshot map[string]devFileState

type devFileState struct {
	Size    int64
	Mode    os.FileMode
	ModTime int64
}

func devCycleHeader(cycle int, trigger string, changed []string) string {
	label := fmt.Sprintf("Cycle %d [%s]", cycle, trigger)
	if len(changed) == 0 {
		return label
	}
	if len(changed) <= 5 {
		return label + " change(s): " + strings.Join(changed, ", ")
	}
	return label + " change(s): " + strings.Join(changed[:5], ", ") + fmt.Sprintf(" (+%d more)", len(changed)-5)
}

func takeDevSnapshot(root string) (devSnapshot, error) {
	out := make(devSnapshot)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if info.IsDir() {
			if shouldSkipDevDir(rel) {
				return filepath.SkipDir
			}
			return nil
		}
		out[rel] = devFileState{
			Size:    info.Size(),
			Mode:    info.Mode(),
			ModTime: info.ModTime().UnixNano(),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func shouldSkipDevDir(rel string) bool {
	base := filepath.Base(rel)
	switch base {
	case ".git", "node_modules", ".venv", "__pycache__":
		return true
	default:
		return false
	}
}

func devSnapshotChanges(prev, next devSnapshot) []string {
	set := map[string]struct{}{}
	for path, state := range next {
		if old, ok := prev[path]; !ok || old != state {
			set[path] = struct{}{}
		}
	}
	for path := range prev {
		if _, ok := next[path]; !ok {
			set[path] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for path := range set {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

func singleLinePreview(text string) string {
	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, "\n", "\\n")
	if len(text) > 160 {
		return text[:160] + "..."
	}
	return text
}
