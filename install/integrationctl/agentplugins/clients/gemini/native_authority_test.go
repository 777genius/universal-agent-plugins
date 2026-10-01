package gemini_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/gemini"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func ownedSkill(t *testing.T, root, body string) domain.NativeObjectOwnership {
	t.Helper()
	path := filepath.Join(root, "skills", "docs")
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
	if body != "" {
		writeTestFile(t, filepath.Join(path, "SKILL.md"), body)
	}
	digest, err := shared.DigestSkillDirectory(path)
	if err != nil {
		t.Fatal(err)
	}
	return domain.NativeObjectOwnership{ObjectID: "gemini-skill:docs", Kind: gemini.GeminiSkillObjectKind,
		LogicalName: "docs", Path: path, ManagedDigest: digest, SourceRelative: "skills/docs", ProtectionClass: "managed"}
}

func assertTestBytes(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil || string(got) != want {
		t.Errorf("TEST bytes at %q = %q, %v; want %q", path, got, err, want)
	}
}

func skillEntries(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, "skills"))
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func TestOwnershipPathCannotRemoveAnotherProfile(t *testing.T) {
	base := testDirectory(t)
	rootA, rootB := filepath.Join(base, "TEST-A", ".gemini"), filepath.Join(base, "TEST-B", ".gemini")
	const body = "# TEST owned skill\n"
	object := ownedSkill(t, rootA, body)
	other := ownedSkill(t, rootB, body)
	anchor := filepath.Join(rootB, "skills", "anchor")
	if err := os.Mkdir(anchor, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(rootA, "skills", "alias")
	if err := os.Symlink(anchor, alias); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	// Keep the literal dot segment: Join would erase the cross-profile resolution.
	object.Path = alias + string(filepath.Separator) + ".." + string(filepath.Separator) + "docs"
	if physical, err := filepath.EvalSymlinks(object.Path); err != nil || physical != other.Path {
		t.Fatalf("TEST path resolves to %q, %v; want profile B", physical, err)
	}
	before := skillEntries(t, rootA)
	for _, recorded := range []string{"", rootA} {
		binding := domain.ClientBinding{NativeProfileRoot: recorded, NativeObjects: []domain.NativeObjectOwnership{object}}
		if err := gemini.New().ValidateBindingProfile(rootA, binding); err == nil {
			t.Errorf("ambiguous ownership accepted with recorded root %q", recorded)
		}
	}
	outcome, err := gemini.New().Deactivate(context.Background(), clients.Env{NativeConfig: nativeconfig.New()},
		domain.DeactivationRequest{Client: domain.DetectedClient{ClientID: domain.ClientGemini, ConfigRoot: rootA},
			Confirmed: true, NativeObjects: []domain.NativeObjectOwnership{object}})
	if err == nil || outcome.ExternalRemovalComplete {
		t.Errorf("confirmed removal accepted ambiguous ownership: %+v, %v", outcome, err)
	}
	assertTestBytes(t, filepath.Join(rootA, "skills", "docs", "SKILL.md"), body)
	assertTestBytes(t, filepath.Join(other.Path, "SKILL.md"), body)
	if got := skillEntries(t, rootA); !reflect.DeepEqual(got, before) {
		t.Errorf("refusal changed selected TEST skill entries: %v; want %v", got, before)
	}
}

func TestOwnershipPathsRequireLiteralDeterministicSpelling(t *testing.T) {
	base := testDirectory(t)
	root := filepath.Join(base, "TEST-profile", ".gemini")
	objects := []domain.NativeObjectOwnership{ownedSkill(t, root, "# TEST skill\n"), ownedMCP(t, root)}
	t.Chdir(base) // A relative spelling resolves to the same file but is not authority.
	for _, object := range objects {
		t.Run(object.Kind, func(t *testing.T) {
			relative, err := filepath.Rel(base, object.Path)
			if err != nil {
				t.Fatal(err)
			}
			sep := string(filepath.Separator)
			for _, path := range []string{relative, " " + object.Path, object.Path + " ", object.Path + sep,
				filepath.Dir(object.Path) + sep + "." + sep + filepath.Base(object.Path),
				filepath.Dir(object.Path) + sep + "absent" + sep + ".." + sep + filepath.Base(object.Path)} {
				bad := object
				bad.Path = path
				binding := domain.ClientBinding{NativeProfileRoot: root, NativeObjects: []domain.NativeObjectOwnership{bad}}
				if err := gemini.New().ValidateBindingProfile(root, binding); err == nil {
					t.Errorf("recorded root admitted ambiguous %s path %q", object.Kind, path)
				}
				if err := gemini.VerifyGeminiNativeObjects(root, binding.NativeObjects, true, nativeconfig.New()); err == nil {
					t.Errorf("allowMissing verification admitted ambiguous %s path %q", object.Kind, path)
				}
			}
		})
	}
}

func TestDesiredReplacementPathCannotSelectAnotherProfile(t *testing.T) {
	base := testDirectory(t)
	rootA, rootB := filepath.Join(base, "TEST-A", ".gemini"), filepath.Join(base, "TEST-B", ".gemini")
	previous := ownedSkill(t, rootA, "# TEST old skill\n")
	active := filepath.Join(base, "TEST-package")
	staged := ownedSkill(t, active, "# TEST new skill\n")
	writeTestFile(t, filepath.Join(active, gemini.GeminiDescriptorName), `{"data_root":"`+filepath.ToSlash(base)+`"}`)
	anchor := filepath.Join(rootB, "skills", "anchor")
	if err := os.MkdirAll(anchor, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(rootA, "skills", "alias")
	if err := os.Symlink(anchor, alias); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	desired := previous
	desired.ManagedDigest = staged.ManagedDigest
	desired.Path = alias + string(filepath.Separator) + ".." + string(filepath.Separator) + "docs"
	before := skillEntries(t, rootA)
	if err := gemini.ApplyGeminiNativeMutation(rootA, active, []domain.NativeObjectOwnership{previous}, []domain.NativeObjectOwnership{desired}); err == nil {
		t.Error("desired replacement accepted a path resolving into another TEST profile")
	}
	assertTestBytes(t, filepath.Join(previous.Path, "SKILL.md"), "# TEST old skill\n")
	if _, err := os.Lstat(filepath.Join(rootB, "skills", "docs")); !os.IsNotExist(err) {
		t.Errorf("refusal installed into profile B: %v", err)
	}
	if got := skillEntries(t, rootA); !reflect.DeepEqual(got, before) {
		t.Errorf("refusal changed selected TEST skill entries: %v; want %v", got, before)
	}
}

func TestSkillDirectoryProofRejectsForeignRegularFile(t *testing.T) {
	root := filepath.Join(testDirectory(t), "TEST-profile", ".gemini")
	object := ownedSkill(t, root, "")
	if err := os.Remove(object.Path); err != nil {
		t.Fatal(err)
	}
	const foreign = "TEST foreign regular file\n"
	writeTestFile(t, object.Path, foreign)
	before := skillEntries(t, root)
	if err := gemini.New().ValidateBindingProfile(root, domain.ClientBinding{NativeObjects: []domain.NativeObjectOwnership{object}}); err == nil {
		t.Error("legacy binding accepted a file as an empty skill directory")
	}
	for _, allowMissing := range []bool{false, true} {
		if err := gemini.VerifyGeminiNativeObjects(root, []domain.NativeObjectOwnership{object}, allowMissing, nativeconfig.New()); err == nil {
			t.Errorf("verification accepted a regular file with allowMissing=%v", allowMissing)
		}
	}
	outcome, err := gemini.New().Deactivate(context.Background(), clients.Env{NativeConfig: nativeconfig.New()},
		domain.DeactivationRequest{Client: domain.DetectedClient{ClientID: domain.ClientGemini, ConfigRoot: root},
			Confirmed: true, NativeObjects: []domain.NativeObjectOwnership{object}})
	if err == nil || outcome.ExternalRemovalComplete {
		t.Errorf("confirmed removal accepted foreign file: %+v, %v", outcome, err)
	}
	assertTestBytes(t, object.Path, foreign)
	if got := skillEntries(t, root); !reflect.DeepEqual(got, before) {
		t.Errorf("refusal changed TEST skill entries: %v; want %v", got, before)
	}
}

func TestSkillProofDistinguishesEmptyDirectoryFromMissing(t *testing.T) {
	root := filepath.Join(testDirectory(t), "TEST-profile", ".gemini")
	object := ownedSkill(t, root, "")
	binding := domain.ClientBinding{NativeObjects: []domain.NativeObjectOwnership{object}}
	if err := gemini.New().ValidateBindingProfile(root, binding); err != nil {
		t.Fatalf("actual empty directory must prove legacy ownership: %v", err)
	}
	if err := os.Remove(object.Path); err != nil {
		t.Fatal(err)
	}
	if err := gemini.New().ValidateBindingProfile(root, binding); err == nil {
		t.Error("missing directory proved legacy ownership")
	}
	binding.NativeProfileRoot = root
	if err := gemini.New().ValidateBindingProfile(root, binding); err != nil {
		t.Errorf("matching recorded root must admit separately authorized missing-object removal: %v", err)
	}
	for _, allowMissing := range []bool{false, true} {
		err := gemini.VerifyGeminiNativeObjects(root, binding.NativeObjects, allowMissing, nativeconfig.New())
		if (err == nil) != allowMissing {
			t.Errorf("missing verification with allowMissing=%v: %v", allowMissing, err)
		}
	}
	outcome, err := gemini.New().Deactivate(context.Background(), clients.Env{NativeConfig: nativeconfig.New()},
		domain.DeactivationRequest{Client: domain.DetectedClient{ClientID: domain.ClientGemini, ConfigRoot: root},
			Confirmed: true, NativeObjects: binding.NativeObjects})
	if err != nil || !outcome.ExternalRemovalComplete {
		t.Errorf("confirmed missing-object removal = %+v, %v", outcome, err)
	}
	if got := skillEntries(t, root); len(got) != 0 {
		t.Errorf("missing-object removal left TEST entries: %v", got)
	}
}

func TestExactOwnershipPathStillRejectsSymlinkComponents(t *testing.T) {
	for _, kind := range []string{"skill leaf", "skills ancestor", "settings leaf"} {
		t.Run(kind, func(t *testing.T) {
			base := testDirectory(t)
			root, other := filepath.Join(base, "TEST-A", ".gemini"), filepath.Join(base, "TEST-B", ".gemini")
			object := ownedSkill(t, other, "# TEST foreign skill\n")
			object.Path = filepath.Join(root, "skills", "docs")
			link, target := object.Path, filepath.Join(other, "skills", "docs")
			if kind == "skills ancestor" {
				link, target = filepath.Dir(link), filepath.Dir(target)
			} else if kind == "settings leaf" {
				object = ownedMCP(t, other)
				object.Path = filepath.Join(root, "settings.json")
				link, target = object.Path, filepath.Join(other, "settings.json")
			}
			if err := os.MkdirAll(filepath.Dir(link), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, link); err != nil {
				t.Skipf("symlink unavailable: %v", err)
			}
			binding := domain.ClientBinding{NativeProfileRoot: root, NativeObjects: []domain.NativeObjectOwnership{object}}
			if err := gemini.New().ValidateBindingProfile(root, binding); err == nil {
				t.Error("exact spelling bypassed symlink containment")
			}
			if err := gemini.VerifyGeminiNativeObjects(root, binding.NativeObjects, true, nativeconfig.New()); err == nil {
				t.Error("allowMissing bypassed symlink containment")
			}
			assertTestBytes(t, filepath.Join(other, "skills", "docs", "SKILL.md"), "# TEST foreign skill\n")
		})
	}
}
