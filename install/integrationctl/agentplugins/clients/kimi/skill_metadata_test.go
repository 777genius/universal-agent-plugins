package kimi

import (
	"bytes"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/packagesnapshot"
)

type kimiMetadataABAIO struct {
	nativeconfig.FileIO
	live                string
	original, transient []byte
	reads               int
}

func (f *kimiMetadataABAIO) ReadNoFollowBounded(path string, limit int64) ([]byte, os.FileMode, bool, error) {
	if filepath.Base(path) != "SKILL.md" {
		file, err := nativeconfig.New().ReadExactFileBounded(path, limit)
		return file.Body, file.Mode, file.Exists, err
	}
	f.reads++
	if err := os.WriteFile(f.live, f.transient, 0600); err != nil {
		return nil, 0, false, err
	}
	file, err := nativeconfig.New().ReadExactFileBounded(path, limit)
	restore := os.WriteFile(f.live, f.original, 0600)
	return file.Body, file.Mode, file.Exists, errors.Join(err, restore)
}

func TestKimiCompletionParsesSealedMetadataDuringLiveABA(t *testing.T) {
	for _, nativeDescription := range []string{"Owned Kimi instructions", "Transient instructions"} {
		t.Run(nativeDescription, func(t *testing.T) {
			f := newKimiCompletionFixture(t, "2.5.2")
			f.env.NativeHTTP = nil
			pending, err := New().Activate(t.Context(), f.env, f.request)
			if err != nil || pending.Activation != domain.ActivationPrepared {
				t.Fatal(pending, err)
			}
			live := filepath.Join(f.request.Plan.NativeRegistryRoot, "skills", "docs", "SKILL.md")
			original, err := os.ReadFile(live)
			if err != nil {
				t.Fatal(err)
			}
			files := &kimiMetadataABAIO{live: live, original: original, transient: bytes.ReplaceAll(original, []byte("Owned Kimi instructions"), []byte("Transient instructions"))}
			f.env.NativeConfig = nativeconfig.NewWithFileIO(files)
			snapshotRoot := t.TempDir()
			f.env.Snapshots = packagesnapshot.Builder{TempRoot: snapshotRoot}
			f.env.NativeHTTP = f
			f.change = func(request *http.Request, data map[string]any) {
				if strings.HasSuffix(request.URL.Path, "/skills") {
					data["skills"].([]any)[0].(map[string]any)["description"] = nativeDescription
				}
			}
			out, err := completeKimiSkills(t.Context(), f.env, f.request, pending)
			if err != nil || f.queries != 1 || files.reads != 2 || (out.Activation == domain.ActivationActive) != (nativeDescription == "Owned Kimi instructions") {
				t.Fatalf("ABA influenced parsed metadata: %+v err=%v queries=%d reads=%d", out, err, f.queries, files.reads)
			}
			body, err := os.ReadFile(live)
			if err != nil || !bytes.Equal(body, original) {
				t.Fatal("live metadata not restored", err)
			}
			left, err := os.ReadDir(snapshotRoot)
			if err != nil || len(left) != 0 {
				t.Fatal("sealed snapshot cleanup incomplete", left, err)
			}
		})
	}
}
