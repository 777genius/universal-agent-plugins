package pluginmanifest

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/publicationexec"
)

func generatePackage(root string, target string) (RenderResult, error) {
	ctx, _, err := loadPackageContext(root, target)
	if err != nil {
		return RenderResult{}, err
	}
	artifactMap := map[string][]byte{}
	for _, name := range ctx.selectedTargets {
		generated, err := renderTargetArtifacts(root, ctx.graph, name)
		if err != nil {
			return RenderResult{}, err
		}
		for _, artifact := range generated {
			relPath := filepath.ToSlash(filepath.Clean(artifact.RelPath))
			if existing, ok := artifactMap[relPath]; ok {
				if !bytes.Equal(existing, artifact.Content) {
					return RenderResult{}, fmt.Errorf("conflicting generated artifact %s across targets", relPath)
				}
				continue
			}
			artifactMap[relPath] = artifact.Content
		}
	}
	publicationArtifacts, err := publicationexec.Generate(ctx.graph, ctx.publication, ctx.selectedTargets)
	if err != nil {
		return RenderResult{}, err
	}
	for _, artifact := range publicationArtifacts {
		relPath := filepath.ToSlash(filepath.Clean(artifact.RelPath))
		if existing, ok := artifactMap[relPath]; ok {
			if !bytes.Equal(existing, artifact.Content) {
				return RenderResult{}, fmt.Errorf("conflicting generated artifact %s across publication channels and targets", relPath)
			}
			continue
		}
		artifactMap[relPath] = artifact.Content
	}
	if strings.TrimSpace(ctx.layout.Path("")) != "" {
		if claudeBoundary, err := buildRootClaudeBoundaryArtifact(root, ctx.layout); err != nil {
			return RenderResult{}, err
		} else if claudeBoundary != nil {
			artifactMap[claudeBoundary.RelPath] = claudeBoundary.Content
		}
		if agentsBoundary, err := buildRootAgentsBoundaryArtifact(root, ctx.layout, ctx.graph); err != nil {
			return RenderResult{}, err
		} else if agentsBoundary != nil {
			artifactMap[agentsBoundary.RelPath] = agentsBoundary.Content
		}
		if readme, err := buildRootReadmeArtifact(root, ctx.layout, ctx.graph.Manifest); err != nil {
			return RenderResult{}, err
		} else if readme != nil {
			artifactMap[readme.RelPath] = readme.Content
		}
		if generatedGuide, err := buildRootGeneratedGuideArtifact(root, ctx.layout, ctx.graph, ctx.publication); err != nil {
			return RenderResult{}, err
		} else if generatedGuide != nil {
			artifactMap[generatedGuide.RelPath] = generatedGuide.Content
		}
	}
	artifacts := make([]Artifact, 0, len(artifactMap))
	for path, content := range artifactMap {
		artifacts = append(artifacts, Artifact{RelPath: path, Content: content})
	}
	slices.SortFunc(artifacts, func(a, b Artifact) int { return strings.Compare(a.RelPath, b.RelPath) })

	expected := map[string]struct{}{}
	for _, artifact := range artifacts {
		expected[artifact.RelPath] = struct{}{}
	}
	var stale []string
	for _, path := range expectedManagedPaths(root, ctx.layout, ctx.graph, ctx.publication, ctx.selectedTargets) {
		if _, ok := expected[path]; ok {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, path)); err == nil {
			stale = append(stale, path)
		}
	}
	slices.Sort(stale)
	return RenderResult{Artifacts: artifacts, StalePaths: stale}, nil
}

func driftPackage(root string, target string) ([]string, error) {
	result, err := generatePackage(root, target)
	if err != nil {
		return nil, err
	}
	var drift []string
	for _, artifact := range result.Artifacts {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(artifact.RelPath)))
		if err != nil {
			drift = append(drift, artifact.RelPath)
			continue
		}
		if !artifactContentEqual(body, artifact.Content) {
			drift = append(drift, artifact.RelPath)
		}
	}
	drift = append(drift, result.StalePaths...)
	slices.Sort(drift)
	return slices.Compact(drift), nil
}
