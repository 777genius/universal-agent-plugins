package shared

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/packagesnapshot"
)

// ReadOwnedSkillMetadata binds the parsed SKILL.md bytes to the same sealed
// component tree as its existing ownership digest. The caller retains the
// original installed path for native catalog matching, never the temporary one.
func ReadOwnedSkillMetadata(ctx context.Context, kernel nativeconfig.Kernel, root, expected string) ([]byte, error) {
	// An unsupported injected backend cannot acquire OS read authority by way
	// of the snapshot builder. Its explicit bounded capability is mandatory.
	if err := kernel.RequireBoundedRead(); err != nil {
		return nil, err
	}
	snapshot, err := (packagesnapshot.Builder{}).Build(ctx, root)
	if err != nil {
		return nil, err
	}
	return readOwnedSkillSnapshot(ctx, kernel, root, expected, snapshot.Root, snapshot.Close)
}

func readOwnedSkillSnapshot(ctx context.Context, kernel nativeconfig.Kernel, original, expected, sealed string, closeSnapshot func() error) (body []byte, err error) {
	defer func() {
		err = errors.Join(err, closeSnapshot())
		if err != nil {
			body = nil
		}
	}()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// PackageSnapshot.Digest has a different framing. Hash the sealed component
	// with the existing Skill framing so receipt identities stay unchanged.
	digest, err := DigestSkillDirectory(sealed)
	if err != nil || digest != expected {
		return nil, errors.Join(err, fmt.Errorf("sealed Skill differs from its ownership digest"))
	}
	file, err := kernel.ReadExactFileBounded(filepath.Join(sealed, "SKILL.md"), 1<<20)
	if err != nil || !file.Exists {
		return nil, errors.Join(err, fmt.Errorf("sealed Skill metadata is unavailable"))
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	actual, err := DigestSkillDirectory(original)
	if err != nil || actual != expected {
		return nil, errors.Join(err, fmt.Errorf("owned Skill changed while capturing metadata"))
	}
	return file.Body, nil
}
