package gemini_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/gemini"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func testDirectory(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func writeTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func profileCapability[T any](t *testing.T) T {
	t.Helper()
	capability, ok := any(gemini.New()).(T)
	if !ok {
		t.Fatalf("Gemini adapter lacks public profile capability %T", *new(T))
	}
	return capability
}

func testHost(home, cwd, env string) clients.Host {
	return clients.NewHost(clients.HostProbes{
		HomeDir: home, WorkingDir: cwd, GOOS: runtime.GOOS,
		Environment: map[string]string{"GEMINI_CLI_HOME": env},
		Lstat:       os.Lstat, EvalSymlinks: filepath.EvalSymlinks,
	})
}

func TestDetectionUsesNativeHomeParent(t *testing.T) {
	base := testDirectory(t)
	home, cwd := filepath.Join(base, "home"), filepath.Join(base, "project")
	for _, tc := range []struct{ name, value, want string }{
		{"empty", "", filepath.Join(home, ".gemini")},
		{"absolute", filepath.Join(base, "selected"), filepath.Join(base, "selected", ".gemini")},
		{"relative", "./selected", filepath.Join(cwd, "selected", ".gemini")},
		{"relative dotdot", "../selected", filepath.Join(base, "selected", ".gemini")},
		{"padded relative", " selected ", filepath.Join(cwd, " selected ", ".gemini")},
		{"whitespace is nonempty", " ", filepath.Join(cwd, " ", ".gemini")},
		{"padded absolute", filepath.Join(base, "selected") + " ", filepath.Join(base, "selected ", ".gemini")},
		{"home parent is already dotgemini", filepath.Join(base, ".gemini"), filepath.Join(base, ".gemini", ".gemini")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if runtime.GOOS == "windows" && (tc.name == "padded relative" || tc.name == "padded absolute" || tc.name == "whitespace is nonempty") {
				t.Skip("native Windows trailing-space identity requires separate qualification")
			}
			if err := os.MkdirAll(tc.want, 0700); err != nil {
				t.Fatal(err)
			}
			host := testHost(home, cwd, tc.value)
			t.Setenv("GEMINI_CLI_HOME", filepath.Join(base, "ambient-other"))
			t.Chdir(base)
			got := gemini.New().DetectSurfaces(host)
			if got.Err != nil || got.ConfigRoot != tc.want || !got.Surfaces[1].Detected {
				t.Fatalf("detection = %+v; want physical native root %q", got, tc.want)
			}
		})
	}
}

func TestDetectionCanonicalizesAliasesAndRefusesUnsafeAncestors(t *testing.T) {
	base := testDirectory(t)
	target := filepath.Join(base, "target")
	if err := os.MkdirAll(filepath.Join(target, ".gemini"), 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(base, "alias")
	if err := os.Symlink(target, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	for _, suffix := range []string{"", "missing/nested"} {
		got := gemini.New().DetectSurfaces(testHost(base, base, filepath.Join(alias, suffix)))
		want := filepath.Join(target, suffix, ".gemini")
		if got.Err != nil || got.ConfigRoot != want {
			t.Fatalf("alias detection = %+v; want %q", got, want)
		}
		if suffix != "" {
			if _, err := os.Lstat(want); !os.IsNotExist(err) || got.Surfaces[1].Detected {
				t.Fatal("missing profile was materialized or detected")
			}
		}
	}
	file := filepath.Join(base, "file")
	writeTestFile(t, file, "sentinel")
	dangling := filepath.Join(base, "dangling")
	if err := os.Symlink(filepath.Join(base, "absent"), dangling); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{file, filepath.Join(file, "child"), dangling, "bad\x00root", "bad\nroot"} {
		if got := gemini.New().DetectSurfaces(testHost(base, base, value)); got.Err == nil || got.ConfigRoot != "" {
			t.Fatalf("unsafe detection = %+v", got)
		}
	}
	host := clients.NewHost(clients.HostProbes{HomeDir: base, Lstat: func(string) (os.FileInfo, error) { return nil, os.ErrPermission }})
	if got := gemini.New().DetectSurfaces(host); got.Err == nil {
		t.Fatal("unreadable ancestor became profile authority")
	}
	if got := gemini.New().DetectSurfaces(testHost(base, "", "relative")); got.Err == nil {
		t.Fatal("relative environment without original cwd became profile authority")
	}
}

func TestExplicitProfileBoundary(t *testing.T) {
	resolver := profileCapability[clients.ProfileResolver](t)
	base := testDirectory(t)
	root := filepath.Join(base, ".gemini")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(base, "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	for _, suffix := range []string{"", "missing/nested"} {
		got, err := resolver.ResolveProfileRoot(filepath.Join(alias, suffix))
		if err != nil || got != filepath.Join(root, suffix) {
			t.Fatalf("resolve alias = %q, %v", got, err)
		}
	}
	for _, bad := range []string{"", "relative", string(filepath.Separator), root + string(filepath.Separator), root + string(filepath.Separator) + ".", " " + alias, alias + " ", alias + "\n", alias + "\x00"} {
		if _, err := resolver.ResolveProfileRoot(bad); err == nil {
			t.Fatalf("ambiguous explicit root accepted: %q", bad)
		}
	}
	file := filepath.Join(base, "file")
	writeTestFile(t, file, "sentinel")
	if _, err := resolver.ResolveProfileRoot(filepath.Join(file, "child")); err == nil {
		t.Fatal("file ancestor accepted")
	}
	missing := filepath.Join(base, "new", ".gemini")
	if got, err := resolver.ResolveProfileRoot(missing); err != nil || got != missing {
		t.Fatalf("missing root = %q, %v", got, err)
	}
	if _, err := os.Lstat(missing); !os.IsNotExist(err) {
		t.Fatal("resolver created missing root")
	}
}

func ownedMCP(t *testing.T, root string) domain.NativeObjectOwnership {
	t.Helper()
	receipt, err := nativeconfig.New().Apply(nativeconfig.Request{
		Paths: gemini.GeminiConfigPaths(root), Codec: nativeconfig.CodecGemini,
		Action: nativeconfig.ActionAdd, Name: "docs",
		Server: nativeconfig.Server{Type: "remote", RemoteTransport: "streamable-http", URL: "https://example.test/docs"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return domain.NativeObjectOwnership{Kind: gemini.GeminiMCPObjectKind, LogicalName: "docs", Path: receipt.Path, ManagedDigest: receipt.Digest}
}

func TestBindingProfileAuthority(t *testing.T) {
	validator := profileCapability[clients.ProfileBindingValidator](t)
	base := testDirectory(t)
	root, other := filepath.Join(base, "A", ".gemini"), filepath.Join(base, "B", ".gemini")
	object, otherObject := ownedMCP(t, root), ownedMCP(t, other)
	skillPath := filepath.Join(root, "skills", "docs")
	writeTestFile(t, filepath.Join(skillPath, "SKILL.md"), "# TEST skill\n")
	digest, err := shared.DigestSkillDirectory(skillPath)
	if err != nil {
		t.Fatal(err)
	}
	skill := domain.NativeObjectOwnership{Kind: gemini.GeminiSkillObjectKind, LogicalName: "docs", Path: skillPath, ManagedDigest: digest}
	changedSkill := skill
	changedSkill.ManagedDigest = "sha256:unowned"
	missingSkill := skill
	missingSkill.LogicalName, missingSkill.Path = "missing", filepath.Join(root, "skills", "missing")
	original, err := os.ReadFile(object.Path)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, selected, recorded, document string
		objects                            []domain.NativeObjectOwnership
		wantErr                            bool
	}{
		{name: "recorded match", selected: root, recorded: root},
		{name: "recorded missing match", selected: filepath.Join(base, "missing", ".gemini"), recorded: filepath.Join(base, "missing", ".gemini")},
		{name: "recorded mismatch despite same names", selected: root, recorded: other, objects: []domain.NativeObjectOwnership{object}, wantErr: true},
		{name: "legacy exact A", selected: root, objects: []domain.NativeObjectOwnership{object}},
		{name: "legacy exact B", selected: other, objects: []domain.NativeObjectOwnership{otherObject}},
		{name: "legacy exact skill", selected: root, objects: []domain.NativeObjectOwnership{skill}},
		{name: "legacy missing skill", selected: root, objects: []domain.NativeObjectOwnership{missingSkill}, wantErr: true},
		{name: "legacy changed skill digest", selected: root, objects: []domain.NativeObjectOwnership{changedSkill}, wantErr: true},
		{name: "recorded root with foreign object path", selected: root, recorded: root, objects: []domain.NativeObjectOwnership{otherObject}, wantErr: true},
		{name: "legacy wrong physical path", selected: other, objects: []domain.NativeObjectOwnership{object}, wantErr: true},
		{name: "legacy name only", selected: root, wantErr: true},
		{name: "legacy missing", selected: root, document: "{}", objects: []domain.NativeObjectOwnership{object}, wantErr: true},
		{name: "legacy drift", selected: root, document: "{\"mcpServers\":{\"docs\":{\"httpUrl\":\"https://example.test/foreign\"}}}", objects: []domain.NativeObjectOwnership{object}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := original
			if tc.document != "" {
				body = []byte(tc.document)
			}
			writeTestFile(t, object.Path, string(body))
			binding := domain.ClientBinding{NativeProfileRoot: tc.recorded, NativeObjects: tc.objects, PhysicalArtifact: "docs", TargetLocator: filepath.Join(base, "package")}
			if err := validator.ValidateBindingProfile(tc.selected, binding); (err != nil) != tc.wantErr {
				t.Fatalf("profile validation = %v; wantErr %v", err, tc.wantErr)
			}
			if got, err := os.ReadFile(object.Path); err != nil || string(got) != string(body) {
				t.Fatal("binding validation mutated profile A")
			}
			if got, err := os.ReadFile(otherObject.Path); err != nil || string(got) != string(original) {
				t.Fatal("binding validation mutated profile B")
			}
		})
	}
}

func TestSelectedProbeEnvironment(t *testing.T) {
	probe := profileCapability[clients.VersionProbeEnvironment](t)
	base := testDirectory(t)
	for _, name := range []string{"profile-A", "profile-B", "padded parent "} {
		if runtime.GOOS == "windows" && name == "padded parent " {
			continue // Native Windows trailing-space identity needs qualification.
		}
		root := filepath.Join(base, name, ".gemini")
		writeTestFile(t, filepath.Join(root, "settings.json"), "{\"sentinel\":true}\n")
		t.Setenv("GEMINI_CLI_HOME", filepath.Join(base, "ambient-other"))
		got, err := probe.VersionProbeEnvironment(root)
		want := []string{"GEMINI_CLI_HOME=" + filepath.Dir(root)}
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("probe environment = %v, %v; want %v", got, err, want)
		}
		selected := gemini.New().DetectSurfaces(testHost(filepath.Join(base, "other-home"), base, strings.TrimPrefix(got[0], "GEMINI_CLI_HOME=")))
		if selected.Err != nil || selected.ConfigRoot != root {
			t.Fatalf("probe environment selected another profile: %+v", selected)
		}
	}
	for _, root := range []string{filepath.Join(base, "arbitrary-config"), "relative/.gemini", filepath.Join(base, ".gemini") + " "} {
		if _, err := probe.VersionProbeEnvironment(root); err == nil {
			t.Fatalf("unrepresentable probe root accepted: %q", root)
		}
	}
	root := filepath.Join(base, "physical", ".gemini")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(base, ".gemini")
	if err := os.Symlink(root, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := probe.VersionProbeEnvironment(alias); err == nil {
		t.Fatal("probe accepted unresolved alias")
	}
	physical := filepath.Join(base, "arbitrary-physical")
	if err := os.Mkdir(physical, 0700); err != nil {
		t.Fatal(err)
	}
	nativeAlias := filepath.Join(base, "relocated", ".gemini")
	if err := os.Mkdir(filepath.Dir(nativeAlias), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(physical, nativeAlias); err != nil {
		t.Fatal(err)
	}
	selected := gemini.New().DetectSurfaces(testHost(base, base, filepath.Dir(nativeAlias)))
	if selected.Err != nil || selected.ConfigRoot != physical {
		t.Fatalf("native alias did not resolve physical identity: %+v", selected)
	}
	if _, err := probe.VersionProbeEnvironment(selected.ConfigRoot); err == nil {
		t.Fatal("probe guessed a home parent for arbitrary physical config root")
	}
}

func TestNativeMutationNeverTrimsProfile(t *testing.T) {
	root := filepath.Join(testDirectory(t), ".gemini")
	writeTestFile(t, filepath.Join(root, "settings.json"), "{\"sentinel\":true}\n")
	if err := gemini.ApplyGeminiNativeMutation(root+" ", "", nil, nil); err == nil {
		t.Fatal("native mutation trimmed another explicit profile into authority")
	}
	if body, err := os.ReadFile(filepath.Join(root, "settings.json")); err != nil || string(body) != "{\"sentinel\":true}\n" {
		t.Fatal("sentinel changed")
	}
	if _, err := os.Lstat(root + " "); runtime.GOOS != "windows" && !os.IsNotExist(err) {
		t.Fatal("refusal materialized padded profile")
	}
}

func TestLifecycleRejectsProfileMismatch(t *testing.T) {
	base := testDirectory(t)
	root, other := filepath.Join(base, "A", ".gemini"), filepath.Join(base, "B", ".gemini")
	for _, profile := range []string{root, other} {
		writeTestFile(t, filepath.Join(profile, "settings.json"), "{\"sentinel\":true}\n")
	}
	client := domain.DetectedClient{ClientID: domain.ClientGemini, ConfigRoot: root}
	plan := domain.DeliveryPlan{ClientID: domain.ClientGemini, NativeRegistryRoot: other}
	if _, err := gemini.New().Activate(context.Background(), clients.Env{}, domain.ActivationRequest{Client: client, Plan: plan, Delivery: domain.StagedDelivery{ClientID: domain.ClientGemini}}); err == nil {
		t.Fatal("activation accepted conflicting profiles")
	}
	precondition := profileCapability[clients.PlanPrecondition](t)
	if err := precondition.CheckPlanPrecondition(clients.PlanInput{Client: client}, &plan); err == nil {
		t.Fatal("plan accepted conflicting profiles")
	}
	binding := domain.ClientBinding{NativeProfileRoot: other}
	plan.NativeRegistryRoot = root
	if finding, err := gemini.New().InspectNativeRegistry(context.Background(), clients.Env{NativeConfig: nativeconfig.New()}, client, plan, &binding); err == nil || finding != clients.RegistryIndeterminate {
		t.Fatalf("inspection ignored recorded mismatch: %v %v", finding, err)
	}
	for _, confirmed := range []bool{false, true} {
		if _, err := gemini.New().Deactivate(context.Background(), clients.Env{NativeConfig: nativeconfig.New()}, domain.DeactivationRequest{Client: domain.DetectedClient{ConfigRoot: "relative"}, Confirmed: confirmed}); err == nil {
			t.Fatal("removal accepted ambiguous profile")
		}
	}
	for _, profile := range []string{root, other} {
		if body, err := os.ReadFile(filepath.Join(profile, "settings.json")); err != nil || string(body) != "{\"sentinel\":true}\n" {
			t.Fatal("profile refusal changed sentinel")
		}
	}
}
