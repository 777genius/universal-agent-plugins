package project

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/dependencymanifest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packageview"
)

// DependencyDiagnostic contains only public-safe manager findings. TOML bytes,
// selectors and parser messages remain private, like standard raw extensions.
type DependencyDiagnostic struct{ Code, Location string }

// dependencyDiagnostics shares the capture used for standard authoring. Missing
// files are optional; unavailable evidence must never assert an empty package.
func dependencyDiagnostics(extensions map[string]json.RawMessage, input packageview.Input, lease *packageview.Lease) []DependencyDiagnostic {
	var diagnostics []DependencyDiagnostic
	if err := dependencymanifest.RejectLegacyDependencies(extensions); err != nil {
		diagnostics = append(diagnostics, DependencyDiagnostic{"dependency_declaration_moved", "plugin.json"})
	}
	body, code := capturedDependencyManifest(input, lease)
	if code != "" {
		return append(diagnostics, DependencyDiagnostic{code, "uap.toml"})
	}
	if body == nil {
		return diagnostics
	}
	doc, err := dependencymanifest.Decode(body, "uap.toml")
	if err == nil {
		_, err = doc.Package()
	}
	if err != nil {
		location := "uap.toml"
		var source *dependencymanifest.Error
		if errors.As(err, &source) && source.Line > 0 {
			location += ":" + strconv.Itoa(source.Line)
		}
		diagnostics = append(diagnostics, DependencyDiagnostic{"dependency_manifest_invalid", location})
	}
	return diagnostics
}

// Resolve contained symlinks only through retained inventory. Content never
// probes the source tree, including when a source changes after Capture.
func capturedDependencyManifest(input packageview.Input, lease *packageview.Lease) ([]byte, string) {
	if _, exists := observation(input, "uap.toml"); !exists {
		if input.Coverage.InventoryComplete {
			return nil, ""
		}
		return nil, "dependency_manifest_unavailable"
	}
	path, code := capturedDependencyPath(input)
	if code != "" {
		return nil, code
	}
	body, ok := lease.Content(path)
	if !ok {
		return nil, "dependency_manifest_unavailable"
	}
	// Keep present empty bytes distinguishable from an absent optional file.
	return append([]byte{}, body...), ""
}

// capturedDependencyPath proves each traversed component before following a
// retained link; cleaning first could erase a missing or non-directory parent.
func capturedDependencyPath(input packageview.Input) (string, string) {
	todo := []string{"uap.toml"}
	var stack []string
	links := 0
	for len(todo) > 0 {
		part := todo[0]
		todo = todo[1:]
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			if len(stack) == 0 {
				return "", "dependency_manifest_unavailable"
			}
			stack = stack[:len(stack)-1]
			continue
		}
		candidate := strings.Join(append(append([]string{}, stack...), part), "/")
		entry, exists := observation(input, candidate)
		if !exists || entry.State != packageview.Present || !entry.Captured {
			return "", "dependency_manifest_unavailable"
		}
		if entry.Kind == "symlink" {
			links++
			if links > 40 || strings.HasPrefix(entry.Target, "/") || strings.Contains(entry.Target, `\`) {
				return "", "dependency_manifest_unavailable"
			}
			todo = append(strings.Split(entry.Target, "/"), todo...)
			continue
		}
		if len(todo) > 0 {
			if entry.Kind != "directory" {
				return "", "dependency_manifest_wrong_kind"
			}
			stack = append(stack, part)
			continue
		}
		if entry.Kind != "file" {
			return "", "dependency_manifest_wrong_kind"
		}
		return candidate, ""
	}
	return "", "dependency_manifest_wrong_kind"
}
