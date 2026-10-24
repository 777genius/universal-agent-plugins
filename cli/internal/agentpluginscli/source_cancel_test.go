package agentpluginscli

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

type cancellationDuringRetention struct {
	context.Context
	cancel context.CancelFunc
	checks atomic.Int32
}

func (ctx *cancellationDuringRetention) Err() error {
	if ctx.checks.Add(1) == 2 {
		ctx.cancel()
	}
	return ctx.Context.Err()
}

func TestDependencyRetentionHonorsOperationCancellation(t *testing.T) {
	fixture := newCLIFixture(t, nil)
	loaded, err := fixture.app.acquireLocal(t.Context(), writeCLIPlugin(t))
	if err != nil {
		t.Fatal(err)
	}
	originalRoot := loaded.envelope.SnapshotRoot
	preview, cleanup, err := dependencyPreviewApp(fixture.app, &options{})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	pin := environment.LockedPackage{Alias: "demo", SourceKey: "github:acme/demo:", DigestAlgorithm: domain.TreeDigestAlgorithm,
		TreeDigest: loaded.envelope.TreeDigest, ManifestDigest: loaded.envelope.ManifestDigest,
		Source: environment.LockedSource{Git: &environment.LockedGitSource{Repository: "acme/demo", Commit: strings.Repeat("a", 40)}}}
	base, cancel := context.WithCancel(t.Context())
	defer cancel()
	operation := &cancellationDuringRetention{Context: base, cancel: cancel}
	err = (dependencyResolver{app: preview}).finishLoaded(operation, loaded, pin)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("retention detached operation cancellation: %v", err)
	}
	if len(preview.dependencySnapshots.snapshots) != 0 {
		t.Fatal("canceled retention published a command snapshot")
	}
	if _, err := os.Stat(originalRoot); !os.IsNotExist(err) {
		t.Fatal("canceled retention left original snapshot")
	}
	if _, err := os.Stat(preview.dependencyStore().Root); !os.IsNotExist(err) {
		t.Fatal("canceled retention published durable store")
	}
}
