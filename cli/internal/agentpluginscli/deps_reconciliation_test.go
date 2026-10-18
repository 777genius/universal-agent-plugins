package agentpluginscli

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func reconciliationHarness(t *testing.T) (cliFixture, *dependencyTestSource, string) {
	t.Helper()
	repo := dependencyRepo(t)
	f, source, _ := dependencySpecHarness(t, repo, dependencyClientSpec{id: domain.ClientCodex, executable: true, activation: "active", externalUninstall: true})
	mp, _ := f.app.dependencyPaths()
	dependencyWrite(t, mp, "schema_version = 1\n[dependencies.demo]\ngit = \"acme/demo\"\nref = \"main\"\n[targets]\ncodex = [\"demo\"]\n")
	dependencyRun(t, f, "install", "--yes", "--format", "json")
	return f, source, repo
}

func reconciliationFiles(t *testing.T, f cliFixture, projection string) map[string]string {
	t.Helper()
	mp, lp := f.app.dependencyPaths()
	result := map[string]string{}
	for _, p := range []string{mp, lp, f.store.Path} {
		body, err := os.ReadFile(p)
		if os.IsNotExist(err) {
			result[p] = "<absent>"
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		result[p] = string(body)
	}
	if projection != "" {
		err := filepath.WalkDir(projection, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			body, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			result[p] = string(body)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return result
}

func assertReconciliationRefusal(t *testing.T, f cliFixture, source *dependencyTestSource, projection string, args ...string) {
	t.Helper()
	before := reconciliationFiles(t, f, projection)
	downloads, lookups := source.downloads, source.lookups
	_, _, err := f.execute(false, args...)
	if err == nil {
		t.Fatalf("accepted inconsistent dependency lock: %v", args)
	}
	if !reflect.DeepEqual(before, reconciliationFiles(t, f, projection)) {
		t.Fatal("refusal changed manifest, lock, state or projection")
	}
	if source.downloads != downloads || source.lookups != lookups {
		t.Fatalf("refusal acquired source: downloads %d -> %d lookups %d -> %d", downloads, source.downloads, lookups, source.lookups)
	}
}

// Intent/structure errors must fail before acquisition, publication or repair.
func TestReconciliationCLIRejectsMissingOrChangedLock(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*testing.T, cliFixture)
	}{
		{"missing", func(t *testing.T, f cliFixture) {
			_, lp := f.app.dependencyPaths()
			if err := os.Remove(lp); err != nil {
				t.Fatal(err)
			}
		}},
		{"targets", func(t *testing.T, f cliFixture) {
			mp, _ := f.app.dependencyPaths()
			dependencyWrite(t, mp, strings.ReplaceAll(dependencyRead(t, mp), `codex = ["demo"]`, `codex = []`))
		}},
		{"intent digest", func(t *testing.T, f cliFixture) {
			mutateReconciliationLock(t, f, func(l *environment.Lock) { l.ManifestDigest = "sha256:" + strings.Repeat("f", 64) })
		}},
		{"source identity", func(t *testing.T, f cliFixture) {
			mutateReconciliationLock(t, f, func(l *environment.Lock) { l.Packages[0].SourceKey = "github:acme/other:" })
		}},
		{"commit identity", func(t *testing.T, f cliFixture) {
			mutateReconciliationLock(t, f, func(l *environment.Lock) { l.Packages[0].Source.Git.Commit = "main" })
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, source, _ := reconciliationHarness(t)
			binding := dependencyBinding(t, f)
			dependencyWrite(t, filepath.Join(binding.TargetPath, "user-edit.txt"), "preserve drift on refused install")
			tc.change(t, f)
			for _, verb := range []string{"install", "deps sync"} {
				args := strings.Fields(verb)
				args = append(args, "--locked", "--yes", "--format", "json")
				assertReconciliationRefusal(t, f, source, binding.TargetPath, args...)
			}
		})
	}
}

func mutateReconciliationLock(t *testing.T, f cliFixture, change func(*environment.Lock)) {
	t.Helper()
	_, lp := f.app.dependencyPaths()
	var locked environment.Lock
	if err := json.Unmarshal([]byte(dependencyRead(t, lp)), &locked); err != nil {
		t.Fatal(err)
	}
	change(&locked)
	body, err := json.MarshalIndent(locked, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	dependencyWrite(t, lp, string(body))
}

// A matching intent digest cannot hide omitted declarations or a different
// pinned manifest. Both install forms must reject before repairing drift.
func TestReconciliationCLIValidatesPinnedDeclarations(t *testing.T) {
	for _, tc := range []struct {
		name                            string
		declares, flat, version, tamper bool
		accept                          bool
	}{
		{name: "missing edge", declares: true},
		{name: "legacy flat with declarations", declares: true, flat: true},
		{name: "pinned version mismatch", version: true},
		{name: "corrupt stored bytes", tamper: true},
		{name: "legacy flat without declarations", flat: true, accept: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, source, repo := reconciliationHarness(t)
			binding := dependencyBinding(t, f)
			mp, lp := f.app.dependencyPaths()
			req, err := readDependencyManifest(mp)
			if err != nil {
				t.Fatal(err)
			}
			if tc.declares {
				dependencyWrite(t, filepath.Join(repo, "uap.toml"), "schema_version = 1\n[dependencies]\n\"acme/missing\" = \"^1.2\"\n")
				dependencyGit(t, repo, "add", ".")
				dependencyGit(t, repo, "commit", "-m", "test: pinned dependency declaration")
				pin, err := (dependencyResolver{app: f.app, requirements: req}).Resolve(context.Background(), req.Dependencies["demo"], []string{"codex"}, nil, true)
				if err != nil {
					t.Fatal(err)
				}
				pin.Targets = []string{"codex"}
				mutateReconciliationLock(t, f, func(l *environment.Lock) { l.Packages = []environment.LockedPackage{pin} })
			}
			if tc.flat || tc.version {
				mutateReconciliationLock(t, f, func(l *environment.Lock) {
					if tc.flat {
						l.Packages[0].Version = ""
						l.Packages[0].RequiredBy = nil
					} else {
						l.Packages[0].Version = "99.0.0"
					}
				})
			}
			if tc.tamper {
				var lock environment.Lock
				if err := json.Unmarshal([]byte(dependencyRead(t, lp)), &lock); err != nil {
					t.Fatal(err)
				}
				pin := lock.Packages[0]
				stored, err := f.app.dependencyStore().Get(context.Background(), pin.DigestAlgorithm, pin.TreeDigest)
				if err != nil {
					t.Fatal(err)
				}
				dependencyWrite(t, filepath.Join(stored.Root, "tampered.txt"), "changed stored immutable bytes")
			}
			if tc.accept {
				before := reconciliationFiles(t, f, binding.TargetPath)
				downloads, lookups := source.downloads, source.lookups
				dependencyRun(t, f, "install", "--locked", "--yes", "--format", "json")
				if !reflect.DeepEqual(before, reconciliationFiles(t, f, binding.TargetPath)) || source.downloads != downloads || source.lookups != lookups {
					t.Fatal("legacy flat replay changed files or acquired sources")
				}
				return
			}
			dependencyWrite(t, filepath.Join(binding.TargetPath, "user-edit.txt"), "retain drift until graph is validated")
			for _, locked := range []bool{true, false} {
				args := []string{"install", "--yes", "--format", "json"}
				if locked {
					args = append(args, "--locked")
				}
				assertReconciliationRefusal(t, f, source, binding.TargetPath, args...)
			}
		})
	}
}
