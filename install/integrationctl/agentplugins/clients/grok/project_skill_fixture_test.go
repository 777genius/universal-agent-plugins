package grok

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/packagesnapshot"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type projectSkillFixture struct {
	request domain.ActivationRequest
	runner  *projectSkillRunner
	env     clients.Env
}

type projectSkillRunner struct {
	t        *testing.T
	request  domain.ActivationRequest
	document map[string]any
	calls    int
	during   func()
	err      error
}

func newProjectSkillFixture(t *testing.T) *projectSkillFixture {
	t.Helper()
	request := projectTestRequest(t)
	request.Client.Version = "1.0.70"
	request.Plan.PackageMode, request.Plan.Policy = domain.PackageNative, domain.PolicyAllowed
	request.Plan.Authentication = domain.AuthenticationNotChecked
	request.Plan.Warnings = []string{"authentication_not_catalog_verified"}
	root := request.Plan.ActivePath
	projectTestWrite(t, filepath.Join(root, ".grok-plugin", "plugin.json"), `{"name":"demo","skills":"./skills/"}`)
	projectTestWrite(t, filepath.Join(root, "skills", "docs", "SKILL.md"), "---\nname: docs\ndescription: Exact owned text\n---\nInert Skill.\n")
	projectTestWrite(t, filepath.Join(root, "skills", "docs", "references", "note.txt"), "original resource\n")
	projectTestWrite(t, filepath.Join(request.Client.ConfigRoot, "config.toml"), "[plugins]\nenabled=['demo']\n")
	projectTestWrite(t, filepath.Join(request.Client.ConfigRoot, "trusted_folders.toml"), "# native-owned trust, unchanged by observation\n")
	digest, err := (packagesnapshot.Builder{}).Digest(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	request.Delivery.ArtifactDigest = digest
	request.Delivery.NativeObjects = []domain.NativeObjectOwnership{{ObjectID: "package:grok:" + request.Plan.PhysicalArtifactID,
		Kind: "managed_package_directory", LogicalName: "demo", Path: root, ManagedDigest: digest, ProtectionClass: "managed"}}
	runner := &projectSkillRunner{t: t, request: request, document: projectSkillDocument(request)}
	return &projectSkillFixture{request: request, runner: runner, env: clients.Env{Runner: runner, NativeConfig: nativeconfig.New()}}
}

func projectSkillDocument(request domain.ActivationRequest) map[string]any {
	return map[string]any{"grokVersion": "1.0.70", "cwd": request.Plan.TargetContext.ProjectRoot,
		"projectRoot": request.Plan.TargetContext.ProjectRoot + string(filepath.Separator), "projectTrusted": true, "mcpServers": []any{},
		"plugins": []any{map[string]any{"name": "demo", "scope": "project", "path": request.Plan.ActivePath, "enabled": true,
			"provides": map[string]any{"skills": 1, "agents": 0, "hooks": false, "mcpServers": 0}}},
		"skills": []any{map[string]any{"name": "docs", "description": "Exact owned text", "userInvocable": true,
			"source": map[string]any{"type": "plugin", "plugin_name": "demo", "path": filepath.Join(request.Plan.ActivePath, "skills", "docs", "SKILL.md")}}}}
}

func (r *projectSkillRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	r.calls++
	if !reflect.DeepEqual(command.Argv, []string{r.request.Client.ExecutablePath, "inspect", "--json"}) ||
		command.Dir != r.request.Plan.TargetContext.ProjectRoot || command.StdoutLimitBytes != 1<<20 {
		r.t.Fatal("unexpected native command or scope", command)
	}
	if r.during != nil {
		r.during()
	}
	body, err := json.Marshal(r.document)
	if err != nil {
		r.t.Fatal(err)
	}
	return legacyports.CommandResult{Stdout: body}, r.err
}

func (f *projectSkillFixture) skill() map[string]any {
	return f.runner.document["skills"].([]any)[0].(map[string]any)
}

func (f *projectSkillFixture) plugin() map[string]any {
	return f.runner.document["plugins"].([]any)[0].(map[string]any)
}

func projectSkillFilesUnchanged(t *testing.T, paths ...string) func() {
	t.Helper()
	type snapshot struct {
		body []byte
		info os.FileInfo
	}
	before := map[string]snapshot{}
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		before[path] = snapshot{body: body, info: info}
	}
	return func() {
		t.Helper()
		for path, original := range before {
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(path)
			if err != nil || !reflect.DeepEqual(body, original.body) || !os.SameFile(info, original.info) ||
				info.Mode() != original.info.Mode() || !info.ModTime().Equal(original.info.ModTime()) {
				t.Fatal("read-only observation changed an original file", path, err)
			}
		}
	}
}
