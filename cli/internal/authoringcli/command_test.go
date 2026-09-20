package authoringcli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/cli/internal/exitx"
	"github.com/spf13/cobra"
)

type request struct {
	Options Options
	Path    string
}
type result struct {
	Path string `json:"path"`
}

func fixtureFactory(run RunnerFunc[request, result], support Support) Factory {
	return func() (*cobra.Command, error) {
		return NewCommand(Spec[request, result]{
			Use: "validate [path]", Short: "Validate a standard package", Args: cobra.ExactArgs(1), Support: support,
			Decode: func(_ *cobra.Command, opts Options, args []string) (request, error) {
				return request{opts, args[0]}, nil
			},
			Runner: run,
			Render: func(s Streams, opts Options, r result, err error) error {
				if opts.Format == "json" {
					state := "success"
					if err != nil {
						state = "failure"
					}
					return json.NewEncoder(s.Out).Encode(struct {
						Schema  int    `json:"schema_version"`
						Command string `json:"command"`
						Result  string `json:"result"`
						Data    result `json:"data"`
					}{1, "author.validate", state, r})
				}
				_, e := fmt.Fprintln(s.Out, r.Path)
				return e
			},
		})
	}
}

func rootFor(t *testing.T, nested bool, factory Factory) *cobra.Command {
	t.Helper()
	if !nested {
		root, err := NewPluginKitRoot(factory)
		if err != nil {
			t.Fatal(err)
		}
		return root
	}
	root := agentpluginscli.NewRoot(agentpluginscli.App{})
	author, err := NewAuthorCommand(factory)
	if err != nil {
		t.Fatal(err)
	}
	root.AddCommand(author)
	return root
}

func execute(root *cobra.Command, args ...string) (string, string, error) {
	var out, diag bytes.Buffer
	ctx := root.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	err := Factory(func() (*cobra.Command, error) { return root, nil }).Execute(ctx, args, Streams{strings.NewReader("private input"), &out, &diag})
	return out.String(), diag.String(), err
}

func TestEntrypointParityAndIsolation(t *testing.T) {
	path := t.TempDir()
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			var got []request
			sentinel := exitx.Wrap(errors.New("validation failed"), 7)
			factory := fixtureFactory(func(ctx context.Context, req request) (result, error) {
				got = append(got, req)
				if fail {
					return result{req.Path}, sentinel
				}
				return result{req.Path}, nil
			}, Support{true, true, true, true})
			a := rootFor(t, false, factory)
			b := rootFor(t, true, factory)
			outA, errA, runA := execute(a, "--format", "json", "--no-color", "--dry-run", "--target", "cursor", "validate", path)
			outB, errB, runB := execute(b, "--target", "cursor", "author", "validate", path, "--format", "json", "--no-color", "--dry-run")
			if outA != outB || errA != errB || len(got) != 2 || got[0] != got[1] {
				t.Fatalf("parity: %q %q / %q %q / %+v", outA, outB, errA, errB, got)
			}
			if fail && (runA != sentinel || runB != sentinel || exitx.Code(runA) != 7) {
				t.Fatal("lost runner error/exit code")
			}
			if !fail && (runA != nil || runB != nil) {
				t.Fatal(runA, runB)
			}
			decoder := json.NewDecoder(strings.NewReader(outA))
			var doc any
			if decoder.Decode(&doc) != nil || decoder.Decode(&doc) != io.EOF {
				t.Fatal("not exactly one JSON document")
			}
			// Independent invocations must not carry format/target/booleans.
			_, _, _ = execute(rootFor(t, false, factory), "validate", path)
			if got[2].Options != (Options{Format: "human"}) {
				t.Fatalf("stale flags: %+v", got[2])
			}
		})
	}
}

func TestInstallerFlagsRejectedBeforeRunner(t *testing.T) {
	for _, flag := range []string{"--scope=user", "--accept-security-risk=false", "--security-details=false"} {
		for _, before := range []bool{true, false} {
			t.Run(fmt.Sprint(flag, before), func(t *testing.T) {
				calls := 0
				root := rootFor(t, true, fixtureFactory(func(_ context.Context, req request) (result, error) { calls++; return result{}, nil }, Support{Format: true}))
				args := []string{"author", "validate", t.TempDir()}
				if before {
					args = append([]string{flag}, args...)
				} else {
					args = append(args, flag)
				}
				_, _, err := execute(root, args...)
				if err == nil || !strings.Contains(err.Error(), "installer-only") || calls != 0 {
					t.Fatalf("%v calls=%d", err, calls)
				}
				_, _, err = execute(rootFor(t, true, fixtureFactory(func(_ context.Context, req request) (result, error) { calls++; return result{}, nil }, Support{Format: true})), "author", "validate", t.TempDir())
				if err != nil || calls != 1 {
					t.Fatalf("fresh invocation after rejection: %v calls=%d", err, calls)
				}
			})
		}
	}
}

func TestUnsupportedFlagsAndArguments(t *testing.T) {
	for _, args := range [][]string{
		{"--target=cursor", "author", "validate", "fixture"},
		{"author", "validate", "fixture", "--dry-run=false"},
		{"author", "validate", "fixture", "--format=yaml"},
		{"author", "validate", "--format=json"},
	} {
		calls := 0
		root := rootFor(t, true, fixtureFactory(func(_ context.Context, r request) (result, error) { calls++; return result{}, nil }, Support{Format: true}))
		_, _, err := execute(root, args...)
		if err == nil || calls != 0 {
			t.Fatalf("%v: %v calls=%d", args, err, calls)
		}
		_, _, err = execute(rootFor(t, true, fixtureFactory(func(_ context.Context, req request) (result, error) { calls++; return result{}, nil }, Support{Format: true})), "author", "validate", t.TempDir())
		if err != nil || calls != 1 {
			t.Fatalf("fresh invocation after args rejection: %v", err)
		}
	}
}

func TestContextStreamsAndRenderingFailure(t *testing.T) {
	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "passed")
	runErr := exitx.Wrap(errors.New("runner"), 9)
	renderErr := errors.New("writer")
	cmd, err := NewCommand(Spec[string, string]{Use: "inspect", Support: Support{},
		Decode: func(_ *cobra.Command, _ Options, _ []string) (string, error) { return "request", nil },
		Runner: RunnerFunc[string, string](func(got context.Context, r string) (string, error) {
			if got.Value(key{}) != "passed" || r != "request" {
				t.Fatal("context/request lost")
			}
			return "result", runErr
		}),
		Render: func(s Streams, _ Options, r string, e error) error {
			b, _ := io.ReadAll(s.In)
			if string(b) != "private input" || r != "result" || e != runErr {
				t.Fatal("streams/result lost")
			}
			fmt.Fprint(s.Err, "diagnostic")
			return renderErr
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	root, err := NewPluginKitRoot(func() (*cobra.Command, error) { return cmd, nil })
	if err != nil {
		t.Fatal(err)
	}
	root.SetContext(ctx)
	out, diag, err := execute(root, "inspect")
	if out != "" || diag != "diagnostic" || !errors.Is(err, runErr) || !errors.Is(err, renderErr) || exitx.Code(err) != 9 {
		t.Fatalf("%q %q %v", out, diag, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	root = rootFor(t, false, fixtureFactory(func(context.Context, request) (result, error) { calls++; return result{}, nil }, Support{}))
	root.SetContext(ctx)
	_, _, err = execute(root, "validate", t.TempDir())
	if !errors.Is(err, context.Canceled) || calls != 0 {
		t.Fatal("cancellation did not stop runner", err)
	}
}

func TestConstructionHelpAndInstallerParsing(t *testing.T) {
	factory := fixtureFactory(func(context.Context, request) (result, error) { return result{}, nil }, Support{Format: true})
	a := rootFor(t, false, factory)
	b := rootFor(t, true, factory)
	ca, _, _ := a.Find([]string{"validate"})
	cb, _, _ := b.Find([]string{"author", "validate"})
	if ca == cb || ca.Flags() == cb.Flags() || ca.Parent() == cb.Parent() {
		t.Fatal("shared mutable commands")
	}
	out, _, err := execute(b, "author", "validate", "--help")
	if err != nil || !strings.Contains(out, "agentplugins author validate") || !strings.Contains(out, "installer-only") && !strings.Contains(out, "Installer-only") {
		t.Fatalf("%q %v", out, err)
	}
	out, _, _ = execute(rootFor(t, true, factory), "--help")
	if strings.Contains(out, "Build Agent Plugins packages") {
		t.Fatal("unreleased author visible")
	}
	// Actual installer root parsing, stopped before all services/effects.
	root := agentpluginscli.NewRoot(agentpluginscli.App{})
	add, _, err := root.Find([]string{"add"})
	if err != nil {
		t.Fatal(err)
	}
	add.Args = cobra.ArbitraryArgs
	add.PreRunE = nil
	add.RunE = func(cmd *cobra.Command, _ []string) error {
		target, _ := cmd.Flags().GetString("target")
		if target != "cursor" {
			t.Fatal(target)
		}
		return nil
	}
	_, _, err = execute(root, "--target", "cursor", "add", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewPluginKitRoot(); err == nil {
		t.Fatal("empty success tree")
	}
	if _, err := NewCommand(Spec[string, string]{Use: "stub"}); err == nil {
		t.Fatal("missing runner allowed")
	}
	cached, _ := factory()
	f := func() (*cobra.Command, error) { return cached, nil }
	if _, err := NewPluginKitRoot(f); err != nil {
		t.Fatal(err)
	}
	if _, err := NewAuthorCommand(f); err == nil {
		t.Fatal("Cobra parent reused")
	}
}

func TestShadowAndWrongTypeRejected(t *testing.T) {
	for _, wrongType := range []bool{false, true} {
		root := &cobra.Command{Use: "agentplugins"}
		if wrongType {
			root.PersistentFlags().Bool("format", false, "")
		} else {
			root.PersistentFlags().String("format", "human", "")
		}
		calls := 0
		cmd, _ := fixtureFactory(func(context.Context, request) (result, error) { calls++; return result{}, nil }, Support{Format: true})()
		if !wrongType {
			cmd.Flags().String("format", "human", "")
		}
		root.AddCommand(cmd)
		_, _, err := execute(root, "validate", t.TempDir())
		if err == nil || calls != 0 {
			t.Fatal("bad adapter allowed", err)
		}
	}
}

func TestNilRunnerAndDecoderFailure(t *testing.T) {
	var run RunnerFunc[string, string]
	spec := Spec[string, string]{Use: "inspect", Decode: func(*cobra.Command, Options, []string) (string, error) { return "", nil }, Runner: run, Render: func(Streams, Options, string, error) error { return nil }}
	if _, err := NewCommand(spec); err == nil {
		t.Fatal("typed nil runner accepted")
	}
	calls := 0
	spec.Runner = RunnerFunc[string, string](func(context.Context, string) (string, error) { calls++; return "", nil })
	sentinel := errors.New("decode")
	spec.Decode = func(*cobra.Command, Options, []string) (string, error) { return "", sentinel }
	cmd, err := NewCommand(spec)
	if err != nil {
		t.Fatal(err)
	}
	root, err := NewPluginKitRoot(func() (*cobra.Command, error) { return cmd, nil })
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = execute(root, "inspect")
	if !errors.Is(err, sentinel) || calls != 0 {
		t.Fatal("decoder failure reached service", err)
	}
}

func TestConcurrentIndependentTrees(t *testing.T) {
	for i := 0; i < 8; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			path := t.TempDir()
			factory := fixtureFactory(func(_ context.Context, r request) (result, error) {
				if r.Path != path || r.Options.Format != "json" {
					t.Fatal("cross-tree request contamination")
				}
				return result{r.Path}, nil
			}, Support{Format: true})
			root := rootFor(t, i%2 == 0, factory)
			args := []string{"--format=json", "validate", path}
			if i%2 == 0 {
				args = []string{"--format=json", "author", "validate", path}
			}
			if _, _, err := execute(root, args...); err != nil {
				t.Fatal(err)
			}
		})
	}
}
