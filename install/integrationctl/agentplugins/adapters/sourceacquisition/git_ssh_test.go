package sourceacquisition

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

func TestNativeSSHGitWithIsolatedKeysAndHostVerification(t *testing.T) {
	sshd, err := exec.LookPath("sshd")
	if err != nil {
		t.Skip("native SSH fixture requires openssh-server")
	}
	root, _, commit := nativeFixtureRepository(t)
	current, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	fixture := newNativeSSHFixture(t, sshd, root, current.Username)
	t.Setenv("GIT_SSH_COMMAND", "ssh -F "+fixture.clientConfig)
	source := sourcespec.Git{URL: fmt.Sprintf("ssh://%s@127.0.0.1:%d%s", current.Username, fixture.port, filepath.Join(root, "repo.git"))}
	acquirer := Acquirer{TempRoot: t.TempDir()}
	resolved, err := acquirer.ResolveGitSource(t.Context(), source)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Resolution.Commit != commit {
		t.Fatal("SSH resolved another commit")
	}
	snapshot, err := acquirer.AcquireGit(t.Context(), source, commit, "")
	if err != nil {
		t.Fatal(err)
	}
	_ = packagedigest.Remove(snapshot)
	scp, err := sourcespec.Parse(current.Username + "@uap-source-fixture:" + filepath.Join(root, "repo.git"))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err = acquirer.AcquireGit(t.Context(), *scp.Git, commit, "")
	if err != nil {
		t.Fatal(err)
	}
	_ = packagedigest.Remove(snapshot)
	if err := os.WriteFile(fixture.knownHosts, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := acquirer.AcquireGit(t.Context(), source, commit, ""); err == nil {
		t.Fatal("untrusted SSH host accepted")
	}
}

type nativeSSHFixture struct {
	port                     int
	clientConfig, knownHosts string
}

func newNativeSSHFixture(t *testing.T, sshd, repositoryRoot, username string) nativeSSHFixture {
	t.Helper()
	dir := t.TempDir()
	sshFixtureKey(t, filepath.Join(dir, "host"))
	sshFixtureKey(t, filepath.Join(dir, "identity"))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	clientKey, err := os.ReadFile(filepath.Join(dir, "identity.pub"))
	if err != nil {
		t.Fatal(err)
	}
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	authorized := fmt.Sprintf("command=\"%s upload-pack '%s'\",no-port-forwarding,no-agent-forwarding,no-X11-forwarding,no-pty %s", git, filepath.Join(repositoryRoot, "repo.git"), clientKey)
	sshFixtureWrite(t, filepath.Join(dir, "authorized_keys"), authorized)
	config := fmt.Sprintf("Port %d\nListenAddress 127.0.0.1\nHostKey %s\nPidFile %s\nAuthorizedKeysFile %s\nStrictModes no\nPasswordAuthentication no\nKbdInteractiveAuthentication no\nUsePAM no\nPermitRootLogin forced-commands-only\nAllowUsers %s\nLogLevel ERROR\n", port, filepath.Join(dir, "host"), filepath.Join(dir, "pid"), filepath.Join(dir, "authorized_keys"), username)
	serverConfig := filepath.Join(dir, "sshd_config")
	sshFixtureWrite(t, serverConfig, config)
	var logs bytes.Buffer
	command := exec.Command(sshd, "-D", "-e", "-f", serverConfig)
	command.Stdout = &logs
	command.Stderr = &logs
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = command.Process.Kill(); _ = command.Wait() })
	waitSSHFixture(t, port, &logs)
	hostKey, err := os.ReadFile(filepath.Join(dir, "host.pub"))
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(string(hostKey))
	knownHosts := filepath.Join(dir, "known_hosts")
	sshFixtureWrite(t, knownHosts, fmt.Sprintf("[127.0.0.1]:%d %s %s\n", port, fields[0], fields[1]))
	clientConfig := filepath.Join(dir, "ssh_config")
	sshFixtureWrite(t, clientConfig, fmt.Sprintf("Host *\n HostName 127.0.0.1\n Port %d\n User %s\n IdentityFile %s\n IdentitiesOnly yes\n UserKnownHostsFile %s\n StrictHostKeyChecking yes\n BatchMode yes\n", port, username, filepath.Join(dir, "identity"), knownHosts))
	return nativeSSHFixture{port: port, clientConfig: clientConfig, knownHosts: knownHosts}
}

func sshFixtureKey(t *testing.T, path string) {
	t.Helper()
	command := exec.CommandContext(t.Context(), "ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", path)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("fixture key generation: %v %s", err, output)
	}
}
func sshFixtureWrite(t *testing.T, path, value string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
}
func waitSSHFixture(t *testing.T, port int, logs *bytes.Buffer) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		connection, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 100*time.Millisecond)
		if err == nil {
			_ = connection.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("disposable sshd did not start: %s", logs.String())
}
