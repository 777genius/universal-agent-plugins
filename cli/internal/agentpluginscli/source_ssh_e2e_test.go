package agentpluginscli

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDirectSourceSavedLifecycleRealSSH(t *testing.T) {
	sshd, err := exec.LookPath("sshd")
	if err != nil {
		if os.Getenv("UAP_REQUIRE_NATIVE_SSH") == "1" {
			t.Fatal("required real SSH CLI fixture needs openssh-server")
		}
		t.Skip("real SSH CLI fixture requires openssh-server")
	}
	current, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	for _, spelling := range []string{"ssh-url", "scp"} {
		t.Run(spelling, func(t *testing.T) {
			transport := newDirectTransportFixture(t, "git", false)
			port, config := directSSHServer(t, sshd, transport.remote, current.Username)
			t.Setenv("GIT_SSH_COMMAND", "ssh -F "+config)
			endpoint := fmt.Sprintf("ssh://%s@127.0.0.1:%d%s", current.Username, port, transport.remote)
			if spelling == "scp" {
				endpoint = current.Username + "@uap-cli-fixture:" + transport.remote
			}
			fixture := directCLIEnvironment(t, transport)
			directSavedLifecycleEndpoint(t, fixture, transport, "git", endpoint)
		})
	}
}

func directSSHServer(t *testing.T, sshd, repository, username string) (int, string) {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{"host", "identity"} {
		command := exec.CommandContext(t.Context(), "ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", filepath.Join(root, name))
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("generate disposable SSH key: %v %s", err, output)
		}
	}
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	public := dependencyRead(t, filepath.Join(root, "identity.pub"))
	dependencyWrite(t, filepath.Join(root, "authorized_keys"), fmt.Sprintf("command=\"%s upload-pack '%s'\",no-port-forwarding,no-agent-forwarding,no-X11-forwarding,no-pty %s", git, repository, public))
	serverConfig := filepath.Join(root, "sshd_config")
	dependencyWrite(t, serverConfig, fmt.Sprintf("Port %d\nListenAddress 127.0.0.1\nHostKey %s\nPidFile %s\nAuthorizedKeysFile %s\nStrictModes no\nPasswordAuthentication no\nKbdInteractiveAuthentication no\nUsePAM no\nPermitRootLogin forced-commands-only\nAllowUsers %s\nLogLevel ERROR\n", port, filepath.Join(root, "host"), filepath.Join(root, "pid"), filepath.Join(root, "authorized_keys"), username))
	logs, err := os.Create(filepath.Join(root, "sshd.log"))
	if err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(t.Context(), sshd, "-D", "-e", "-f", serverConfig)
	command.Stdout, command.Stderr = logs, logs
	if err := command.Start(); err != nil {
		_ = logs.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = command.Process.Kill(); _ = command.Wait(); _ = logs.Close() })
	directWaitSSH(t, port, logs.Name())
	host := strings.Fields(dependencyRead(t, filepath.Join(root, "host.pub")))
	dependencyWrite(t, filepath.Join(root, "known_hosts"), fmt.Sprintf("[127.0.0.1]:%d %s %s\n", port, host[0], host[1]))
	clientConfig := filepath.Join(root, "ssh_config")
	dependencyWrite(t, clientConfig, fmt.Sprintf("Host *\n HostName 127.0.0.1\n Port %d\n User %s\n IdentityFile %s\n IdentitiesOnly yes\n UserKnownHostsFile %s\n StrictHostKeyChecking yes\n BatchMode yes\n", port, username, filepath.Join(root, "identity"), filepath.Join(root, "known_hosts")))
	return port, clientConfig
}

func directWaitSSH(t *testing.T, port int, log string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		connection, err := (&net.Dialer{Timeout: 100 * time.Millisecond}).DialContext(t.Context(), "tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			_ = connection.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("disposable SSH server failed: %s", dependencyRead(t, log))
}
