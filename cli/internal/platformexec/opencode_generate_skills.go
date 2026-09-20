package platformexec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func renderPortableSkills(root string, paths []string, outputRoot string) ([]pluginmodel.Artifact, error) {
	var artifacts []pluginmodel.Artifact
	for _, rel := range paths {
		artifact, err := portableSkillArtifact(root, rel, outputRoot)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, artifact)
	}
	return compactArtifacts(artifacts), nil
}

func portableSkillArtifact(root, rel, outputRoot string) (pluginmodel.Artifact, error) {
	body, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return pluginmodel.Artifact{}, err
	}
	child, err := portableSkillChildPath(rel)
	if err != nil {
		return pluginmodel.Artifact{}, err
	}
	return pluginmodel.Artifact{
		RelPath: filepath.ToSlash(filepath.Join(outputRoot, child)),
		Content: body,
	}, nil
}

func portableSkillChildPath(rel string) (string, error) {
	normalizedRel := normalizePortableSkillPath(rel)
	child, err := filepath.Rel(filepath.FromSlash("skills"), filepath.FromSlash(normalizedRel))
	if err != nil {
		return "", err
	}
	if child == "." || strings.HasPrefix(child, ".."+string(filepath.Separator)) || child == ".." {
		return "", fmt.Errorf("portable skill path %s must live under skills/", rel)
	}
	return child, nil
}

func normalizePortableSkillPath(rel string) string {
	normalizedRel := filepath.ToSlash(rel)
	switch {
	case strings.HasPrefix(normalizedRel, pluginmodel.SourceDirName+"/skills/"):
		return strings.TrimPrefix(normalizedRel, pluginmodel.SourceDirName+"/")
	case normalizedRel == pluginmodel.SourceDirName+"/skills":
		return "skills"
	default:
		return normalizedRel
	}
}
