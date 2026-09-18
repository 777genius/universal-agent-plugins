package providers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/internal/goldentest"
)

type stagingTree struct {
	Digests map[string]string
	Objects []domain.NativeObjectOwnership
}

func TestStagerGoldenTreesAcrossClients(t *testing.T) {
	t.Parallel()
	for _, definition := range domain.ClientDefinitions() {
		t.Run(string(definition.ID), func(t *testing.T) {
			t.Parallel()
			envelope := stagingEnvelope(t)
			plan := stagingPlan(t, definition.ID, domain.PackageProjection)
			delivery, err := testStager(Stager{}).Stage(context.Background(), envelope, plan, "golden", domain.CompatibilityHints{})
			if err != nil {
				t.Fatal(err)
			}
			replace := []goldentest.Replacement{
				{From: plan.ActivePath, To: "${ACTIVE}"},
				{From: plan.NativeRegistryRoot, To: "${CONFIG}"},
				{From: delivery.StagingPath, To: "${STAGING}"},
				{From: plan.TargetRoot, To: "${TARGET}"},
				{From: plan.TargetAnchor, To: "${ANCHOR}"},
			}
			files, err := digestTree(delivery.StagingPath, replace)
			if err != nil {
				t.Fatal(err)
			}
			objects := append([]domain.NativeObjectOwnership(nil), delivery.NativeObjects...)
			for i := range objects {
				if objects[i].Kind == "managed_package_directory" {
					objects[i].ManagedDigest = "${ARTIFACT}"
				}
			}
			goldentest.Golden{
				Dir:     filepath.Join("testdata", "golden", "staging"),
				Replace: replace,
			}.Assert(t, string(definition.ID), stagingTree{Digests: files, Objects: objects})
		})
	}
}

func digestTree(root string, replace []goldentest.Replacement) (map[string]string, error) {
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			files[rel+"/"] = "dir"
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(body)
		for _, replacement := range replace {
			text = strings.ReplaceAll(text, replacement.From, replacement.To)
		}
		if filepath.Separator != '/' {
			text = strings.ReplaceAll(text, string(filepath.Separator), "/")
		}
		sum := sha256.Sum256([]byte(text))
		files[rel] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(files))
	for key := range files {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	ordered := make(map[string]string, len(files))
	for _, key := range keys {
		ordered[key] = files[key]
	}
	return ordered, nil
}
