package platformexec

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func validateGeminiGeneratedHooks(root string, graph pluginmodel.PackageGraph, authoredHookPaths []string) []Diagnostic {
	const generatedHooksPath = "hooks/hooks.json"
	body, err := os.ReadFile(filepath.Join(root, generatedHooksPath))
	if len(authoredHookPaths) > 0 {
		return validateGeminiAuthoredGeneratedHooks(root, authoredHookPaths[0], generatedHooksPath, body, err)
	}
	if !geminiUsesGeneratedHooks(graph, pluginmodel.TargetState{Target: "gemini"}) {
		return nil
	}
	if err != nil {
		return []Diagnostic{geminiHookDiagnostic(
			CodeGeneratedContractInvalid,
			generatedHooksPath,
			"Gemini generated hooks/hooks.json is not readable",
		)}
	}
	renderedHooks, parseErr := parseGeminiHooks(body)
	if parseErr != nil {
		return []Diagnostic{geminiHookDiagnostic(
			CodeManifestInvalid,
			generatedHooksPath,
			fmt.Sprintf("Gemini generated hooks file %s is invalid: %v", generatedHooksPath, parseErr),
		)}
	}
	expectedHooks, parseErr := parseGeminiHooks(defaultGeminiHooks(strings.TrimSpace(graph.Launcher.Entrypoint)))
	if parseErr != nil {
		return nil
	}
	if jsonDocumentsEqual(expectedHooks, renderedHooks) {
		return nil
	}
	return []Diagnostic{geminiHookDiagnostic(
		CodeGeneratedContractInvalid,
		generatedHooksPath,
		"Gemini generated hooks/hooks.json does not match the managed launcher-derived hooks projection",
	)}
}

func validateGeminiAuthoredGeneratedHooks(root, authoredHookPath, generatedHooksPath string, body []byte, readErr error) []Diagnostic {
	if readErr != nil {
		return []Diagnostic{geminiHookDiagnostic(
			CodeGeneratedContractInvalid,
			generatedHooksPath,
			"Gemini generated hooks/hooks.json is not readable",
		)}
	}
	renderedHooks, err := parseGeminiHooks(body)
	if err != nil {
		return []Diagnostic{geminiHookDiagnostic(
			CodeManifestInvalid,
			generatedHooksPath,
			fmt.Sprintf("Gemini generated hooks file %s is invalid: %v", generatedHooksPath, err),
		)}
	}
	authoredBody, err := os.ReadFile(filepath.Join(root, authoredHookPath))
	if err != nil {
		return nil
	}
	authoredHooks, err := parseGeminiHooks(authoredBody)
	if err != nil {
		return nil
	}
	if jsonDocumentsEqual(authoredHooks, renderedHooks) {
		return nil
	}
	return []Diagnostic{geminiHookDiagnostic(
		CodeGeneratedContractInvalid,
		generatedHooksPath,
		"Gemini generated hooks/hooks.json does not match authored targets/gemini/hooks/hooks.json",
	)}
}

func validateGeminiHookFiles(root string, rels []string) []Diagnostic {
	var diagnostics []Diagnostic
	for _, rel := range rels {
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			diagnostics = append(diagnostics, geminiHookDiagnostic(
				CodeManifestInvalid,
				rel,
				fmt.Sprintf("Gemini JSON asset %s is not readable: %v", rel, err),
			))
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal(body, &raw); err != nil {
			diagnostics = append(diagnostics, geminiHookDiagnostic(
				CodeManifestInvalid,
				rel,
				fmt.Sprintf("Gemini hooks file %s is invalid JSON: %v", rel, err),
			))
			continue
		}
		if _, ok := raw["hooks"].(map[string]any); ok {
			continue
		}
		diagnostics = append(diagnostics, geminiHookDiagnostic(
			CodeManifestInvalid,
			rel,
			fmt.Sprintf("Gemini hooks file %s must define a top-level hooks object", rel),
		))
	}
	return diagnostics
}
