package sourceacquisition

import (
	"context"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/cgi"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

func TestNativeGitEnvironmentSeparatesAuthenticationFromRouting(t *testing.T) {
	env := nativeGitEnvironment([]string{"HOME=/user", "SSH_AUTH_SOCK=/agent", "GIT_ASKPASS=/helper", "GIT_SSH_COMMAND=ssh -F /config", "GIT_SSL_CAINFO=/ca", "GIT_DIR=/evil", "GIT_WORK_TREE=/evil", "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=http.extraHeader", "GIT_CONFIG_VALUE_0=secret", "GIT_TRACE=/leak", "GIT_SSL_NO_VERIFY=1"})
	joined := strings.Join(env, "\n")
	for _, wanted := range []string{"HOME=/user", "SSH_AUTH_SOCK=/agent", "GIT_ASKPASS=/helper", "GIT_SSH_COMMAND=ssh -F /config", "GIT_SSL_CAINFO=/ca", "GIT_TERMINAL_PROMPT=0", "GCM_INTERACTIVE=Never", "GIT_ALLOW_PROTOCOL=https:ssh"} {
		if !strings.Contains(joined, wanted) {
			t.Fatalf("missing native auth input %s", wanted)
		}
	}
	for _, forbidden := range []string{"GIT_DIR=", "GIT_WORK_TREE=", "GIT_CONFIG_", "GIT_TRACE=", "GIT_SSL_NO_VERIFY=1"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("unsafe native environment %s", forbidden)
		}
	}
	anonymous := strings.Join(isolatedGitEnvironment(env, "/private"), "\n")
	if strings.Contains(anonymous, "SSH_AUTH_SOCK=") || strings.Contains(anonymous, "GIT_ASKPASS=") || strings.Contains(anonymous, "HOME=/user") {
		t.Fatal("signed anonymous environment acquired native credentials")
	}
}

func nativeFixtureGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.CommandContext(t.Context(), "git", args...)
	command.Dir = dir
	command.Env = isolatedGitEnvironment(os.Environ(), dir)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("fixture git %s: %v: %s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func nativeFixtureRepository(t *testing.T) (root, work, commit string) {
	t.Helper()
	root = t.TempDir()
	work = filepath.Join(root, "work")
	if err := os.Mkdir(work, 0o700); err != nil {
		t.Fatal(err)
	}
	nativeFixtureGit(t, work, "init", "--quiet")
	for name, body := range map[string]string{"plugin.json": `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"transport-demo","version":"1.0.0"}`, "skills/demo/SKILL.md": "---\nname: transport-demo\ndescription: Transport fixture\n---\nDo not execute package payloads.\n", "payload": "inert bytes\n", ".gitattributes": "payload filter=trap\n"} {
		target := filepath.Join(work, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	nativeFixtureGit(t, work, "add", ".")
	nativeFixtureGit(t, work, "-c", "user.name=Source Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--quiet", "-m", "fixture")
	commit = nativeFixtureGit(t, work, "rev-parse", "HEAD")
	nativeFixtureGit(t, root, "clone", "--quiet", "--bare", work, filepath.Join(root, "repo.git"))
	return root, work, commit
}

func nativeHTTPSFixture(t *testing.T, root string, private bool) *httptest.Server {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	backend := &cgi.Handler{Path: git, Args: []string{"http-backend"}, Env: []string{"GIT_PROJECT_ROOT=" + root, "GIT_HTTP_EXPORT_ALL=1"}}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if private {
			user, password, ok := r.BasicAuth()
			if !ok || user != "fixture" || password != "fixture-token-never-log" {
				w.Header().Set("WWW-Authenticate", `Basic realm="fixture"`)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}
		backend.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	caPath := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caPath, ca, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_SSL_CAINFO", caPath)
	return server
}

func TestNativeHTTPSGitPublicAndPrivateCredentialHelper(t *testing.T) {
	for _, private := range []bool{false, true} {
		t.Run(fmt.Sprint(private), func(t *testing.T) {
			root, _, commit := nativeFixtureRepository(t)
			server := nativeHTTPSFixture(t, root, private)
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_CONFIG_HOME", home)
			endpoint := server.URL + "/repo.git"
			source := sourcespec.Git{URL: endpoint}
			acquirer := Acquirer{TempRoot: t.TempDir()}
			if private {
				_, err := acquirer.ResolveGit(t.Context(), source, "default", false)
				if err == nil || strings.Contains(err.Error(), "fixture-token-never-log") {
					t.Fatalf("unprovisioned private source accepted or leaked credentials: %v", err)
				}
			}
			configureNativeFixtureCredentials(t, home, server.URL, private)
			resolved, err := acquirer.ResolveGitSource(t.Context(), source)
			if err != nil {
				t.Fatal(err)
			}
			if resolved.Resolution.Commit != commit {
				t.Fatalf("resolved %s, expected %s", resolved.Resolution.Commit, commit)
			}
			snapshot, err := acquirer.AcquireGit(t.Context(), source, commit, "")
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = packagedigest.Remove(snapshot) }()
			if snapshot.Source.SourceURL != endpoint || snapshot.Source.ResolvedRevision != commit || snapshot.Source.SourceKind != "git" {
				t.Fatalf("wrong immutable identity %+v", snapshot.Source)
			}
			if _, err := os.Stat(filepath.Join(home, "filter-ran")); !os.IsNotExist(err) {
				t.Fatal("package attributes executed a user smudge filter")
			}
			if _, err := os.Stat(filepath.Join(home, "hook-ran")); !os.IsNotExist(err) {
				t.Fatal("fetch executed a configured hook")
			}
			if snapshot.TreeDigest == "" {
				t.Fatal("unsealed native snapshot")
			}
		})
	}
}

func configureNativeFixtureCredentials(t *testing.T, home, origin string, private bool) {
	t.Helper()
	hooks := filepath.Join(home, "hooks")
	if err := os.Mkdir(hooks, 0o700); err != nil {
		t.Fatal(err)
	}
	hook := "#!/bin/sh\ntouch '" + filepath.Join(home, "hook-ran") + "'\n"
	if err := os.WriteFile(filepath.Join(hooks, "reference-transaction"), []byte(hook), 0o700); err != nil {
		t.Fatal(err)
	}
	config := "[core]\n hooksPath = " + hooks + "\n[filter \"trap\"]\n smudge = touch '" + filepath.Join(home, "filter-ran") + "'; cat\n required = true\n"
	if private {
		config += "[credential \"" + origin + "\"]\n helper = \"!f() { printf 'username=fixture\\npassword=fixture-token-never-log\\n'; }; f\"\n useHttpPath = true\n"
	}
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestNativeGitBrowserSlashRefAndAmbiguity(t *testing.T) {
	root, work, commit := nativeFixtureRepository(t)
	nativeFixtureGit(t, work, "branch", "feature/a")
	nativeFixtureGit(t, work, "push", "--quiet", filepath.Join(root, "repo.git"), "feature/a")
	server := nativeHTTPSFixture(t, root, false)
	t.Setenv("HOME", t.TempDir())
	acquirer := Acquirer{TempRoot: t.TempDir()}
	source := sourcespec.Git{URL: server.URL + "/repo.git", Browser: &sourcespec.BrowserTree{Provider: "gitlab", Suffix: "feature/a/pkg"}}
	resolved, err := acquirer.ResolveGitSource(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Git.Path != "pkg" || resolved.Git.Ref != "refs/heads/feature/a" || resolved.Git.Commit != commit || resolved.Git.Browser != nil {
		t.Fatalf("browser coordinates changed %+v", resolved)
	}
	nativeFixtureGit(t, work, "tag", "feature")
	nativeFixtureGit(t, work, "push", "--quiet", filepath.Join(root, "repo.git"), "refs/tags/feature")
	if _, err := acquirer.ResolveGitSource(t.Context(), source); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("ambiguous browser branch/path accepted: %v", err)
	}
}
