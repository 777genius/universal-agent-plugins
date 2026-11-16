package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/filetree"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type geminiIntactFiles struct {
	writes  int
	allowed string
}

func (*geminiIntactFiles) ReadNoFollow(path string) ([]byte, os.FileMode, bool, error) {
	s, err := nativeconfig.New().ReadExactFile(path)
	return s.Body, s.Mode, s.Exists, err
}
func (f *geminiIntactFiles) WriteAtomic(path string, body []byte, mode os.FileMode) error {
	f.writes++
	if path == f.allowed {
		return os.WriteFile(path, body, mode)
	}
	return errors.New("intact setup must not write config")
}
func (f *geminiIntactFiles) RemoveNoFollow(string) error {
	f.writes++
	return errors.New("intact setup must not remove config")
}

type geminiIntactProbe struct {
	skill string
	calls int
}

func (p *geminiIntactProbe) Run(_ context.Context, c legacyports.Command) (legacyports.CommandResult, error) {
	p.calls++
	if len(c.Argv) > 1 && c.Argv[1] == "skills" {
		return legacyports.CommandResult{CleanupComplete: true, Stdout: []byte("Discovered Agent Skills:\n\naudit [Enabled]\n  Description: audit marker\n  Location:    " + p.skill + "\n\n")}, nil
	}
	return legacyports.CommandResult{CleanupComplete: true, Stderr: []byte("Configured MCP servers:\n\n✓ foreign: foreign-server  (stdio) - Connected\n✓ audit: owned-server --audit (stdio) - Connected\n")}, nil
}

func TestGeminiAdapterIntactHealthPreservesProjectionAndNativeFiles(t *testing.T) {
	request, skill := geminiIntactHealthFixture(t)
	files := &geminiIntactFiles{}
	probe := &geminiIntactProbe{skill: filepath.Join(skill, "SKILL.md")}
	env := clients.Env{NativeConfig: nativeconfig.NewWithFileIO(files), Runner: probe}
	paths := []string{skill, probe.skill, filepath.Join(request.Plan.NativeRegistryRoot, "settings.json"), filepath.Join(request.Client.ConfigRoot, "trustedFolders.json")}
	unchanged := captureGeminiIntactFiles(t, paths)
	for range 2 {
		out, err := New().Activate(t.Context(), env, request)
		if err != nil || out.Activation != domain.ActivationActive || out.NativeEffect != domain.NativeEffectUnchanged || files.writes != 0 {
			t.Fatalf("intact health reapplied projection: %+v err=%v writes=%d", out, err, files.writes)
		}
	}
	if probe.calls != 4 {
		t.Fatalf("health observation missing: calls=%d", probe.calls)
	}
	unchanged()
	if err := os.WriteFile(probe.skill, []byte("foreign drift"), 0600); err != nil {
		t.Fatal(err)
	}
	if intactGeminiProjectProjection(env, request, request.Plan.NativeRegistryRoot) {
		t.Fatal("drift qualified intact setup")
	}
	if err := os.Remove(probe.skill); err != nil {
		t.Fatal(err)
	}
	if intactGeminiProjectProjection(env, request, request.Plan.NativeRegistryRoot) {
		t.Fatal("missing object qualified intact setup")
	}
}

func captureGeminiIntactFiles(t *testing.T, paths []string) func() {
	t.Helper()
	var err error
	infos := make([]os.FileInfo, len(paths))
	bodies := make([][]byte, len(paths))
	for i, path := range paths {
		infos[i], err = os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if !infos[i].IsDir() {
			bodies[i], err = os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	return func() {
		for i, path := range paths {
			after, err := os.Stat(path)
			if err != nil || !os.SameFile(infos[i], after) {
				t.Fatalf("intact inode changed: %s %v", path, err)
			}
			if !after.IsDir() {
				body, err := os.ReadFile(path)
				if err != nil || string(body) != string(bodies[i]) {
					t.Fatalf("intact bytes changed: %s %v", path, err)
				}
			}
		}
	}
}

func geminiIntactHealthFixture(t *testing.T) (domain.ActivationRequest, string) {
	t.Helper()
	cwd := t.TempDir()
	request, _ := geminiMCPReviewFixture(t, nativeconfig.Server{Type: "stdio", Command: "owned-server", Args: []string{"--audit"}, CWD: cwd})
	request.VerifyOnly = false
	request.Replacing = true
	request.Delivery.ActivePath = cwd
	mcp, err := json.Marshal(map[string]any{"mcpServers": map[string]any{"audit": map[string]any{"type": "stdio", "command": "owned-server", "args": []string{"--audit"}, "cwd": "${PLUGIN_ROOT}"}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(request.Delivery.ActivePath, "mcp.json"), mcp, 0600); err != nil {
		t.Fatal(err)
	}
	if err := filetree.CopyDir(filepath.Join(request.Plan.NativeRegistryRoot, "skills", "audit"), filepath.Join(request.Delivery.ActivePath, "skills", "audit")); err != nil {
		t.Fatal(err)
	}
	descriptor, err := json.Marshal(GeminiDescriptor{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(request.Delivery.ActivePath, GeminiDescriptorName), descriptor, 0600); err != nil {
		t.Fatal(err)
	}
	skill := filepath.Join(request.Plan.NativeRegistryRoot, "skills", "audit")
	digest, err := shared.DigestSkillDirectory(skill)
	if err != nil {
		t.Fatal(err)
	}
	object := domain.NativeObjectOwnership{ObjectID: GeminiSkillObjectKind + ":audit", Kind: GeminiSkillObjectKind, LogicalName: "audit", Path: skill, SourceRelative: "skills/audit", ManagedDigest: digest, ProtectionClass: "managed"}
	request.Delivery.NativeObjects = append(request.Delivery.NativeObjects, object)
	persisted, err := json.Marshal(request.Delivery.NativeObjects)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(persisted, &request.PreviousNativeObjects); err != nil {
		t.Fatal(err)
	}
	request.Plan.Components = append(request.Plan.Components, domain.ComponentDecision{Kind: domain.ComponentSkill, Name: "audit", Support: domain.SupportNative})
	return request, skill
}

func TestGeminiAdapterIntactHealthRecordsOnlyMissingTrustWrite(t *testing.T) {
	request, skill := geminiIntactHealthFixture(t)
	profile := filepath.Join(request.Client.ConfigRoot, "trustedFolders.json")
	body, err := os.ReadFile(profile)
	if err != nil {
		t.Fatal(err)
	}
	var mapping map[string]string
	if err := json.Unmarshal(body, &mapping); err != nil {
		t.Fatal(err)
	}
	delete(mapping, request.Plan.TargetContext.ProjectRoot)
	body, err = json.Marshal(mapping)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(profile, body, 0600); err != nil {
		t.Fatal(err)
	}
	files := &geminiIntactFiles{allowed: profile}
	probe := &geminiIntactProbe{skill: filepath.Join(skill, "SKILL.md")}
	env := clients.Env{NativeConfig: nativeconfig.NewWithFileIO(files), Runner: probe}
	unchanged := captureGeminiIntactFiles(t, []string{skill, probe.skill, filepath.Join(request.Plan.NativeRegistryRoot, "settings.json")})
	out, err := New().Activate(t.Context(), env, request)
	if err != nil || out.NativeEffect != domain.NativeEffectCommitted || out.Activation != domain.ActivationActive || files.writes != 1 {
		t.Fatalf("missing trust was not recorded as sole effect: %+v err=%v writes=%d", out, err, files.writes)
	}
	unchanged()
	body, err = os.ReadFile(profile)
	if err != nil {
		t.Fatal(err)
	}
	var after map[string]string
	if err := json.Unmarshal(body, &after); err != nil {
		t.Fatal(err)
	}
	for key, value := range mapping {
		if after[key] != value {
			t.Fatal("foreign trust changed")
		}
	}
	if after[request.Plan.TargetContext.ProjectRoot] != "TRUST_FOLDER" {
		t.Fatal("exact selected trust missing")
	}
	out, err = New().Activate(t.Context(), env, request)
	if err != nil || out.NativeEffect != domain.NativeEffectUnchanged || files.writes != 1 {
		t.Fatalf("trusted repeat wrote config: %+v %v", out, err)
	}
	unchanged()
}
