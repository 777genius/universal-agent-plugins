package agentpluginscli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func codexDoctorFixture(t *testing.T) (domain.Installation, string) {
	t.Helper()
	project := t.TempDir()
	skill := filepath.Join(project, ".agents", "skills", "demo")
	mcp := filepath.Join(project, ".codex", "config.toml")
	if err := os.MkdirAll(skill, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(mcp), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("---\nname: demo\ndescription: disposable test\n---\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mcp, []byte("# keep foreign config\n[mcp_servers.demo]\ncommand = 'disposable-inert'\n[mcp_servers.foreign]\nurl = 'https://foreign.invalid/mcp'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	installation := domain.Installation{InstallationID: "i", DeclaredName: "fixture", Clients: map[string]domain.ClientBinding{
		"codex": {
			ClientID: "codex", Scope: string(domain.ScopeProject),
			Materialization: domain.MaterializationMaterialized,
			Activation:      domain.ActivationPrepared, Verification: domain.VerificationInstalled,
			NativeProfileRoot: project,
			NativeObjects: []domain.NativeObjectOwnership{
				{ObjectID: "skill:demo", Kind: "codex_project_skill_directory", LogicalName: "demo", ProtectionClass: "managed", Path: skill},
				{ObjectID: "mcp:demo", Kind: "codex_project_mcp_server", LogicalName: "demo", ProtectionClass: "managed_entry", Path: mcp},
			},
		},
	}}
	return installation, project
}

func doctorCode(findings []doctorFinding, code string) bool {
	for _, f := range findings {
		if f.Code == code {
			return true
		}
	}
	return false
}

func doctorSingleProject(t *testing.T, inst domain.Installation) []doctorFinding {
	t.Helper()
	return doctorCodexNativeFindings(domain.StateFileV2{Installations: []domain.Installation{inst}}, nil)
}

func TestCodexDoctorReadsOwnedProjectWithoutMutation(t *testing.T) {
	t.Parallel()
	inst, root := codexDoctorFixture(t)
	config := filepath.Join(root, ".codex", "config.toml")
	skill := filepath.Join(root, ".agents", "skills", "demo", "SKILL.md")
	beforeConfig, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	beforeSkill, err := os.ReadFile(skill)
	if err != nil {
		t.Fatal(err)
	}
	for range 6 {
		findings := doctorSingleProject(t, inst)
		if len(findings) != 0 {
			t.Fatalf("healthy native file structure incorrectly degraded: %+v", findings)
		}
	}
	afterConfig, _ := os.ReadFile(config)
	afterSkill, _ := os.ReadFile(skill)
	if !reflect.DeepEqual(beforeConfig, afterConfig) || !reflect.DeepEqual(beforeSkill, afterSkill) {
		t.Fatal("read-only native audit wrote a foreign config or Skill")
	}
}

func TestCodexDoctorDetectsMissingSkillOrMCPWithoutTouchingForeign(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, remove, want string
	}{
		{"missing skill", "skill", "codex_project_component_missing"},
		{"missing SKILL.md", "skillfile", "codex_project_skill_component_missing"},
		{"missing MCP config", "mcp", "codex_project_component_missing"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			inst, root := codexDoctorFixture(t)
			paths := map[string]string{
				"skill": filepath.Join(root, ".agents", "skills", "demo"),
				"skillfile": filepath.Join(root, ".agents", "skills", "demo", "SKILL.md"),
				"mcp": filepath.Join(root, ".codex", "config.toml"),
			}
			if err := os.RemoveAll(paths[c.remove]); err != nil {
				t.Fatal(err)
			}
			findings := doctorSingleProject(t, inst)
			if !doctorCode(findings, c.want) {
				t.Fatalf("missing native component not detected: %+v", findings)
			}
		})
	}
}

func TestCodexDoctorTOMLMemberAndMalformedDocument(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, body, want string
	}{
		{"valid foreign sibling", "# foreign\n[mcp_servers.demo]\ncommand='inert'\n[mcp_servers.sibling]\nurl='https://unused.invalid'\n", ""},
		{"missing managed member", "[mcp_servers.sibling]\nurl='https://unused.invalid'\n", "codex_project_mcp_member_missing"},
		{"non-table managed member", "[mcp_servers]\ndemo=false\n", "codex_project_mcp_member_invalid"},
		{"missing collection", "[not_mcp_servers]\nkeep='value'\n", "codex_project_mcp_missing"},
		{"malformed", "[mcp_servers.demo\n", "codex_project_mcp_invalid"},
		{"overlarge", strings.Repeat("#", doctorNativeFileMax+1), "codex_project_mcp_component_too_large"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			inst, root := codexDoctorFixture(t)
			config := filepath.Join(root, ".codex", "config.toml")
			if err := os.WriteFile(config, []byte(c.body), 0o600); err != nil {
				t.Fatal(err)
			}
			got := doctorSingleProject(t, inst)
			if c.want == "" && len(got) != 0 || c.want != "" && !doctorCode(got, c.want) {
				t.Fatalf("Codex config audit result: %+v", got)
			}
			marker, _ := os.ReadFile(config)
			if string(marker) != c.body {
				t.Fatal("read-only audit modified original TOML")
			}
		})
	}
}

func TestCodexDoctorRecognizesMultipleOwnedServersInOneDocument(t *testing.T) {
	t.Parallel()
	inst, root := codexDoctorFixture(t)
	mcp := filepath.Join(root, ".codex", "config.toml")
	b := inst.Clients["codex"]
	b.NativeObjects = append(b.NativeObjects, domain.NativeObjectOwnership{
		ObjectID: "mcp:other", Kind: "codex_project_mcp_server", LogicalName: "other",
		Path: mcp, ProtectionClass: "managed_entry",
	})
	inst.Clients["codex"] = b
	if err := os.WriteFile(mcp, []byte("[mcp_servers.demo]\ncommand='inert'\n[mcp_servers.other]\ncommand='inert'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := doctorSingleProject(t, inst); len(got) != 0 {
		t.Fatalf("shared Codex MCP file falsely counted as duplicate path: %+v", got)
	}
	if err := os.WriteFile(mcp, []byte("[mcp_servers.demo]\ncommand='inert'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := doctorSingleProject(t, inst); !doctorCode(got, "codex_project_mcp_member_missing") {
		t.Fatalf("missing second MCP member ignored: %+v", got)
	}
}

func TestCodexDoctorRejectsDriftedNativeOwnershipReceipts(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		mutate func(*domain.ClientBinding, string)
		want string
	}{
		{"skill path escaped", func(b *domain.ClientBinding, root string) { b.NativeObjects[0].Path = filepath.Join(filepath.Dir(root), "outside") }, "codex_project_path_mismatch"},
		{"MCP points to user profile", func(b *domain.ClientBinding, _ string) { b.NativeObjects[1].Path = "/tmp/user-global-config.toml" }, "codex_project_path_mismatch"},
		{"invalid skill name", func(b *domain.ClientBinding, _ string) { b.NativeObjects[0].LogicalName = "../foreign" }, "codex_project_invalid_skill_receipt"},
		{"invalid MCP name", func(b *domain.ClientBinding, _ string) { b.NativeObjects[1].LogicalName = ".." }, "codex_project_invalid_mcp_receipt"},
		{"no owner", func(b *domain.ClientBinding, _ string) { b.NativeObjects[0].ProtectionClass = "foreign" }, "codex_project_ownership_unverified"},
		{"duplicate object identity", func(b *domain.ClientBinding, _ string) { b.NativeObjects = append(b.NativeObjects, b.NativeObjects[0]) }, "codex_project_duplicate_receipt"},
		{"relative project root", func(b *domain.ClientBinding, _ string) { b.NativeProfileRoot = "../other" }, "codex_project_root_invalid"},
		{"missing project root", func(b *domain.ClientBinding, _ string) { b.NativeProfileRoot = "" }, "codex_project_receipts_unavailable"},
		{"missing ownership list", func(b *domain.ClientBinding, _ string) { b.NativeObjects = nil }, "codex_project_receipts_unavailable"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			inst, root := codexDoctorFixture(t)
			b := inst.Clients["codex"]
			test.mutate(&b, root)
			inst.Clients["codex"] = b
			findings := doctorSingleProject(t, inst)
			if !doctorCode(findings, test.want) {
				t.Fatalf("altered ownership accepted: %+v", findings)
			}
		})
	}
}

func TestCodexDoctorRejectsSymlinksAndDirectoryAliases(t *testing.T) {
	t.Parallel()
	for _, target := range []string{"skill", "skillfile", "config", "configparent"} {
		t.Run(target, func(t *testing.T) {
			inst, root := codexDoctorFixture(t)
			outside := t.TempDir()
			source := filepath.Join(root, ".codex", "config.toml")
			kind := "codex_project_component_symlink"
			switch target {
			case "skill":
				source = filepath.Join(root, ".agents", "skills", "demo")
			case "skillfile":
				source = filepath.Join(root, ".agents", "skills", "demo", "SKILL.md")
				kind = "codex_project_skill_component_symlink"
			case "configparent":
				source = filepath.Join(root, ".codex")
			}
			if err := os.RemoveAll(source); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, source); err != nil {
				t.Skipf("host does not allow disposable symlink creation: %v", err)
			}
			if got := doctorSingleProject(t, inst); !doctorCode(got, kind) {
				t.Fatalf("linked project component not detected: %+v", got)
			}
			files, err := os.ReadDir(outside)
			if err != nil || len(files) != 0 {
				t.Fatal("native audit wrote or followed outside project symlink")
			}
		})
	}
}

func TestCodexDoctorRejectsWrongTypesAndBoundedInputs(t *testing.T) {
	t.Parallel()
	inst, root := codexDoctorFixture(t)
	skill := filepath.Join(root, ".agents", "skills", "demo")
	if err := os.RemoveAll(skill); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skill, []byte("not-a-directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := doctorSingleProject(t, inst); !doctorCode(got, "codex_project_component_wrong_type") {
		t.Fatalf("file masquerading as skill directory accepted: %+v", got)
	}
	binding := inst.Clients["codex"]
	binding.NativeObjects = make([]domain.NativeObjectOwnership, doctorNativeObjectMax+1)
	inst.Clients["codex"] = binding
	if got := doctorSingleProject(t, inst); !doctorCode(got, "codex_project_inventory_too_large") {
		t.Fatalf("oversized native inventory not bounded: %+v", got)
	}
}

func TestCodexDoctorDoesNotExposeTOMLOrProfileSecrets(t *testing.T) {
	t.Parallel()
	inst, root := codexDoctorFixture(t)
	canary := "SYNTHETIC_SECRET_DO_NOT_PRINT_43587"
	config := filepath.Join(root, ".codex", "config.toml")
	if err := os.WriteFile(config, []byte("[mcp_servers.demo]\ncommand='inert'\napi_key='"+canary+"'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	binding := inst.Clients["codex"]
	binding.NativeProfileRoot = root
	inst.Clients["codex"] = binding
	findings := doctorSingleProject(t, inst)
	body, err := json.Marshal(findings)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), canary) || strings.Contains(string(body), root) {
		t.Fatalf("native doctor leaked user profile or secret: %s", body)
	}
}

func TestCodexDoctorSkipsAbsentAndUserBindings(t *testing.T) {
	t.Parallel()
	inst, root := codexDoctorFixture(t)
	b := inst.Clients["codex"]
	b.NativeProfileRoot = root
	b.Materialization = domain.MaterializationAbsent
	inst.Clients["codex"] = b
	if got := doctorSingleProject(t, inst); len(got) != 0 {
		t.Fatalf("removed binding unexpectedly audited: %+v", got)
	}
	b.Materialization = domain.MaterializationMaterialized
	b.Scope = string(domain.ScopeUser)
	inst.Clients["codex"] = b
	if got := doctorSingleProject(t, inst); len(got) != 0 {
		t.Fatalf("user native profile passed to project audit: %+v", got)
	}
}

func TestCodexDoctorNativePathRecognizer(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"", ".", "..", "nested/escape", "nested\\escape"} {
		if doctorNativeLeafName(name) {
			t.Fatalf("unsafe native component identity accepted: %q", name)
		}
	}
	for _, name := range []string{"normal", "docs-kit", "mcp_server", "with.dots"} {
		if !doctorNativeLeafName(name) {
			t.Fatalf("safe native identifier rejected: %q", name)
		}
	}
}

// Invalid foreign and duplicate MCP ownership must never create a second,
// fabricated missing-member finding for a server UAP does not own.
func TestCodexDoctorDoesNotInspectForeignOrInvalidMCPReceipts(t *testing.T) {
	t.Parallel()
	for _, variant := range []string{"foreign", "bad_path", "duplicate"} {
		t.Run(variant, func(t *testing.T) {
			installation, root := codexDoctorFixture(t)
			binding := installation.Clients["codex"]
			mcp := filepath.Join(root, ".codex", "config.toml")
			object := domain.NativeObjectOwnership{
				ObjectID: "mcp:not-recorded", Kind: "codex_project_mcp_server",
				LogicalName: "not-recorded", Path: mcp, ProtectionClass: "managed_entry",
			}
			want := ""
			switch variant {
			case "foreign":
				object.ProtectionClass = "foreign"
				want = "codex_project_ownership_unverified"
			case "bad_path":
				object.Path = filepath.Join(t.TempDir(), "config.toml")
				want = "codex_project_path_mismatch"
			case "duplicate":
				// The first copy is not manager-owned. The second must be
				// rejected as duplicate without creating an MCP member check.
				object.ProtectionClass = "foreign"
				binding.NativeObjects = append(binding.NativeObjects, object)
				want = "codex_project_duplicate_receipt"
			}
			binding.NativeObjects = append(binding.NativeObjects, object)
			installation.Clients["codex"] = binding
			findings := doctorSingleProject(t, installation)
			if !doctorCode(findings, want) {
				t.Fatalf("invalid MCP ownership was accepted: %+v", findings)
			}
			if doctorCode(findings, "codex_project_mcp_member_missing") {
				t.Fatalf("foreign or duplicate MCP server was audited as owned: %+v", findings)
			}
		})
	}
}
