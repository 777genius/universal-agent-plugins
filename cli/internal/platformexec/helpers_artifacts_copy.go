package platformexec

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func copyArtifactDirs(root string, dirs ...artifactDir) ([]pluginmodel.Artifact, error) {
	var artifacts []pluginmodel.Artifact
	for _, dir := range dirs {
		copied, err := copyArtifacts(root, dir.src, dir.dst)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, copied...)
	}
	return artifacts, nil
}

func copyArtifacts(root, srcDir, dstRoot string) ([]pluginmodel.Artifact, error) {
	full := filepath.Join(root, srcDir)
	var artifacts []pluginmodel.Artifact
	if _, err := os.Stat(full); err != nil {
		return nil, nil
	}
	err := filepath.WalkDir(full, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil {
			return err
		}
		rel, err := filepath.Rel(full, path)
		if err != nil {
			return err
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("imported %s must not contain symlinks: %s is a symlink", srcDir, filepath.ToSlash(rel))
		}
		if d.IsDir() {
			return nil
		}
		body, err := readRegularFile(path, filepath.ToSlash(filepath.Join(srcDir, rel)))
		if err != nil {
			return err
		}
		artifacts = append(artifacts, pluginmodel.Artifact{
			RelPath: filepath.ToSlash(filepath.Join(dstRoot, rel)),
			Content: body,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.SortFunc(artifacts, func(a, b pluginmodel.Artifact) int { return strings.Compare(a.RelPath, b.RelPath) })
	return artifacts, nil
}

func copySingleArtifactIfExists(root, srcRel, dstRel string) ([]pluginmodel.Artifact, error) {
	if strings.TrimSpace(srcRel) == "" {
		return nil, nil
	}
	body, err := readRegularFile(filepath.Join(root, srcRel), srcRel)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return []pluginmodel.Artifact{{RelPath: filepath.ToSlash(dstRel), Content: body}}, nil
}

func compactArtifacts(artifacts []pluginmodel.Artifact) []pluginmodel.Artifact {
	slices.SortFunc(artifacts, func(a, b pluginmodel.Artifact) int { return strings.Compare(a.RelPath, b.RelPath) })
	out := make([]pluginmodel.Artifact, 0, len(artifacts))
	for _, artifact := range artifacts {
		n := len(out)
		if n > 0 && out[n-1].RelPath == artifact.RelPath {
			out[n-1] = artifact
			continue
		}
		out = append(out, artifact)
	}
	return out
}

func copyArtifactsFromRefs(root string, refs []string, dstRoot string) ([]pluginmodel.Artifact, error) {
	var artifacts []pluginmodel.Artifact
	for _, ref := range refs {
		var err error
		ref, err = resolveRelativeRef(root, ref)
		if err != nil {
			return nil, err
		}
		if ref == "" {
			continue
		}
		full := filepath.Join(root, ref)
		info, err := os.Lstat(full)
		if err != nil {
			return nil, fileAccessError(err, ref)
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return nil, fmt.Errorf("imported %s must not be a symlink", ref)
		}
		if info.IsDir() {
			copied, err := copyArtifacts(root, ref, dstRoot)
			if err != nil {
				return nil, err
			}
			artifacts = append(artifacts, copied...)
			continue
		}
		body, err := readRegularFile(full, ref)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, pluginmodel.Artifact{
			RelPath: filepath.ToSlash(filepath.Join(dstRoot, filepath.Base(ref))),
			Content: body,
		})
	}
	return compactArtifacts(artifacts), nil
}
