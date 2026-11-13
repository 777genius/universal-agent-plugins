package installer

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/hermes"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

const hermesRootDeathFixture = "UAP_TEST_HERMES_ROOT_DEATH"

type hermesRootDeathStore struct{ transaction.StateStore }

func (s hermesRootDeathStore) Save(state domain.StateFileV2) error {
	if err := s.StateStore.Save(state); err != nil {
		return err
	}
	for _, receipt := range state.TransactionReceipts {
		if strings.HasPrefix(receipt.AfterDigest, "native-root-v1:") && receipt.Phase == transaction.ReceiptPhaseCommitted {
			os.Exit(71) // No defers: root committed, native activation not acknowledged.
		}
	}
	return nil
}

func hermesRootDeathEngine(t *testing.T, base string) (*Engine, Request) {
	t.Helper()
	registry, err := clients.NewRegistry(hermes.NewProject())
	if err != nil {
		t.Fatal(err)
	}
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(base, "A")
	engine, err := New(Config{ProjectRoot: project, StateRoot: filepath.Join(project, ".uap"), Registry: registry,
		Runner: forbiddenV2Runner{t}, EnableNativeObserver: true, TrustedLocalPackages: true, HelperExecutable: helper})
	if err != nil {
		t.Fatal(err)
	}
	request := Request{Scope: domain.ScopeProject, ProjectRoot: project, Operation: OpInstall,
		PackageRoot: filepath.Join(base, "package"), ClientID: "hermes", ClientConfigRoot: filepath.Join(base, "user"),
		ClientExecutable: filepath.Join(base, "inert-hermes"), ClientVersion: "0.23.0",
		InstallationID: "00000000-0000-4000-8000-000000000024", OperationID: "hermes-root-death",
		RequiredComponents: []string{"mcp"}}
	return engine, request
}

func TestProjectHermesRootAcknowledgementDeathChild(t *testing.T) {
	base := os.Getenv(hermesRootDeathFixture)
	if base == "" {
		return
	}
	engine, request := hermesRootDeathEngine(t, base)
	handle, err := engine.Prepare(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	engine.store = hermesRootDeathStore{engine.store}
	_, err = engine.Apply(t.Context(), handle, Decision{Confirmed: true})
	t.Fatalf("did not die at the committed root boundary: %v", err)
}

// This records the existing generic Project native-attempt recovery boundary.
// The standalone root is fully owned and independently verifiable, but it must
// never become evidence that MCP activation or its binding acknowledgement ran.
func TestProjectHermesRootDeathPreservesUnacknowledgedNativeAttempt(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { deathAllowSnapshotCleanup(t, base) })
	deathWrite(t, filepath.Join(base, "A", ".git", "keep"), "project fixture\n")
	deathWrite(t, filepath.Join(base, "user", "config.yaml"), "model: unchanged\n")
	deathWrite(t, filepath.Join(base, "package", "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"hermes-death","version":"1.0.0"}`)
	deathWrite(t, filepath.Join(base, "package", "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"streamable-http","url":"http://127.0.0.1:9/inert"}}}`)
	command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestProjectHermesRootAcknowledgementDeathChild$")
	command.Env = append(os.Environ(), hermesRootDeathFixture+"="+base)
	output, err := command.CombinedOutput()
	var exited *exec.ExitError
	if !errors.As(err, &exited) || exited.ExitCode() != 71 {
		t.Fatalf("child did not reach committed root boundary: %v\n%s", err, output)
	}
	fresh, request := hermesRootDeathEngine(t, base)
	state, err := fresh.store.Load()
	if err != nil || len(state.Installations) != 1 || len(state.TransactionReceipts) != 1 {
		t.Fatalf("root boundary lost independent state: %+v, %v", state, err)
	}
	receipt := state.TransactionReceipts[0]
	if receipt.Phase != transaction.ReceiptPhaseCommitted || !strings.HasPrefix(receipt.AfterDigest, "native-root-v1:") {
		t.Fatalf("root receipt is not committed: %+v", receipt)
	}
	var binding domain.ClientBinding
	for _, candidate := range state.Installations[0].Clients {
		binding = candidate
	}
	if binding.NativeActivationAttempt == "" || binding.PendingNativeIntent != nil || len(binding.NativeObjects) != 1 || binding.NativeObjects[0].Kind != "managed_package_directory" || len(binding.Receipts) != 1 {
		t.Fatalf("root publication acknowledged a native effect or changed package ownership: %+v", binding)
	}
	root := filepath.Join(base, "A", ".hermes", "uap-session")
	if binding.TargetLocator == root || deathRead(t, filepath.Join(root, ".uap-owner")) != "uap-hermes-project-session-v1\n"+request.ProjectRoot+"\n" {
		t.Fatal("root publication changed package target or marker")
	}
	if _, err := os.Lstat(filepath.Join(root, "config.yaml")); !os.IsNotExist(err) {
		t.Fatalf("MCP effect occurred before the stopped boundary: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(binding.TargetLocator, "mcp.json")); err != nil {
		t.Fatalf("committed managed package was lost: %v", err)
	}
	before := deathTree(t, request.ProjectRoot)
	view, err := fresh.Inspect(t.Context())
	if err == nil || !view.Recovery.Required || view.Recovery.Reason != "native attempt has no persisted intent; retain uncertainty" {
		t.Fatalf("fresh inspect guessed native acknowledgement: %+v, %v", view, err)
	}
	if result, err := fresh.Recover(t.Context(), view); !errors.Is(err, ErrRecoveryRequired) || result.Outcome != OutcomeRecovery {
		t.Fatalf("recovery guessed an unrecorded native effect: %+v, %v", result, err)
	}
	request.Operation, request.OperationID = OpRepair, "refuse-after-root-death"
	handle, err := fresh.Prepare(t.Context(), request)
	if handle != nil {
		if closeErr := handle.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
	}
	if err == nil {
		t.Fatal("repair adopted an unacknowledged native attempt")
	}
	deathAssertTree(t, request.ProjectRoot, before)
	if deathRead(t, filepath.Join(base, "user", "config.yaml")) != "model: unchanged\n" {
		t.Fatal("root death changed original User state")
	}
}
