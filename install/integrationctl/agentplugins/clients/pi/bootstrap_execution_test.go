package pi

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/packagesnapshot"
)

// Execute the actual generated module, including its physical-path guard. The
// source and real filesystem paths contain single quotes and JS-shaped text;
// preserving JSON bytes alone would not establish safe JavaScript evaluation.
func TestBootstrapExecutesQuotedPathsWithoutInjection(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node is required for generated JavaScript execution")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(root, "profile'with`quotes")
	data := filepath.Join(root, "data');globalThis.injected=true;('")
	sourceName := "native');globalThis.injected=true;('"
	staging := filepath.Join(profile, ".agentplugins-staging-TEST")
	source := "globalThis.sourceEvaluations = (globalThis.sourceEvaluations || 0) + 1;\nexport default function(pi, descriptor) { pi.record(descriptor, Object.isFrozen(descriptor)); }\n"
	writePi(t, filepath.Join(staging, sourceName, "index.js"), source)
	writePi(t, filepath.Join(staging, sourceName, "package.json"), `{"type":"module"}`)
	raw, err := json.Marshal(extension{Version: 1, Source: sourceName})
	if err != nil {
		t.Fatal(err)
	}
	envelope := domain.PackageEnvelope{SnapshotRoot: staging, Manifest: domain.PluginManifest{Name: "TEST-pi", Extensions: map[string]json.RawMessage{Namespace: raw}}}
	manifest, err := json.Marshal(envelope.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	writePi(t, filepath.Join(staging, "plugin.json"), string(manifest))
	envelope.TreeDigest, err = (packagesnapshot.Builder{}).Digest(context.Background(), staging)
	if err != nil {
		t.Fatal(err)
	}
	plan := domain.DeliveryPlan{ClientID: domain.ClientPi, Scope: domain.ScopeUser, NativeRegistryRoot: profile, TargetAnchor: profile, TargetRoot: filepath.Join(profile, "extensions"), PhysicalArtifactID: "TEST-pi"}
	plan.ActivePath = filepath.Join(plan.TargetRoot, plan.PhysicalArtifactID)
	if _, err := New().Project(context.Background(), clients.ProjectionInput{StagingPath: staging, Plan: plan, Envelope: envelope, PluginDataPath: data}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(plan.TargetRoot, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(staging, plan.ActivePath); err != nil {
		t.Fatal(err)
	}
	wanted, err := descriptorFor(plan, envelope.TreeDigest, data)
	if err != nil {
		t.Fatal(err)
	}
	loader := filepath.Join(root, "loader.mjs")
	writePi(t, loader, `import { readFile } from "node:fs/promises";
export async function load(url, context, next) {
  if (url.endsWith(".ts")) return { format: "module", source: await readFile(new URL(url), "utf8"), shortCircuit: true };
  return next(url, context);
}
`)
	runner := filepath.Join(root, "runner.mjs")
	writePi(t, runner, `import { pathToFileURL } from "node:url";
const observed = [];
let frozen = false;
const entry = await import(pathToFileURL(process.argv[2]).href);
await entry.default({ record(value, isFrozen) { observed.push(value); frozen = isFrozen; } });
process.stdout.write(JSON.stringify({ observed, frozen, imports: globalThis.sourceEvaluations || 0, injected: globalThis.injected === true }));
`)
	run := func(t *testing.T, path string, admitted bool) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, node, "--no-warnings", "--experimental-loader", loader, runner, filepath.Join(path, "index.ts"))
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("generated module execution: %v: %s", err, stderr.String())
		}
		var got struct {
			Observed []Descriptor `json:"observed"`
			Frozen   bool         `json:"frozen"`
			Imports  int          `json:"imports"`
			Injected bool         `json:"injected"`
		}
		if err := json.Unmarshal(output, &got); err != nil {
			t.Fatal(err)
		}
		if got.Injected {
			t.Fatal("path text executed as JavaScript")
		}
		if admitted {
			if got.Imports != 1 || !got.Frozen || !reflect.DeepEqual(got.Observed, []Descriptor{wanted}) {
				t.Fatalf("bound descriptor/source changed: %+v", got)
			}
		} else if got.Imports != 0 || len(got.Observed) != 0 {
			t.Fatalf("unbound source executed: %+v", got)
		}
	}
	run(t, plan.ActivePath, true)
	retained := filepath.Join(plan.TargetRoot, "TEST-visible-retained-copy")
	body, err := os.ReadFile(filepath.Join(plan.ActivePath, "index.ts"))
	if err != nil {
		t.Fatal(err)
	}
	writePi(t, filepath.Join(retained, "index.ts"), string(body))
	writePi(t, filepath.Join(retained, sourceName, "index.js"), source)
	writePi(t, filepath.Join(retained, sourceName, "package.json"), `{"type":"module"}`)
	run(t, retained, false)
	t.Run("misbound-symlink", func(t *testing.T) {
		moved := filepath.Join(root, "TEST-moved-package")
		if err := os.Rename(plan.ActivePath, moved); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(moved, plan.ActivePath); err != nil {
			t.Skipf("native symlink unavailable: %v", err)
		}
		run(t, plan.ActivePath, false)
	})
	if !strings.Contains(string(body), string(mustJSON(t, data))) {
		t.Fatal("descriptor path was not retained as a JSON expression")
	}
}

func mustJSON(t *testing.T, value string) []byte {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return body
}
