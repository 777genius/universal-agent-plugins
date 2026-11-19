package windsurf

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// Keep the real selected profile reader and inject only a project marker at
// the volume root. Tests never create files in the machine's actual root.
type devinSkillContextFiles struct {
	kernel  nativeconfig.Kernel
	marker  string
	present bool
}

func (files *devinSkillContextFiles) ReadNoFollow(path string) ([]byte, os.FileMode, bool, error) {
	file, err := files.kernel.ReadExactFile(path)
	return file.Body, file.Mode, file.Exists, err
}

func (files *devinSkillContextFiles) ReadNoFollowBounded(path string, limit int64) ([]byte, os.FileMode, bool, error) {
	if path == files.marker {
		return nil, 0600, files.present, nil
	}
	root := filepath.VolumeName(path) + string(filepath.Separator)
	if filepath.Dir(path) == root {
		switch filepath.Base(path) {
		case ".git", ".jj", ".devin", ".agents", ".windsurf", ".claude", ".cursor", ".github", ".zed", "opencode.json", ".mcp.json":
			return nil, 0600, false, nil
		}
	}
	file, err := files.kernel.ReadExactFileBounded(path, limit)
	return file.Body, file.Mode, file.Exists, err
}

func (*devinSkillContextFiles) WriteAtomic(string, []byte, os.FileMode) error {
	return fmt.Errorf("read-only observation attempted a write")
}

func (*devinSkillContextFiles) RemoveNoFollow(string) error {
	return fmt.Errorf("read-only observation attempted a removal")
}

func TestDevinGlobalSkillObservationRefusesInheritedContextBeforeAndAfter(t *testing.T) {
	for _, name := range []string{".git", ".jj", ".devin", ".agents", ".claude", ".mcp.json"} {
		for _, before := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/before=%v", name, before), func(t *testing.T) {
				request, env, row, _ := devinGlobalSkillFixture(t)
				root := filepath.VolumeName(request.Client.NativeConfigRoot) + string(filepath.Separator)
				files := &devinSkillContextFiles{kernel: env.NativeConfig, marker: filepath.Join(root, name), present: before}
				env.NativeConfig = nativeconfig.NewWithFileIO(files)
				calls := 0
				env.Runner = devinSkillRunner(func(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
					calls++
					files.present = true
					body, _ := json.Marshal([]any{row})
					return legacyports.CommandResult{Stdout: body}, nil
				})
				got, err := New().Activate(t.Context(), env, request)
				wantCalls := 1
				if before {
					wantCalls = 0
				}
				if calls != wantCalls || got.Activation == domain.ActivationActive || len(got.UserActions) == 0 {
					t.Fatalf("unobserved Global context qualified: calls=%d outcome=%+v err=%v", calls, got, err)
				}
			})
		}
	}
}

func TestDevinGlobalSkillsKeepObservedForeignImportsWithoutLendingOwnership(t *testing.T) {
	for _, provider := range []string{"Claude", "Copilot", "unknown"} {
		for _, owned := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/owned=%v", provider, owned), func(t *testing.T) {
				request, env, row, _ := devinGlobalSkillFixture(t)
				foreign := map[string]any{"name": "foreign-import", "description": "Existing user Skill", "provider": provider,
					"base_dir":     filepath.Join(filepath.Dir(request.Client.NativeConfigRoot), "foreign-import"),
					"display_name": "foreign-import", "triggers": []any{"user", "model"}, "warnings": []any{}, "errors": []any{}}
				rows := []any{row, foreign}
				if owned {
					row["provider"] = provider
					rows = []any{row}
				}
				env.Runner = devinSkillRunner(func(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
					body, _ := json.Marshal(rows)
					return legacyports.CommandResult{Stdout: body}, nil
				})
				got, err := New().Activate(t.Context(), env, request)
				ready := !owned && provider != "unknown"
				if err != nil || (got.Activation == domain.ActivationActive) != ready {
					t.Fatalf("foreign provider scope changed readiness: %+v %v", got, err)
				}
			})
		}
	}
}
