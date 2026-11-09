package agentpluginscli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func TestLocalDependencyPolicyTransitiveAndPins(t *testing.T) {
	t.Parallel()
	pin := environment.LockedPackage{
		Alias: "direct",
		Source: environment.LockedSource{Git: &environment.LockedGitSource{
			Repository: "acme/cli", Commit: strings.Repeat("a", 40),
			Requested: environment.Selector{Kind: environment.SelectorCommit, Value: strings.Repeat("a", 40)},
		}},
	}
	child := environment.LockedPackage{
		Alias: "transitive",
		Source: environment.LockedSource{Archive: &environment.LockedArchiveSource{
			URL: "https://untrusted.example.test/agent.zip", SHA256: strings.Repeat("b", 64),
		}},
		RequiredBy: []environment.RequiredBy{{Alias: "direct"}},
	}
	lock := environment.Lock{Packages: []environment.LockedPackage{pin, child}}
	var policy localDependencyPolicy
	policy.SchemaVersion = 1
	policy.Sources.AllowHosts = policyAllowList{Present: true, Values: []string{"github.com"}}
	if err := policy.evaluate(lock); err == nil || !strings.Contains(err.Error(), "transitive") {
		t.Fatalf("must block transitive disallowed source: %v", err)
	}
	policy.Sources.AllowHosts = policyAllowList{Present: true, Values: []string{"github.com", "untrusted.example.test"}}
	policy.Integrity.RequireArchiveSHA256 = true
	if err := policy.evaluate(lock); err == nil || !strings.Contains(err.Error(), "transitive") {
		t.Fatalf("must require declared hash, not merely downloaded hash: %v", err)
	}
	lock.Packages[1].Source.Archive.RequestedSHA256 = strings.Repeat("b", 64)
	if err := policy.evaluate(lock); err != nil {
		t.Fatalf("valid approved closure rejected: %v", err)
	}
	policy.Integrity.RequirePinnedGit = true
	lock.Packages[0].Source.Git.Requested.Kind = environment.SelectorRef
	if err := policy.evaluate(lock); err == nil || !strings.Contains(err.Error(), "direct") {
		t.Fatalf("unpinned Git request must fail: %v", err)
	}
	lock.Packages[0].Source.Git.Requested.Kind = environment.SelectorCommit
	policy.Sources.DenyHosts = []string{"github.com"}
	if err := policy.evaluate(lock); err == nil || !strings.Contains(err.Error(), "denied") {
		t.Fatalf("explicit denial must override allow-list: %v", err)
	}
}

func TestLocalDependencyPolicyExplicitOptInAndStrictParsing(t *testing.T) {
	t.Parallel()
	if err := checkLocalDependencyPolicy(context.Background(), App{}, "", environment.Lock{}); err != nil {
		t.Fatalf("no opt-in must preserve existing CLI behavior: %v", err)
	}
	root := t.TempDir()
	path := filepath.Join(root, "policy.yaml")
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("schema_version: 1\nsources:\n  allow_hosts: [github.com]\nintegrity:\n  require_pinned_git: true\nmcp:\n  allowed_transports: [stdio, streamable-http]\n")
	policy, err := readLocalDependencyPolicy(path)
	if err != nil || len(policy.Sources.AllowHosts.Values) != 1 || !policy.Integrity.RequirePinnedGit {
		t.Fatalf("valid local policy not parsed: %+v, %v", policy, err)
	}
	for name, body := range map[string]string{
		"unknown": "schema_version: 1\nunsafe: true\n",
		"two-docs": "schema_version: 1\n---\nschema_version: 1\n",
		"invalid-host": "schema_version: 1\nsources:\n  allow_hosts: [GitHub.com]\n",
		"wrong-version": "schema_version: 99\n",
		"duplicate": "schema_version: 1\nschema_version: 1\n",
		"unknown-mcp-transport": "schema_version: 1\nmcp:\n  allowed_transports: [telnet]\n",
	} {
		t.Run(name, func(t *testing.T) {
			write(body)
			if _, err := readLocalDependencyPolicy(path); err == nil {
				t.Fatal("invalid or unknown policy must fail closed")
			}
		})
	}
}

func TestLocalDependencyPolicyRejectsSymlinkAndUnsupportedOrigins(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	real := filepath.Join(root, "policy.yaml")
	if err := os.WriteFile(real, []byte("schema_version: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "policy-link.yaml")
	if err := os.Symlink(real, link); err == nil {
		if _, err := readLocalDependencyPolicy(link); err == nil {
			t.Fatal("policy symlink must not be followed")
		}
	}
	var policy localDependencyPolicy
	policy.SchemaVersion = 1
	if err := policy.evaluate(environment.Lock{Packages: []environment.LockedPackage{{Alias: "future-source"}}}); err == nil {
		t.Fatal("unknown future source must not bypass opt-in source governance")
	}
}

func TestLocalDependencyPolicyHostParsing(t *testing.T) {
	t.Parallel()
	for _, host := range []string{"github.com", "localhost", "git.example.test"} {
		if !validPolicyHost(host) {
			t.Fatalf("legitimate host %q rejected", host)
		}
	}
	for _, host := range []string{"", "GitHub.com", "a..b", "-example.com", "foo:443", "example.com/", "a_b.com"} {
		if validPolicyHost(host) {
			t.Fatalf("invalid host %q accepted", host)
		}
	}
	if host, err := hostFromPolicyURL("ssh://git@git.example.test/team/repo"); err != nil || host != "git.example.test" {
		t.Fatalf("git+ssh normalization failed: %q %v", host, err)
	}
}
