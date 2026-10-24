package skillsource

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const NormalizerV1 = "agent-skill-package/v1"

// Normalize verifies the sealed raw subtree, preserves its original bytes and
// modes under skills/<name>, then seals an ordinary standard-only package.
// Source identity remains the caller's responsibility. raw remains caller-owned.
func Normalize(ctx context.Context, raw domain.PackageSnapshot, expectedName, stableSourceKey string) (domain.PackageSnapshot, error) {
	if strings.TrimSpace(stableSourceKey) == "" || raw.TreeDigest == "" {
		return domain.PackageSnapshot{}, fmt.Errorf("skill normalization requires a sealed source and stable source key")
	}
	builder := packagedigest.Builder{}
	verified, err := builder.SnapshotWithExecutables(ctx, raw.Root, raw.Source, raw.ExecutableFiles)
	if err != nil {
		return domain.PackageSnapshot{}, err
	}
	defer func() { _ = packagedigest.Remove(verified) }()
	if verified.TreeDigest != raw.TreeDigest {
		return domain.PackageSnapshot{}, fmt.Errorf("raw Skill tree changed after acquisition")
	}
	doc := filepath.Join(verified.Root, "SKILL.md")
	info, err := os.Lstat(doc)
	if err != nil || !info.Mode().IsRegular() || info.Size() > int64(conformance.DefaultLimits().SkillBytes) {
		return domain.PackageSnapshot{}, fmt.Errorf("skill requires a bounded regular SKILL.md")
	}
	body, err := os.ReadFile(doc)
	if err != nil {
		return domain.PackageSnapshot{}, err
	}
	skill, err := conformance.ParseSkill(ctx, expectedName, body)
	if err != nil {
		return domain.PackageSnapshot{}, fmt.Errorf("invalid selected Skill: %w", err)
	}
	temp, err := os.MkdirTemp("", "agentplugins-skill-normalize-*")
	if err != nil {
		return domain.PackageSnapshot{}, err
	}
	defer func() { _ = os.RemoveAll(temp) }()
	skillRoot := filepath.Join(temp, "skills", skill.Name)
	if err = os.MkdirAll(skillRoot, 0700); err != nil {
		return domain.PackageSnapshot{}, err
	}
	if err = copySkillTree(ctx, verified.Root, skillRoot); err != nil {
		return domain.PackageSnapshot{}, err
	}
	if err = writeSyntheticManifest(temp, stableSourceKey); err != nil {
		return domain.PackageSnapshot{}, err
	}
	artifact, err := snapshotNormalizedTree(ctx, builder, temp, verified, skill.Name)
	if err != nil {
		return domain.PackageSnapshot{}, err
	}
	artifact.AcquiredAt = raw.AcquiredAt
	return artifact, nil
}

func copySkillTree(ctx context.Context, root, destination string) error {
	output, err := os.OpenRoot(destination)
	if err != nil {
		return err
	}
	defer func() { _ = output.Close() }()
	return filepath.WalkDir(root, func(p string, _ os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if p == root {
			return nil
		}
		relative, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		return copySkillResource(p, output, relative)
	})
}
func copySkillResource(source string, output *os.Root, destination string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	switch {
	case info.IsDir():
		return output.Mkdir(destination, 0700)
	case info.Mode()&os.ModeSymlink != 0:
		link, err := os.Readlink(source)
		if err != nil {
			return err
		}
		return output.Symlink(link, destination)
	case info.Mode().IsRegular():
		data, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		mode := os.FileMode(0600)
		if info.Mode()&0111 != 0 {
			mode = 0700
		}
		return output.WriteFile(destination, data, mode)
	default:
		return fmt.Errorf("unsupported raw Skill resource")
	}
}

func writeSyntheticManifest(root, stableSourceKey string) error {
	sum := sha256.Sum256([]byte(stableSourceKey))
	name := "skill-" + hex.EncodeToString(sum[:])[:56]
	manifest := struct {
		Schema string `json:"$schema"`
		Name   string `json:"name"`
	}{domain.PluginSchemaV1, name}
	data, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err = os.WriteFile(filepath.Join(root, "plugin.json"), data, 0600); err != nil {
		return err
	}
	return nil
}

// snapshotNormalizedTree seals the portable package after raw content is copied.
// Executable identity belongs to the immutable Git inventory, not host mode bits.
func snapshotNormalizedTree(ctx context.Context, builder packagedigest.Builder, root string, raw domain.PackageSnapshot, skillName string) (domain.PackageSnapshot, error) {
	executables := make([]string, 0, len(raw.ExecutableFiles))
	for _, relative := range raw.ExecutableFiles {
		executables = append(executables, path.Join("skills", skillName, relative))
	}
	return builder.SnapshotWithExecutables(ctx, root, raw.Source, executables)
}
