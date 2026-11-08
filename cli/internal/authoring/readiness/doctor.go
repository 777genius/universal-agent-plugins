package readiness

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packageview"
)

// Check is an allowlisted static observation. Pass applies only to the named
// metadata check, never execution, dependency validity, or toolchain readiness.
type Check struct {
	ID     string `json:"id"`
	ItemID string `json:"item_id,omitempty"`
	Status string `json:"status"`
	Action string `json:"action"`
}

func itemID(value string) string { return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(value))) }

func Doctor(p project.Result) []Check {
	checks := []Check{}
	if p.Facts.Package == nil {
		return checks
	}
	entries := make(map[string]packageview.Observation, len(p.Input.Inventory))
	for _, o := range p.Input.Inventory {
		entries[o.Path] = o
	}
	checks = append(checks, languageChecks(entries)...)
	checks = append(checks, hostChecks(p.Facts.Package, entries, p.Input.Coverage.InventoryComplete)...)
	sort.Slice(checks, func(i, j int) bool {
		if checks[i].ID != checks[j].ID {
			return checks[i].ID < checks[j].ID
		}
		return checks[i].ItemID < checks[j].ItemID
	})
	return checks
}

// resolve traverses retained observations, including contained links, without
// lexical cleaning away intermediates or opening any source/host path.
func resolve(entries map[string]packageview.Observation, value string, complete bool) (packageview.Observation, string) {
	unknown := func() (packageview.Observation, string) { return packageview.Observation{}, "not_evaluated" }
	if strings.HasPrefix(value, "${PLUGIN_DATA}") || strings.Contains(value, `\`) {
		return unknown()
	}
	value = strings.TrimPrefix(value, "${PLUGIN_ROOT}/")
	if strings.HasPrefix(value, "/") || strings.Contains(value, "${") {
		return unknown()
	}
	todo := strings.Split(value, "/")
	var stack []string
	var last packageview.Observation
	links := 0
	for len(todo) > 0 {
		part := todo[0]
		todo = todo[1:]
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			if len(stack) == 0 {
				return packageview.Observation{}, "fail"
			}
			stack = stack[:len(stack)-1]
			continue
		}
		key := strings.Join(append(append([]string{}, stack...), part), "/")
		o, ok := entries[key]
		if !ok {
			if complete {
				return o, "fail"
			}
			return unknown()
		}
		if !o.Captured || o.State != packageview.Present {
			return unknown()
		}
		if o.Kind == "symlink" {
			links++
			if links > 40 || strings.HasPrefix(o.Target, "/") || strings.Contains(o.Target, `\`) {
				return unknown()
			}
			todo = append(strings.Split(o.Target, "/"), todo...)
			continue
		}
		if len(todo) > 0 && o.Kind != "directory" {
			return o, "fail"
		}
		stack = append(stack, part)
		last = o
	}
	if last.Kind != "file" {
		return last, "fail"
	}
	return last, "pass"
}
