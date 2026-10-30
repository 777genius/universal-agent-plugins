package agentpluginscli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func transferEmptyAgreement(t *testing.T) freshRestoreEnvironment {
	t.Helper()
	fixture, _, _ := dependencyReviewFixture(t)
	mp, _ := fixture.app.dependencyPaths()
	dependencyWrite(t, mp, "schema_version = 1\n# retain this comment and CRLF below\r\n[dependencies]\n[targets]\n")
	dependencyRun(t, fixture.cliFixture, "deps", "lock", "--format", "json")
	return fixture
}

// Replacing an agreement is a user decision even when the destination is broken.
// Conversely, byte-identical transfers must preserve inode metadata and bypass
// the mutation lock; a mere semantic equality check loses comments and formatting.
func TestDependencyTransferReplacementAndNoop(t *testing.T) {
	for _, direction := range []string{"export", "import"} {
		for _, scenario := range []string{"confirm", "no confirmation", "decline", "dry-run", "identical", "invalid destination"} {
			t.Run(direction+"/"+scenario, func(t *testing.T) {
				fixture := transferEmptyAgreement(t)
				mp, lp := fixture.app.dependencyPaths()
				manifest, lock := dependencyRead(t, mp), dependencyRead(t, lp)
				exchange := t.TempDir()
				source, destination := filepath.Dir(mp), exchange
				if direction == "import" {
					source, destination = exchange, filepath.Dir(mp)
				}
				transferPair(t, source, manifest, lock)
				oldManifest, oldLock := "# previous bytes\n"+manifest, lock+"\n"
				switch scenario {
				case "identical":
					oldManifest, oldLock = manifest, lock
				case "invalid destination":
					oldManifest, oldLock = "not a dependency agreement", "{crash-mixed lock"
				}
				transferPair(t, destination, oldManifest, oldLock)
				if err := os.Chmod(filepath.Join(destination, "uap.toml"), 0640); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(filepath.Join(destination, "uap.lock"), 0644); err != nil {
					t.Fatal(err)
				}
				before := freshRestoreFiles(t, fixture.root, exchange)
				args := []string{"deps", direction, exchange, "--format", "json"}
				if scenario == "confirm" || scenario == "invalid destination" {
					args = append(args, "--yes")
				}
				if scenario == "dry-run" {
					args = append(args, "--dry-run")
				}
				if scenario == "identical" {
					// Keep the held lock outside snapshots: Windows denies reading its locked bytes.
					fixture.app.EnvironmentLockPath = filepath.Join(t.TempDir(), "environment.lock")
					release, err := (processlock.Lock{Path: fixture.app.EnvironmentLockPath}).Acquire(context.Background())
					if err != nil {
						t.Fatal(err)
					}
					defer func() { _ = release() }()
					before = freshRestoreFiles(t, fixture.root, exchange)
				}
				var out string
				var err error
				if scenario == "decline" {
					out, _, err = fixture.executeInput(true, "n\n", "deps", direction, exchange)
					if err != nil || !strings.Contains(out, "No changes made") {
						t.Fatalf("decline: %v %s", err, out)
					}
				} else {
					out, _, err = fixture.execute(false, args...)
				}
				if scenario == "no confirmation" {
					if err == nil {
						t.Fatal("non-TTY replacement accepted without --yes")
					}
				} else if err != nil {
					t.Fatalf("transfer: %v\n%s", err, out)
				}
				if scenario == "confirm" || scenario == "invalid destination" {
					result := transferJSON(t, out)
					if string(result["outcome"]) != `"completed"` || string(result["changed"]) != "true" {
						t.Fatalf("replacement not reported: %s", out)
					}
					if scenario == "invalid destination" {
						var diagnostic string
						if err := json.Unmarshal(result["destination_diagnostic"], &diagnostic); err != nil || !strings.Contains(diagnostic, "uap.toml:1:") || !strings.Contains(diagnostic, "toml:") {
							t.Fatalf("invalid destination TOML diagnostic was discarded: %s", out)
						}
					}
					if dependencyRead(t, filepath.Join(destination, "uap.toml")) != manifest || dependencyRead(t, filepath.Join(destination, "uap.lock")) != lock {
						t.Fatal("confirmed replacement did not copy exact bytes")
					}
					if runtime.GOOS != "windows" {
						for name, mode := range map[string]os.FileMode{"uap.toml": 0640, "uap.lock": 0644} {
							info, err := os.Stat(filepath.Join(destination, name))
							if err != nil || info.Mode().Perm() != mode {
								t.Fatalf("replacement changed existing %s mode: %v %v", name, info, err)
							}
						}
					}
				} else if !reflect.DeepEqual(before, freshRestoreFiles(t, fixture.root, exchange)) {
					t.Fatal("unapproved/preview/identical transfer changed bytes, modes or modification times")
				}
				if fixture.source.downloads != 0 || fixture.source.lookups != 0 || fixture.runner.calls != 0 {
					t.Fatal("plain transfer performed acquisition or native work")
				}
			})
		}
	}
}

// Strict offline validation must happen before even creating an export folder.
func TestDependencyTransferInvalidSourceAndDryRun(t *testing.T) {
	for _, direction := range []string{"export", "import"} {
		for _, corrupt := range []string{"missing manifest", "missing lock", "empty manifest", "empty lock", "unknown manifest", "unknown lock schema", "trailing JSON", "stale digest", "symlink", "directory", "dry-run"} {
			t.Run(direction+"/"+corrupt, func(t *testing.T) {
				fixture := transferEmptyAgreement(t)
				mp, lp := fixture.app.dependencyPaths()
				manifest, lock := dependencyRead(t, mp), dependencyRead(t, lp)
				exchange := filepath.Join(t.TempDir(), "exchange")
				source := filepath.Dir(mp)
				if direction == "import" {
					source = exchange
					transferPair(t, source, manifest, lock)
				}
				smp, slp := filepath.Join(source, "uap.toml"), filepath.Join(source, "uap.lock")
				switch corrupt {
				case "missing manifest":
					if err := os.Remove(smp); err != nil {
						t.Fatal(err)
					}
				case "missing lock":
					if err := os.Remove(slp); err != nil {
						t.Fatal(err)
					}
				case "empty manifest":
					dependencyWrite(t, smp, "")
				case "empty lock":
					dependencyWrite(t, slp, "")
				case "unknown manifest":
					dependencyWrite(t, smp, manifest+"\nunknown_field = true\n")
				case "unknown lock schema":
					var value map[string]any
					if err := json.Unmarshal([]byte(lock), &value); err != nil {
						t.Fatal(err)
					}
					value["schema_version"] = 999
					body, _ := json.Marshal(value)
					dependencyWrite(t, slp, string(body))
				case "trailing JSON":
					dependencyWrite(t, slp, lock+"{}")
				case "stale digest":
					dependencyWrite(t, smp, "schema_version = 1\n[dependencies.x]\ngit = \"acme/x\"\nref = \"main\"\n[targets]\ncodex = [\"x\"]\n")
				case "symlink":
					if runtime.GOOS == "windows" {
						t.Skip("symlink permissions are host dependent")
					}
					target := filepath.Join(t.TempDir(), "target")
					dependencyWrite(t, target, manifest)
					if err := os.Remove(smp); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(target, smp); err != nil {
						t.Fatal(err)
					}
				case "directory":
					if err := os.Remove(slp); err != nil {
						t.Fatal(err)
					}
					if err := os.Mkdir(slp, 0700); err != nil {
						t.Fatal(err)
					}
				}
				if direction == "import" {
					if err := os.RemoveAll(fixture.app.DependenciesRoot); err != nil {
						t.Fatal(err)
					}
				}
				before := freshRestoreFiles(t, fixture.root, exchange)
				args := []string{"deps", direction, exchange, "--yes", "--format", "json"}
				if corrupt == "dry-run" {
					args = append(args, "--dry-run")
				}
				out, _, err := fixture.execute(false, args...)
				if corrupt == "dry-run" {
					if err != nil {
						t.Fatal(err)
					}
					transferJSON(t, out)
				} else if err == nil {
					t.Fatalf("invalid source %s was accepted", corrupt)
				}
				if !reflect.DeepEqual(before, freshRestoreFiles(t, fixture.root, exchange)) || fixture.source.downloads != 0 || fixture.source.lookups != 0 || fixture.runner.calls != 0 {
					t.Fatal("invalid source/dry-run produced durable or installation effects")
				}
			})
		}
	}
}

type transferReadHook struct {
	io.Reader
	once func()
}

func (r *transferReadHook) Read(p []byte) (int, error) {
	if r.once != nil {
		callback := r.once
		r.once = nil
		callback()
	}
	return r.Reader.Read(p)
}

// External edits made after the preview must invalidate consent. Cancellation
// at this same boundary must precede the first durable effect.
func TestDependencyTransferConfirmationRecheck(t *testing.T) {
	for _, direction := range []string{"export", "import"} {
		for _, changed := range []string{"source", "destination", "cancel"} {
			t.Run(direction+"/"+changed, func(t *testing.T) {
				fixture := transferEmptyAgreement(t)
				mp, lp := fixture.app.dependencyPaths()
				manifest, lock := dependencyRead(t, mp), dependencyRead(t, lp)
				exchange := t.TempDir()
				source, destination := filepath.Dir(mp), exchange
				if direction == "import" {
					source, destination = exchange, filepath.Dir(mp)
				}
				transferPair(t, source, manifest, lock)
				transferPair(t, destination, "# old\n"+manifest, lock+"\n")
				beforeSource, beforeDestination := freshRestoreFiles(t, source), freshRestoreFiles(t, destination)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				var out bytes.Buffer
				app := fixture.app
				app.Terminal, app.Output, app.ErrorOutput = true, &out, io.Discard
				app.Input = &transferReadHook{Reader: strings.NewReader("y\n"), once: func() {
					if out.Len() == 0 || !strings.Contains(out.String(), "uap.toml") {
						t.Fatal("confirmation preceded the reviewable file preview")
					}
					if changed == "cancel" {
						cancel()
						return
					}
					root := source
					if changed == "destination" {
						root = destination
					}
					dependencyWrite(t, filepath.Join(root, "uap.toml"), "# concurrent edit\n"+manifest)
				}}
				command := NewRoot(app)
				command.SetArgs([]string{"deps", direction, exchange})
				err := command.ExecuteContext(ctx)
				if changed == "cancel" {
					if !errors.Is(err, context.Canceled) {
						t.Fatalf("cancellation lost: %v", err)
					}
				} else if !errors.Is(err, installer.ErrPlanChanged) {
					t.Fatalf("approved stale plan accepted: %v", err)
				}
				// Lock metadata is allowed on a recheck refusal; only pair preimages
				// and the external editor's bytes are authoritative here.
				if changed != "source" {
					for _, name := range []string{"uap.toml", "uap.lock"} {
						path := filepath.Join(source, name)
						if !reflect.DeepEqual(beforeSource[path], freshRestoreFiles(t, path)[path]) {
							t.Fatal("recheck changed source pair")
						}
					}
				}
				if changed != "destination" {
					for _, name := range []string{"uap.toml", "uap.lock"} {
						path := filepath.Join(destination, name)
						if !reflect.DeepEqual(beforeDestination[path], freshRestoreFiles(t, path)[path]) {
							t.Fatal("recheck changed destination pair")
						}
					}
				}
			})
		}
	}
}

func TestDependencyTransferPublicationRollback(t *testing.T) {
	for _, scenario := range []string{"existing", "new leaves", "renamed then failed", "rollback failed", "external edit", "external read-only toggle"} {
		t.Run(scenario, func(t *testing.T) {
			fixture := transferEmptyAgreement(t)
			mp, lp := fixture.app.dependencyPaths()
			manifest, lock := dependencyRead(t, mp), dependencyRead(t, lp)
			sourceBefore := freshRestoreFiles(t, fixture.app.DependenciesRoot)
			destination := t.TempDir()
			dmp, dlp := filepath.Join(destination, "uap.toml"), filepath.Join(destination, "uap.lock")
			oldManifest, oldLock := "# before\n"+manifest, lock+"\n"
			if scenario != "new leaves" {
				transferPair(t, destination, oldManifest, oldLock)
				if err := os.Chmod(dmp, 0640); err != nil {
					t.Fatal(err)
				}
			}
			publish := publishEnvironmentFile
			defer func() { publishEnvironmentFile = publish }()
			publishEnvironmentFile = func(path string, body []byte) error {
				if path == dlp && string(body) == lock {
					if scenario == "renamed then failed" {
						if err := publish(path, body); err != nil {
							return err
						}
					}
					if scenario == "external edit" {
						dependencyWrite(t, dmp, "external editor owns these bytes")
					}
					if scenario == "external read-only toggle" {
						if err := os.Chmod(dmp, 0400); err != nil {
							t.Fatal(err)
						}
					}
					return errors.New("injected second publication failure")
				}
				if scenario == "rollback failed" && path == dmp && string(body) == oldManifest {
					return errors.New("injected rollback failure")
				}
				return publish(path, body)
			}
			out, _, err := fixture.execute(false, "deps", "export", destination, "--yes", "--format", "json")
			if err == nil || !strings.Contains(err.Error(), "injected second publication failure") {
				t.Fatalf("publication error lost: %v %s", err, out)
			}
			partial := scenario == "rollback failed" || scenario == "external edit" || scenario == "external read-only toggle"
			if partial {
				result := transferJSON(t, out)
				if string(result["outcome"]) != `"partial"` || !strings.Contains(err.Error(), "rollback incomplete") || !strings.Contains(err.Error(), fixture.app.DependenciesRoot) {
					t.Fatalf("partial rollback lacks truthful retry state: %v %s", err, out)
				}
				want := manifest
				if scenario == "external edit" {
					want = "external editor owns these bytes"
				}
				if dependencyRead(t, dmp) != want {
					t.Fatal("rollback discarded candidate/external bytes despite failure")
				}
				if scenario == "external read-only toggle" {
					info, statErr := os.Stat(dmp)
					if statErr != nil || info.Mode().Perm()&0200 != 0 || !strings.Contains(err.Error(), "mode changed externally") {
						t.Fatalf("rollback ignored external read-only state: %v %v %v", info, statErr, err)
					}
				}
			} else if scenario == "new leaves" {
				for _, path := range []string{dmp, dlp} {
					if _, err := os.Lstat(path); !os.IsNotExist(err) {
						t.Fatalf("rollback left new leaf %s: %v", path, err)
					}
				}
			} else {
				if dependencyRead(t, dmp) != oldManifest || dependencyRead(t, dlp) != oldLock {
					t.Fatal("rollback failed to restore pair preimages")
				}
				if runtime.GOOS != "windows" {
					info, err := os.Stat(dmp)
					if err != nil || info.Mode().Perm() != 0640 {
						t.Fatalf("rollback lost preimage mode: %v %v", info, err)
					}
				}
			}
			if !reflect.DeepEqual(sourceBefore, freshRestoreFiles(t, fixture.app.DependenciesRoot)) {
				t.Fatal("publication failure changed export source")
			}
			publishEnvironmentFile = publish
			if scenario == "external read-only toggle" {
				if err := os.Chmod(dmp, 0640); err != nil {
					t.Fatal(err)
				}
			}
			transferJSON(t, dependencyRun(t, fixture.cliFixture, "deps", "export", destination, "--yes", "--format", "json"))
			if dependencyRead(t, dmp) != manifest || dependencyRead(t, dlp) != lock || fixture.source.lookups != 0 || fixture.source.downloads != 0 {
				t.Fatal("retry failed to restore exact pair without resolution")
			}
		})
	}
}
