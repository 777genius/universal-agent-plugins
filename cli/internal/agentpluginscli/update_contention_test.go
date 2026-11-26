package agentpluginscli

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

type commandBoundaryLock struct{ ports.MutationLock }

func (lock commandBoundaryLock) Acquire(ctx context.Context) (ports.UnlockFunc, error) {
	release, err := lock.MutationLock.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := fmt.Fprintln(os.Stdout, "UAP_COMMAND_LOCKED"); err != nil {
		return nil, errors.Join(err, release())
	}
	var signal [1]byte
	if _, err := io.ReadFull(os.Stdin, signal[:]); err != nil {
		return nil, errors.Join(err, release())
	}
	return release, nil
}

// The child runs a real add command while holding the production OS mutex.
// The competing public update must refuse, then succeed after the child exits.
func TestUpdateRefusesConcurrentAddAndRetriesAfterExit(t *testing.T) {
	root := os.Getenv("UAP_UPDATE_CONTENTION_ROOT")
	if root != "" {
		fixture := contentionFixture(t, root)
		fixture.app.Lifecycle.Lock = commandBoundaryLock{fixture.app.Lifecycle.Lock}
		if stdout, stderr, err := fixture.execute(false, "add", os.Getenv("UAP_UPDATE_CONTENTION_PLUGIN"), "--target", "cursor", "--format", "json"); err != nil {
			t.Fatalf("holder add: %v\n%s\n%s", err, stdout, stderr)
		}
		return
	}
	root = t.TempDir()
	fixture := contentionFixture(t, root)
	alpha := writeNamedCLIPlugin(t, "alpha", "1.0.0")
	if _, _, err := fixture.execute(false, "add", alpha, "--target", "cursor"); err != nil {
		t.Fatal(err)
	}
	before := dependencyRead(t, fixture.store.Path)
	seed, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(onlyCLIClient(seed.Installations[0]).TargetLocator, "plugin.json")
	targetBefore := dependencyRead(t, target)
	writeNamedCLIPluginVersion(t, alpha, "alpha", "2.0.0")
	beta := writeNamedCLIPlugin(t, "beta", "1.0.0")
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestUpdateRefusesConcurrentAddAndRetriesAfterExit$")
	child.Env = append(os.Environ(), "UAP_UPDATE_CONTENTION_ROOT="+root, "UAP_UPDATE_CONTENTION_PLUGIN="+beta)
	stdin, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	child.Stderr = &stderr
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = child.Process.Kill(); _ = stdin.Close() })
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "UAP_COMMAND_LOCKED" {
		t.Fatalf("holder boundary: %q %v", line, err)
	}
	_, _, err = fixture.execute(false, "update", "alpha", "--format", "json")
	if !errors.Is(err, processlock.ErrActive) || !strings.Contains(err.Error(), "wait for it to finish, then retry the same command") {
		t.Fatalf("competing command did not preserve wait/retry guidance: %v", err)
	}
	if after := dependencyRead(t, fixture.store.Path); after != before {
		t.Fatal("refused update changed durable state")
	}
	if after := dependencyRead(t, target); after != targetBefore {
		t.Fatal("refused update changed the managed package")
	}
	if _, err := stdin.Write([]byte{1}); err != nil {
		t.Fatal(err)
	}
	if err := child.Wait(); err != nil {
		t.Fatalf("holder add: %v\n%s", err, stderr.String())
	}
	if stdout, _, err := fixture.execute(false, "update", "alpha", "--format", "json"); err != nil || !strings.Contains(stdout, `"mutated":true`) {
		t.Fatalf("retry after holder exit: %v\n%s", err, stdout)
	}
}

func contentionFixture(t *testing.T, root string) cliFixture {
	t.Helper()
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	client := fixtureClient(t, domain.ClientCursor)
	client.ConfigRoot = filepath.Join(canonical, "client", "cursor")
	return newCLIFixtureAt(t, []domain.DetectedClient{client}, root)
}
