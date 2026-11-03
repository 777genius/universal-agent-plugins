package agentpluginscli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

type dependencyTreeCancelWriter struct {
	bytes.Buffer
	cancel context.CancelFunc
	writes int
}

func (writer *dependencyTreeCancelWriter) Write(body []byte) (int, error) {
	writer.writes++
	if writer.writes == 4 {
		writer.cancel()
	}
	return writer.Buffer.Write(body)
}

func TestDependencyTreeCancellationDuringRendering(t *testing.T) {
	tree := dependencyTree{Roots: []string{"p000"}}
	for i := 0; i < 256; i++ {
		pkg := environment.LockedPackage{Alias: fmt.Sprintf("p%03d", i), Version: "1.0.0"}
		if i > 0 {
			pkg.RequiredBy = []environment.RequiredBy{{Alias: fmt.Sprintf("p%03d", i-1)}}
		}
		tree.Packages = append(tree.Packages, pkg)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	writer := &dependencyTreeCancelWriter{cancel: cancel}
	if err := renderDependencyTree(ctx, writer, tree); !errors.Is(err, context.Canceled) {
		t.Fatalf("tree ignored cancellation during output: %v", err)
	}
	if writer.writes != 4 || bytes.Contains(writer.Bytes(), []byte("p255")) {
		t.Fatalf("tree continued rendering after cancellation: writes=%d", writer.writes)
	}
	var untouched bytes.Buffer
	if err := renderDependencyTree(ctx, &untouched, tree); !errors.Is(err, context.Canceled) || untouched.Len() != 0 {
		t.Fatalf("pre-canceled tree wrote output: error=%v bytes=%d", err, untouched.Len())
	}
}
