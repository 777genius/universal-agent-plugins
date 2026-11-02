package sourceacquisition

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

// Two separate SSH hosts must independently authenticate and return their own
// exact immutable commit and bytes. A key for A must never authorize host B.
// This is native-transport evidence, not a claim of a multi-host locked install.
func TestNativeSSHGitTwoHostCredentialIsolation(t *testing.T) {
	if runtime.GOOS == "windows" {
		if os.Getenv("UAP_REQUIRE_NATIVE_SSH") == "1" {
			t.Fatal("required two-host SSH fixture needs a Unix OpenSSH server")
		}
		t.Skip("disposable two-host SSH fixture needs Unix sshd")
	}
	sshd, err := exec.LookPath("sshd")
	if err != nil {
		if os.Getenv("UAP_REQUIRE_NATIVE_SSH") == "1" {
			t.Fatal("required two-host SSH fixture needs openssh-server")
		}
		t.Skip("disposable two-host SSH fixture needs openssh-server")
	}
	current, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}

	rootA, _, commitA := nativeFixtureRepository(t)
	rootB, workB, _ := nativeFixtureRepository(t)
	// A wrong-host checkout must be visible independently of Git metadata.
	if err := os.WriteFile(filepath.Join(workB, "payload"), []byte("host-b distinct payload\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	nativeFixtureGit(t, workB, "add", "payload")
	nativeFixtureGit(t, workB, "-c", "user.name=Source Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--quiet", "-m", "unique host b")
	commitB := nativeFixtureGit(t, workB, "rev-parse", "HEAD")
	branchB := nativeFixtureGit(t, workB, "symbolic-ref", "--short", "HEAD")
	nativeFixtureGit(t, workB, "push", "--quiet", filepath.Join(rootB, "repo.git"), "HEAD:refs/heads/"+branchB)
	if commitA == commitB {
		t.Fatal("distinct hosts resolved an identical fixture commit")
	}

	serverA := newNativeSSHFixture(t, sshd, rootA, current.Username)
	serverB := newNativeSSHFixture(t, sshd, rootB, current.Username)
	hosts := []struct {
		alias, root, commit, payload string
		server                       nativeSSHFixture
	}{
		{"uap-host-a.example.test", rootA, commitA, "inert bytes\n", serverA},
		{"uap-host-b.example.test", rootB, commitB, "host-b distinct payload\n", serverB},
	}
	var knownHosts, config strings.Builder
	for _, host := range hosts {
		keyBytes, err := os.ReadFile(filepath.Join(filepath.Dir(host.server.clientConfig), "host.pub"))
		if err != nil {
			t.Fatal(err)
		}
		key := strings.Fields(string(keyBytes))
		if len(key) < 2 {
			t.Fatalf("invalid fixture host key for %s", host.alias)
		}
		fmt.Fprintf(&knownHosts, "%s %s %s\n", host.alias, key[0], key[1])
	}
	configDir := t.TempDir()
	knownHostsPath := filepath.Join(configDir, "known_hosts")
	if err := os.WriteFile(knownHostsPath, []byte(knownHosts.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, host := range hosts {
		fmt.Fprintf(&config, "Host %s\n HostName 127.0.0.1\n Port %d\n User %s\n IdentityFile %s\n IdentitiesOnly yes\n IdentityAgent none\n UserKnownHostsFile %s\n HostKeyAlias %s\n StrictHostKeyChecking yes\n BatchMode yes\n PasswordAuthentication no\n KbdInteractiveAuthentication no\n", host.alias, host.server.port, current.Username, filepath.Join(filepath.Dir(host.server.clientConfig), "identity"), knownHostsPath, host.alias)
	}
	configPath := filepath.Join(configDir, "ssh_config")
	if err := os.WriteFile(configPath, []byte(config.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_SSH_COMMAND", "ssh -F "+configPath)

	for _, host := range hosts {
		t.Run(host.alias, func(t *testing.T) {
			source := sourcespec.Git{URL: fmt.Sprintf("ssh://%s@%s:%d%s", current.Username, host.alias, host.server.port, filepath.Join(host.root, "repo.git"))}
			acquirer := Acquirer{TempRoot: t.TempDir()}
			resolved, err := acquirer.ResolveGitSource(t.Context(), source)
			if err != nil || resolved.Resolution.Commit != host.commit {
				t.Fatalf("wrong host/commit resolution for %s: %+v %v", host.alias, resolved, err)
			}
			snapshot, err := acquirer.AcquireGit(t.Context(), source, host.commit, "")
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = packagedigest.Remove(snapshot) }()
			if snapshot.Source.SourceURL != source.URL || snapshot.Source.ResolvedRevision != host.commit || snapshot.TreeDigest == "" {
				t.Fatalf("wrong source identity for %s: %+v", host.alias, snapshot.Source)
			}
			body, err := os.ReadFile(filepath.Join(snapshot.Root, "payload"))
			if err != nil || string(body) != host.payload {
				t.Fatalf("wrong host bytes for %s: %q %v", host.alias, body, err)
			}
		})
	}

	// A key that legitimately belongs to host A must not authenticate to B,
	// even though both servers are on loopback and their payloads are valid.
	identityA := filepath.Join(filepath.Dir(serverA.clientConfig), "identity")
	identityB := filepath.Join(filepath.Dir(serverB.clientConfig), "identity")
	badConfig := strings.Replace(config.String(), "IdentityFile "+identityB+"\n", "IdentityFile "+identityA+"\n", 1)
	if badConfig == config.String() {
		t.Fatal("negative credential-isolation fixture did not change host B")
	}
	badConfigPath := filepath.Join(configDir, "wrong_host_key_config")
	if err := os.WriteFile(badConfigPath, []byte(badConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_SSH_COMMAND", "ssh -F "+badConfigPath)
	bSource := sourcespec.Git{URL: fmt.Sprintf("ssh://%s@%s:%d%s", current.Username, hosts[1].alias, serverB.port, filepath.Join(rootB, "repo.git"))}
	if _, err := (Acquirer{TempRoot: t.TempDir()}).ResolveGitSource(t.Context(), bSource); err == nil {
		t.Fatal("host B incorrectly accepted host A's private SSH identity")
	}
}
