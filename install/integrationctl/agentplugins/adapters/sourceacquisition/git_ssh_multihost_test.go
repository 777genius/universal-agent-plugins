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

type nativeSSHMultiHostCase struct {
	alias, root, commit, payload string
	server                       nativeSSHFixture
}

// Check real two-host SSH authentication and immutable bytes. A private key
// authorized on A cannot be used to acquire from B.
func TestNativeSSHGitTwoHostCredentialIsolation(t *testing.T) {
	if runtime.GOOS == "windows" {
		if os.Getenv("UAP_REQUIRE_NATIVE_SSH") == "1" {
			t.Fatal("required SSH fixture needs a Unix OpenSSH server")
		}
		t.Skip("disposable multi-host sshd requires Unix")
	}
	sshd, err := exec.LookPath("sshd")
	if err != nil {
		if os.Getenv("UAP_REQUIRE_NATIVE_SSH") == "1" {
			t.Fatal("required SSH fixture needs openssh-server")
		}
		t.Skip("disposable multi-host sshd requires openssh-server")
	}
	current, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	hosts := nativeSSHMultiHostCases(t, sshd, current.Username)
	configPath, configBody := nativeSSHMultiHostConfig(t, hosts, current.Username)
	t.Setenv("GIT_SSH_COMMAND", "ssh -F "+configPath)
	for _, host := range hosts {
		t.Run(host.alias, func(t *testing.T) {
			verifyNativeSSHMultiHostCheckout(t, host, current.Username)
		})
	}
	refuseNativeSSHForeignIdentity(t, hosts, configPath, configBody, current.Username)
}

func nativeSSHMultiHostCases(t *testing.T, sshd, username string) []nativeSSHMultiHostCase {
	t.Helper()
	rootA, _, commitA := nativeFixtureRepository(t)
	rootB, workB, _ := nativeFixtureRepository(t)
	if err := os.WriteFile(filepath.Join(workB, "payload"), []byte("host-b distinct payload\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	nativeFixtureGit(t, workB, "add", "payload")
	nativeFixtureGit(t, workB, "-c", "user.name=Source Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--quiet", "-m", "unique host b")
	commitB := nativeFixtureGit(t, workB, "rev-parse", "HEAD")
	branchB := nativeFixtureGit(t, workB, "symbolic-ref", "--short", "HEAD")
	nativeFixtureGit(t, workB, "push", "--quiet", filepath.Join(rootB, "repo.git"), "HEAD:refs/heads/"+branchB)
	if commitA == commitB {
		t.Fatal("two SSH hosts resolved identical fixture commits")
	}
	return []nativeSSHMultiHostCase{
		{"uap-host-a.example.test", rootA, commitA, "inert bytes\n", newNativeSSHFixture(t, sshd, rootA, username)},
		{"uap-host-b.example.test", rootB, commitB, "host-b distinct payload\n", newNativeSSHFixture(t, sshd, rootB, username)},
	}
}

func nativeSSHMultiHostConfig(t *testing.T, hosts []nativeSSHMultiHostCase, username string) (string, string) {
	t.Helper()
	var knownHosts, config strings.Builder
	for _, host := range hosts {
		keyBytes, err := os.ReadFile(filepath.Join(filepath.Dir(host.server.clientConfig), "host.pub"))
		if err != nil {
			t.Fatal(err)
		}
		key := strings.Fields(string(keyBytes))
		if len(key) < 2 {
			t.Fatalf("invalid SSH fixture host key for %s", host.alias)
		}
		fmt.Fprintf(&knownHosts, "%s %s %s\n", host.alias, key[0], key[1])
	}
	dir := t.TempDir()
	knownHostsPath := filepath.Join(dir, "known_hosts")
	if err := os.WriteFile(knownHostsPath, []byte(knownHosts.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, host := range hosts {
		fmt.Fprintf(&config, "Host %s\n HostName 127.0.0.1\n Port %d\n User %s\n IdentityFile %s\n IdentitiesOnly yes\n IdentityAgent none\n UserKnownHostsFile %s\n HostKeyAlias %s\n StrictHostKeyChecking yes\n BatchMode yes\n PasswordAuthentication no\n KbdInteractiveAuthentication no\n", host.alias, host.server.port, username, filepath.Join(filepath.Dir(host.server.clientConfig), "identity"), knownHostsPath, host.alias)
	}
	path := filepath.Join(dir, "ssh_config")
	if err := os.WriteFile(path, []byte(config.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return path, config.String()
}

func verifyNativeSSHMultiHostCheckout(t *testing.T, host nativeSSHMultiHostCase, username string) {
	t.Helper()
	source := sourcespec.Git{URL: fmt.Sprintf("ssh://%s@%s:%d%s", username, host.alias, host.server.port, filepath.Join(host.root, "repo.git"))}
	acquirer := Acquirer{TempRoot: t.TempDir()}
	resolved, err := acquirer.ResolveGitSource(t.Context(), source)
	if err != nil || resolved.Resolution.Commit != host.commit {
		t.Fatalf("wrong host or commit resolution for %s: %+v %v", host.alias, resolved, err)
	}
	snapshot, err := acquirer.AcquireGit(t.Context(), source, host.commit, "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = packagedigest.Remove(snapshot) }()
	if snapshot.Source.SourceURL != source.URL || snapshot.Source.ResolvedRevision != host.commit || snapshot.TreeDigest == "" {
		t.Fatalf("wrong immutable source identity for %s: %+v", host.alias, snapshot.Source)
	}
	body, err := os.ReadFile(filepath.Join(snapshot.Root, "payload"))
	if err != nil || string(body) != host.payload {
		t.Fatalf("wrong host content for %s: %q %v", host.alias, body, err)
	}
}

func refuseNativeSSHForeignIdentity(t *testing.T, hosts []nativeSSHMultiHostCase, configPath, configBody, username string) {
	t.Helper()
	identityA := filepath.Join(filepath.Dir(hosts[0].server.clientConfig), "identity")
	identityB := filepath.Join(filepath.Dir(hosts[1].server.clientConfig), "identity")
	badConfig := strings.Replace(configBody, "IdentityFile "+identityB+"\n", "IdentityFile "+identityA+"\n", 1)
	if badConfig == configBody {
		t.Fatal("foreign-identity SSH fixture did not change host B")
	}
	path := filepath.Join(filepath.Dir(configPath), "wrong_host_key_config")
	if err := os.WriteFile(path, []byte(badConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_SSH_COMMAND", "ssh -F "+path)
	hostB := hosts[1]
	sourceB := sourcespec.Git{URL: fmt.Sprintf("ssh://%s@%s:%d%s", username, hostB.alias, hostB.server.port, filepath.Join(hostB.root, "repo.git"))}
	if _, err := (Acquirer{TempRoot: t.TempDir()}).ResolveGitSource(t.Context(), sourceB); err == nil {
		t.Fatal("host B incorrectly accepted host A's private SSH identity")
	}
}
