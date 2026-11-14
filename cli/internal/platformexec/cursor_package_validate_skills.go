package platformexec

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func validateCursorPortableSkillsContract(root string, manifest map[string]any, graph pluginmodel.PackageGraph) []Diagnostic {
	paths := graph.Portable.Paths("skills")
	if len(paths) == 0 {
		return validateCursorManifestWithoutPortableSkills(manifest)
	}
	var diagnostics []Diagnostic
	diagnostics = append(diagnostics, validateCursorSkillsRef(manifest)...)
	for _, rel := range paths {
		diagnostics = append(diagnostics, validateCursorPortableSkillProjection(root, rel)...)
	}
	return diagnostics
}

func validateCursorManifestWithoutPortableSkills(manifest map[string]any) []Diagnostic {
	if _, ok := manifest["skills"]; !ok {
		return nil
	}
	return []Diagnostic{cursorPluginManifestDiagnostic(
		CodeGeneratedContractInvalid,
		fmt.Sprintf("Cursor plugin manifest %s may not define skills when portable skills are absent", cursorPluginManifestPath),
	)}
}

func validateCursorSkillsRef(manifest map[string]any) []Diagnostic {
	ref, ok := manifest["skills"]
	if !ok {
		return []Diagnostic{cursorPluginManifestDiagnostic(
			CodeGeneratedContractInvalid,
			fmt.Sprintf("Cursor plugin manifest %s must reference %q when portable skills are authored", cursorPluginManifestPath, cursorPluginSkillsRef),
		)}
	}
	refText, ok := ref.(string)
	if !ok {
		return []Diagnostic{cursorPluginManifestDiagnostic(
			CodeGeneratedContractInvalid,
			fmt.Sprintf("Cursor plugin manifest %s field %q must be the string %q", cursorPluginManifestPath, "skills", cursorPluginSkillsRef),
		)}
	}
	if strings.TrimSpace(refText) == cursorPluginSkillsRef {
		return nil
	}
	return []Diagnostic{cursorPluginManifestDiagnostic(
		CodeGeneratedContractInvalid,
		fmt.Sprintf("Cursor plugin manifest %s must use %q for skills when portable skills are authored", cursorPluginManifestPath, cursorPluginSkillsRef),
	)}
}

func validateCursorPortableSkillProjection(root string, rel string) []Diagnostic {
	child, err := portableSkillChildPath(rel)
	if err != nil {
		return []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeGeneratedContractInvalid,
			Path:     filepath.ToSlash(rel),
			Target:   "cursor",
			Message:  fmt.Sprintf("Cursor portable skill path %s is invalid: %v", rel, err),
		}}
	}
	renderedRel := filepath.ToSlash(filepath.Join("skills", child))
	sourceBody, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeGeneratedContractInvalid,
			Path:     filepath.ToSlash(rel),
			Target:   "cursor",
			Message:  fmt.Sprintf("Cursor portable skill source %s is not readable: %v", rel, err),
		}}
	}
	renderedBody, err := os.ReadFile(filepath.Join(root, renderedRel))
	if err != nil {
		return []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeGeneratedContractInvalid,
			Path:     renderedRel,
			Target:   "cursor",
			Message:  fmt.Sprintf("Cursor generated skill %s is not readable", renderedRel),
		}}
	}
	if bytes.Equal(sourceBody, renderedBody) {
		return nil
	}
	return []Diagnostic{{
		Severity: SeverityFailure,
		Code:     CodeGeneratedContractInvalid,
		Path:     renderedRel,
		Target:   "cursor",
		Message:  fmt.Sprintf("Cursor generated skill %s does not match authored portable skill %s", renderedRel, rel),
	}}
}
