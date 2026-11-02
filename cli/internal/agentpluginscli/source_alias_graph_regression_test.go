package agentpluginscli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

type aliasGraphFixture struct {
	cli        cliFixture
	transport  *directTransportFixture
	root       environment.Requirement
	mu         sync.Mutex
	requests   map[string]int
	archives   map[string][]byte
	childNames []string
}

func aliasGraphPackage(t *testing.T, root, name string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "skills", name), 0700); err != nil {
		t.Fatal(err)
	}
	dependencyWrite(t, filepath.Join(root, "plugin.json"), fmt.Sprintf(`{"$schema":%q,"name":%q,"version":"1.0.0"}`, domain.PluginSchemaV1, name))
	dependencyWrite(t, filepath.Join(root, "skills", name, "SKILL.md"), fmt.Sprintf("---\nname: %s\ndescription: Inert transitive fixture\n---\nText only.\n", name))
}

func newAliasGraphFixture(t *testing.T, kinds []string, rootAlias string, collision bool) *aliasGraphFixture {
	t.Helper()
	f := &aliasGraphFixture{transport: newDirectTransportFixture(t, "zip", false), requests: map[string]int{}, archives: map[string][]byte{}, childNames: []string{"first-name", "second-name"}}
	if collision {
		f.childNames[1] = f.childNames[0]
	}
	oldHandler := f.transport.server.Config.Handler
	f.transport.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.requests[r.URL.Path]++
		payload, archive := f.archives[r.URL.Path]
		f.mu.Unlock()
		if archive {
			_, _ = w.Write(payload)
			return
		}
		oldHandler.ServeHTTP(w, r)
	})
	manifest := "schema_version = 1\n"
	for i, kind := range kinds {
		name := f.childNames[i]
		path := fmt.Sprintf("plugins/child-%d", i)
		manifest += fmt.Sprintf("[dependencies.local-%d]\n", i)
		if kind == "git" {
			aliasGraphPackage(t, filepath.Join(f.transport.work, path), name)
			manifest += fmt.Sprintf("git = %q\npath = %q\n", f.transport.server.URL+"/repo.git", path)
		} else {
			root := t.TempDir()
			aliasGraphPackage(t, root, name)
			endpoint := fmt.Sprintf("/child-%d.zip", i)
			f.archives[endpoint] = directArchiveBytes(t, root, "zip", "publisher-wrapper/")
			manifest += fmt.Sprintf("archive = %q\n", f.transport.server.URL+endpoint)
		}
	}
	if strings.Contains(strings.Join(kinds, ","), "git") {
		dependencyGit(t, f.transport.work, "add", ".")
		dependencyGit(t, f.transport.work, "commit", "-m", "inert URL Git graph packages")
		dependencyGit(t, f.transport.work, "push", "--quiet", f.transport.remote, "main")
	}
	root := t.TempDir()
	aliasGraphPackage(t, root, "root-manifest")
	dependencyWrite(t, filepath.Join(root, "uap.toml"), manifest)
	f.archives["/root.zip"] = directArchiveBytes(t, root, "zip", "root-wrapper/")
	f.root = environment.Requirement{Alias: rootAlias, Archive: &environment.ArchiveSource{URL: f.transport.server.URL + "/root.zip"}}
	f.cli = directCLIEnvironment(t, f.transport)
	return f
}

func (f *aliasGraphFixture) requirements() environment.Requirements {
	return environment.Requirements{SchemaVersion: 1, Dependencies: map[string]environment.Requirement{f.root.Alias: f.root}, Targets: map[string][]string{"gemini": {f.root.Alias}}}
}

func (f *aliasGraphFixture) resolve(t *testing.T, requirements environment.Requirements, previous environment.Lock, update string) environment.Lock {
	t.Helper()
	lock, err := (environment.Coordinator{Resolver: dependencyResolver{app: f.cli.app, requirements: requirements}}).Resolve(context.Background(), requirements, previous, update)
	if err != nil {
		t.Fatal(err)
	}
	return lock
}

func (f *aliasGraphFixture) requestCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	count := 0
	for _, n := range f.requests {
		count += n
	}
	return count
}

func TestDirectTransitiveArchivesAndURLGitUseAcquiredNames(t *testing.T) {
	for _, kinds := range [][]string{{"archive", "archive"}, {"git", "git"}, {"git", "archive"}} {
		t.Run(strings.Join(kinds, "+"), func(t *testing.T) {
			f := newAliasGraphFixture(t, kinds, "candidate", false)
			requirements := f.requirements()
			lock := f.resolve(t, requirements, environment.Lock{}, "")
			if len(lock.Packages) != 3 {
				t.Fatalf("lost distinct source tuples: %+v", lock.Packages)
			}
			aliases := map[string]environment.LockedPackage{}
			for _, pin := range lock.Packages {
				aliases[pin.Alias] = pin
				if pin.SourceKey == "" || strings.Contains(pin.Alias, "source-acquisition") {
					t.Fatal("operational identity entered lock", pin)
				}
			}
			if aliases["candidate"].Source.Archive == nil || aliases["candidate"].Source.Archive.URL != f.root.Archive.URL {
				t.Fatal("explicit candidate root alias was replaced")
			}
			for _, name := range f.childNames {
				pin, ok := aliases[name]
				if !ok || len(pin.RequiredBy) != 1 || pin.RequiredBy[0].Alias != "candidate" {
					t.Fatalf("acquired manifest/edge identity missing: %s %+v", name, pin)
				}
			}
			before := f.requestCount()
			replayed := f.resolve(t, requirements, lock, "")
			if !reflect.DeepEqual(lock, replayed) || f.requestCount() != before {
				t.Fatal("exact replay changed pin or reacquired intact sources")
			}
		})
	}
}

func TestDirectTransitivePinnedCandidateSurvivesExplicitUpdate(t *testing.T) {
	for _, kind := range []string{"archive", "git"} {
		t.Run(kind, func(t *testing.T) {
			f := newAliasGraphFixture(t, []string{kind, kind}, "root", false)
			requirements := f.requirements()
			lock := f.resolve(t, requirements, environment.Lock{}, "")
			var pinned environment.LockedPackage
			for i := range lock.Packages {
				if lock.Packages[i].Alias == f.childNames[0] {
					lock.Packages[i].Alias = "candidate"
					pinned = lock.Packages[i]
				}
			}
			sort.Slice(lock.Packages, func(i, j int) bool { return lock.Packages[i].Alias < lock.Packages[j].Alias })
			before := f.requestCount()
			replayed := f.resolve(t, requirements, lock, "")
			if !reflect.DeepEqual(lock, replayed) || f.requestCount() != before {
				t.Fatal("pinned literal candidate was replaced or reacquired")
			}
			requirements.Dependencies["candidate"] = pinned.Requirement()
			requirements.Targets["gemini"] = []string{"root", "candidate"}
			updated := f.resolve(t, requirements, replayed, "candidate")
			if f.requestCount() <= before || !reflect.DeepEqual(updated.Packages, replayed.Packages) {
				t.Fatalf("moving explicit candidate did not reacquire exact pin: before%d after%d %+v", before, f.requestCount(), updated.Packages)
			}
		})
	}
}

func TestDirectTransitiveRealManifestAliasCollisionRefuses(t *testing.T) {
	f := newAliasGraphFixture(t, []string{"archive", "git"}, "root", true)
	requirements := f.requirements()
	_, err := (environment.Coordinator{Resolver: dependencyResolver{app: f.cli.app, requirements: requirements}}).Resolve(context.Background(), requirements, environment.Lock{}, "")
	if err == nil || !strings.Contains(err.Error(), "collides") {
		t.Fatalf("real acquired alias collision was renamed or accepted: %v", err)
	}
}
