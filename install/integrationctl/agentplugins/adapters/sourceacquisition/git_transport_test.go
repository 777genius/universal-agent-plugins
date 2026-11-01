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
			diagnostic := &nativeTransportDiagnosticRunner{delegate: OSRunner{}, operation: "resolve", report: func(operation, class, status string) {
				t.Logf("native HTTPS Git diagnostic: operation=%s class=%s exit_status=%s", operation, class, status)
			}}
			acquirer := Acquirer{TempRoot: t.TempDir(), Runner: diagnostic}
			configureNativeFixtureCredentials(t, home, server.URL, false)
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
			diagnostic.operation = "acquire"
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
	if err := os.MkdirAll(hooks, 0o700); err != nil {
		t.Fatal(err)
	}
	hook := "#!/bin/sh\ntouch '" + filepath.Join(home, "hook-ran") + "'\n"
	if err := os.WriteFile(filepath.Join(hooks, "reference-transaction"), []byte(hook), 0o700); err != nil {
		t.Fatal(err)
	}
	// Reset inherited helpers and opt the fixture CA into Windows Schannel; see
	// https://git-scm.com/docs/gitcredentials and git-config#httpschannelUseSSLCAInfo.
	config := "[credential]\n helper =\n[http]\n schannelUseSSLCAInfo = true\n[core]\n hooksPath = " + hooks + "\n[filter \"trap\"]\n smudge = touch '" + filepath.Join(home, "filter-ran") + "'; cat\n required = true\n"
	if private {
		config += "[credential \"" + origin + "\"]\n helper = \"!f() { printf 'username=fixture\\npassword=fixture-token-never-log\\n'; }; f\"\n useHttpPath = true\n"
	}
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
}

// forcedCRLFCheckoutRunner emulates a Windows-style checkout default on every
// CI platform. It changes only an isolated, newly initialized Git fixture;
// production still performs the real fetch, checkout and sealed snapshot.
type forcedCRLFCheckoutRunner struct {
	delegate Runner
	injected bool
	checked  bool
}

func (runner *forcedCRLFCheckoutRunner) Run(ctx context.Context, command Command) ([]byte, error) {
	out, err := runner.delegate.Run(ctx, command)
	if err != nil {
		return out, err
	}
	if len(command.Args) == 3 && command.Args[0] == "init" && command.Args[1] == "--quiet" {
		// Simulate the native Windows CRLF default before production applies
		// its own isolated repository configuration.
		config := exec.CommandContext(ctx, "git", "-C", command.Args[2], "config", "core.eol", "crlf")
		config.Env = isolatedGitEnvironment(os.Environ(), command.Dir)
		if output, e := config.CombinedOutput(); e != nil {
			return nil, fmt.Errorf("configure disposable CRLF control: %w: %s", e, output)
		}
		runner.injected = true
	}
	if len(command.Args) >= 4 && command.Args[0] == "-C" && command.Args[2] == "checkout" {
		config := exec.CommandContext(ctx, "git", "-C", command.Args[1], "config", "--get", "core.eol")
		config.Env = isolatedGitEnvironment(os.Environ(), command.Dir)
		output, e := config.Output()
		if e != nil || strings.TrimSpace(string(output)) != "lf" {
			return nil, fmt.Errorf("immutable checkout did not override OS-dependent core.eol: %q (%v)", output, e)
		}
		runner.checked = true
	}
	return out, nil
}

// Git's core.autocrlf=false is not sufficient with .gitattributes text=auto:
// core.eol=crlf materializes CRLF from the same committed LF blob. Compare a
// actual immutable Git acquisition against that adversarial local default.
// Explicit per-path eol=crlf remains respected rather than rewritten to LF.
func TestImmutableGitCheckoutStableWithTextAutoAndExplicitEOL(t *testing.T) {
	root, work, _ := nativeFixtureRepository(t)
	if err := os.WriteFile(filepath.Join(work, ".gitattributes"),
		[]byte("payload text=auto\nexplicit.txt text eol=crlf\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "payload"), []byte("first\nsecond\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "explicit.txt"), []byte("explicit\nwindows\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	nativeFixtureGit(t, work, "add", ".gitattributes", "payload", "explicit.txt")
	nativeFixtureGit(t, work, "-c", "user.name=777genius",
		"-c", "user.email=iliyazelenkog@gmail.com",
		"commit", "--quiet", "-m", "test: textual attribute source")
	commit := nativeFixtureGit(t, work, "rev-parse", "HEAD")
	nativeFixtureGit(t, work, "push", "--quiet", filepath.Join(root, "repo.git"), "HEAD")

	// Keep the real Git fetch/checkout and its hardened process runner, but
	// replace only the remote transport with the already-supported local
	// fixture seam. Platform-specific self-signed TLS is tested separately;
	// an HTTPS certificate failure must not masquerade as an EOL regression.
	source := sourcespec.Git{URL: "https://github.com/example/portable-eol.git"}
	bare := filepath.Join(root, "repo.git")
	fixtureEndpoint := func(repository string) string {
		if repository != "example/portable-eol" {
			t.Fatalf("unexpected fixture repository %q", repository)
		}
		return bare
	}
	baseline, err := (Acquirer{TempRoot: t.TempDir(), Runner: OSRunner{}, URLForRepo: fixtureEndpoint}).
		AcquireGit(t.Context(), source, commit, "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = packagedigest.Remove(baseline) }()

	forced := &forcedCRLFCheckoutRunner{delegate: OSRunner{}}
	got, err := (Acquirer{TempRoot: t.TempDir(), Runner: forced, URLForRepo: fixtureEndpoint}).
		AcquireGit(t.Context(), source, commit, "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = packagedigest.Remove(got) }()
	if !forced.injected || !forced.checked {
		t.Fatal("disposable CRLF control or real immutable Git checkout was not observed")
	}
	if baseline.Source.SourceURL != source.URL || got.Source.SourceURL != source.URL ||
		baseline.Source.ResolvedRevision != commit || got.Source.ResolvedRevision != commit {
		t.Fatalf("disposable Git fixture changed the requested source identity: baseline=%+v forced=%+v", baseline.Source, got.Source)
	}
	if got.TreeDigest == "" || got.TreeDigest != baseline.TreeDigest {
		t.Fatalf("OS checkout changed source tree identity: baseline=%q forced=%q",
			baseline.TreeDigest, got.TreeDigest)
	}
	for _, tc := range []struct {
		name, expected string
	}{
		{name: "payload", expected: "first\nsecond\n"},
		{name: "explicit.txt", expected: "explicit\r\nwindows\r\n"},
	} {
		plain, e := os.ReadFile(filepath.Join(baseline.Root, tc.name))
		if e != nil || string(plain) != tc.expected {
			t.Fatalf("baseline %s bytes differ from Git attribute contract: %q %v",
				tc.name, plain, e)
		}
		acquired, e := os.ReadFile(filepath.Join(got.Root, tc.name))
		if e != nil || string(acquired) != tc.expected {
			t.Fatalf("forced-CRLF %s bytes differ from immutable source: %q %v",
				tc.name, acquired, e)
		}
	}
}

func TestNativeGitBrowserSlashRefAndAmbiguity(t *testing.T) {
	root, work, commit := nativeFixtureRepository(t)
	nativeFixtureGit(t, work, "branch", "feature/a")
	nativeFixtureGit(t, work, "push", "--quiet", filepath.Join(root, "repo.git"), "feature/a")
	server := nativeHTTPSFixture(t, root, false)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	configureNativeFixtureCredentials(t, home, server.URL, false)
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
