package authoringcli

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestFreshExecutionLifecycle(t *testing.T) {
	for _, nested := range []bool{false, true} {
		for _, mode := range []string{"parse", "required", "group", "help", "cancel", "args", "prerun", "runner"} {
			t.Run(mode+map[bool]string{true: "/installer", false: "/standalone"}[nested], func(t *testing.T) {
				var got []Options
				var selections [][]string
				var trees []*cobra.Command
				factory := Factory(func() (*cobra.Command, error) {
					leaf := func() (*cobra.Command, error) {
						return NewCommand(Spec[Options, string]{Use: "inspect", Args: cobra.NoArgs, Support: Support{true, true, true, true},
							Configure: func(cmd *cobra.Command) {
								cmd.Flags().String("name", "", "required name")
								cmd.Flags().Bool("left", false, "group")
								cmd.Flags().Bool("right", false, "group")
								if mode == "required" {
									if err := cmd.MarkFlagRequired("name"); err != nil {
										t.Fatal(err)
									}
								}
								if mode == "group" {
									cmd.MarkFlagsRequiredTogether("left", "right")
								}
							},
							Decode: func(cmd *cobra.Command, opts Options, _ []string) (Options, error) {
								selected, err := cmd.Flags().GetStringSlice("select")
								selections = append(selections, append([]string(nil), selected...))
								return opts, err
							},
							Runner: RunnerFunc[Options, string](func(_ context.Context, opts Options) (string, error) {
								got = append(got, opts)
								if mode == "runner" && len(got) == 1 {
									return "", errors.New("runner failure")
								}
								return "", nil
							}),
							Render: func(Streams, Options, string, error) error { return nil },
						})
					}
					root := rootFor(t, nested, leaf)
					root.PersistentFlags().StringSlice("select", []string{"base"}, "inherited selection")
					if mode == "prerun" && len(trees) == 0 {
						root.PersistentPreRunE = func(*cobra.Command, []string) error { return errors.New("pre-run failure") }
					}
					trees = append(trees, root)
					return root, nil
				})
				invoke := func(ctx context.Context, flags ...string) error {
					args := []string{"inspect"}
					if nested {
						args = []string{"author", "inspect"}
					}
					var out, diag bytes.Buffer
					return factory.Execute(ctx, append(args, flags...), Streams{strings.NewReader(""), &out, &diag})
				}
				flags := []string{"--format=json", "--target=cursor", "--dry-run", "--select=first"}
				ctx := context.Background()
				switch mode {
				case "parse":
					flags = append(flags, "--unknown")
				case "group":
					flags = append(flags, "--left")
				case "help":
					flags = append(flags, "--help")
				case "args":
					flags = append(flags, "extra")
				case "cancel":
					var cancel context.CancelFunc
					ctx, cancel = context.WithCancel(ctx)
					cancel()
				}
				err := invoke(ctx, flags...)
				if (mode == "help") != (err == nil) {
					t.Fatalf("first invocation: %v", err)
				}
				if mode == "cancel" && !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
				if len(got) != map[bool]int{true: 1, false: 0}[mode == "runner"] {
					t.Fatalf("unexpected runner calls: %v", got)
				}
				clean := []string{}
				if mode == "required" {
					clean = append(clean, "--name=demo")
				}
				if err := invoke(context.Background(), clean...); err != nil {
					t.Fatal(err)
				}
				if got[len(got)-1] != (Options{Format: "human"}) {
					t.Fatalf("leaked options: %v", got)
				}
				if !reflect.DeepEqual(selections[len(selections)-1], []string{"base"}) {
					t.Fatal(selections)
				}
				if err := invoke(context.Background(), append(clean, "--select=next")...); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(selections[len(selections)-1], []string{"next"}) {
					t.Fatal(selections)
				}
				// Authoring must not reset values or Changed state owned by the installer.
				if f := trees[0].PersistentFlags().Lookup("target"); f.Value.String() != "cursor" || !f.Changed {
					t.Fatalf("installer flag mutated: %+v", f)
				}
				for i := 1; i < len(trees); i++ {
					if trees[0] == trees[i] || trees[0].PersistentFlags().Lookup("target").Value == trees[i].PersistentFlags().Lookup("target").Value {
						t.Fatal("shared flag state")
					}
				}
			})
		}
	}
}

func TestFreshInheritedAndLocalSlicesAfterSuccess(t *testing.T) {
	for _, inherited := range []bool{false, true} {
		var got [][]string
		factory := Factory(func() (*cobra.Command, error) {
			leaf := func() (*cobra.Command, error) {
				return NewCommand(Spec[[]string, string]{Use: "inspect",
					Configure: func(c *cobra.Command) {
						if !inherited {
							c.Flags().StringSlice("select", []string{"base"}, "")
						}
					},
					Decode: func(c *cobra.Command, _ Options, _ []string) ([]string, error) {
						return c.Flags().GetStringSlice("select")
					},
					Runner: RunnerFunc[[]string, string](func(_ context.Context, s []string) (string, error) {
						got = append(got, append([]string(nil), s...))
						return "", nil
					}),
					Render: func(Streams, Options, string, error) error { return nil },
				})
			}
			root, err := NewPluginKitRoot(leaf)
			if err == nil && inherited {
				root.PersistentFlags().StringSlice("select", []string{"base"}, "")
			}
			return root, err
		})
		for _, args := range [][]string{{"inspect", "--select=first"}, {"inspect"}, {"inspect", "--select=next"}} {
			var out bytes.Buffer
			if err := factory.Execute(context.Background(), args, Streams{Out: &out, Err: &out}); err != nil {
				t.Fatal(err)
			}
		}
		if !reflect.DeepEqual(got, [][]string{{"first"}, {"base"}, {"next"}}) {
			t.Fatal(got)
		}
	}
}

func TestExecutionRejectsConsumedTree(t *testing.T) {
	root := rootFor(t, false, fixtureFactory(func(context.Context, request) (result, error) { return result{}, nil }, Support{}))
	factory := Factory(func() (*cobra.Command, error) { return root, nil })
	var out bytes.Buffer
	streams := Streams{Out: &out, Err: &out}
	if err := factory.Execute(context.Background(), []string{"validate", "--help"}, streams); err != nil {
		t.Fatal(err)
	}
	if err := factory.Execute(context.Background(), []string{"validate", t.TempDir()}, streams); err == nil || !strings.Contains(err.Error(), "single-invocation") {
		t.Fatalf("cached tree accepted: %v", err)
	}
}

func TestExecutionFactoryErrors(t *testing.T) {
	sentinel := errors.New("construction")
	for _, factory := range []Factory{nil, func() (*cobra.Command, error) { return nil, nil }, func() (*cobra.Command, error) { return nil, sentinel }, func() (*cobra.Command, error) {
		parent := &cobra.Command{Use: "parent"}
		child := &cobra.Command{Use: "child"}
		parent.AddCommand(child)
		return child, nil
	}} {
		if err := factory.Execute(context.Background(), nil, Streams{}); err == nil {
			t.Fatal("invalid factory accepted")
		}
	}
	factory := Factory(func() (*cobra.Command, error) { return nil, sentinel })
	if err := factory.Execute(context.Background(), nil, Streams{}); err != sentinel {
		t.Fatal("construction error replaced", err)
	}
}
