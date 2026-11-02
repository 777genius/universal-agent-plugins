package platformexec

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func validateQoderPortableSkillsContract(root string, manifest map[string]any, graph pluginmodel.PackageGraph) []Diagnostic {
	paths := graph.Portable.Paths("skills")
	if len(paths) == 0 {
		return validateQoderManifestWithoutPortableSkills(manifest)
	}
	var diagnostics []Diagnostic
	diagnostics = append(diagnostics, validateQoderSkillsRef(manifest)...)
	for _, rel := range paths {
		diagnostics = append(diagnostics, validateQoderPortableSkillProjection(root, rel)...)
	}
	return diagnostics
}

func validateQoderManifestWithoutPortableSkills(manifest map[string]any) []Diagnostic {
	if _, ok := manifest["skills"]; !ok {
		return nil
	}
	return []Diagnostic{qoderPluginManifestDiagnostic(
		CodeGeneratedContractInvalid,
		fmt.Sprintf("Qoder plugin manifest %s may not define skills when portable skills are absent", qoderPluginManifestPath),
	)}
}

func validateQoderSkillsRef(manifest map[string]any) []Diagnostic {
	ref, ok := manifest["skills"]
	if !ok {
		return []Diagnostic{qoderPluginManifestDiagnostic(
			CodeGeneratedContractInvalid,
			fmt.Sprintf("Qoder plugin manifest %s must reference %q when portable skills are authored", qoderPluginManifestPath, qoderPluginSkillsRef),
		)}
	}
	refText, ok := ref.(string)
	if !ok {
		return []Diagnostic{qoderPluginManifestDiagnostic(
			CodeGeneratedContractInvalid,
			fmt.Sprintf("Qoder plugin manifest %s field %q must be the string %q", qoderPluginManifestPath, "skills", qoderPluginSkillsRef),
		)}
	}
	if strings.TrimSpace(refText) == qoderPluginSkillsRef {
		return nil
	}
	return []Diagnostic{qoderPluginManifestDiagnostic(
		CodeGeneratedContractInvalid,
		fmt.Sprintf("Qoder plugin manifest %s must use %q for skills when portable skills are authored", qoderPluginManifestPath, qoderPluginSkillsRef),
	)}
}

func validateQoderPortableSkillProjection(root string, rel string) []Diagnostic {
	child, err := portableSkillChildPath(rel)
	if err != nil {
		return []Diagnostic{qoderSkillDiagnostic(filepath.ToSlash(rel), fmt.Sprintf("Qoder portable skill path %s is invalid: %v", rel, err))}
	}
	renderedRel := filepath.ToSlash(filepath.Join("skills", child))
	sourceBody, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return []Diagnostic{qoderSkillDiagnostic(filepath.ToSlash(rel), fmt.Sprintf("Qoder portable skill source %s is not readable: %v", rel, err))}
	}
	renderedBody, err := os.ReadFile(filepath.Join(root, renderedRel))
	if err != nil {
		return []Diagnostic{qoderSkillDiagnostic(renderedRel, fmt.Sprintf("Qoder generated skill %s is not readable", renderedRel))}
	}
	if bytes.Equal(sourceBody, renderedBody) {
		return nil
	}
	return []Diagnostic{qoderSkillDiagnostic(renderedRel, fmt.Sprintf("Qoder generated skill %s does not match authored portable skill %s", renderedRel, rel))}
}

func qoderSkillDiagnostic(path string, message string) Diagnostic {
	return Diagnostic{
		Severity: SeverityFailure,
		Code:     CodeGeneratedContractInvalid,
		Path:     path,
		Target:   "qoder",
		Message:  message,
	}
}
