package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/commands"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoringcli"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"github.com/spf13/pflag"
)

const namespace = "prepared-authoring-v2"

// This is intentionally a separate envelope, not the v1 manifest array.
type manifest struct {
	Schema          string      `json:"schema"`
	Namespace       string      `json:"namespace"`
	Status          string      `json:"status"`
	Released        bool        `json:"released"`
	FactoryBaseline string      `json:"factory_baseline_sha"`
	SourceSHA       string      `json:"source_sha"`
	Sources         []sourcePin `json:"sources"`
	Surfaces        []surface   `json:"surfaces"`
}
type surface struct {
	Identity    string  `json:"identity"`
	CommandPath string  `json:"command_path"`
	Commands    []entry `json:"commands"`
}
type entry struct {
	Identity       string     `json:"identity"`
	CommandPath    string     `json:"command_path"`
	Slug           string     `json:"slug"`
	FileName       string     `json:"file_name"`
	Use            string     `json:"use"`
	Short          string     `json:"short"`
	Long           string     `json:"long"`
	Example        string     `json:"example"`
	Aliases        []string   `json:"aliases,omitempty"`
	Deprecated     string     `json:"deprecated,omitempty"`
	LocalFlags     []flagFact `json:"local_flags"`
	InheritedFlags []flagFact `json:"inherited_flags"`
}
type flagFact struct {
	Name         string `json:"name"`
	Shorthand    string `json:"shorthand,omitempty"`
	Type         string `json:"type"`
	Default      string `json:"default"`
	NoOptDefault string `json:"no_opt_default,omitempty"`
	Usage        string `json:"usage"`
	Deprecated   string `json:"deprecated,omitempty"`
}

// ReleaseSelection is the existing exported, non-executing construction seam.
// Nil args only observes the root; no ParseFlags, Execute, Args, Run, completion,
// project service, or installer callback is invoked by this adapter.
func trees() ([]*cobra.Command, error) {
	app := commands.App{PublicContract: true, Release: &commands.ReleaseOptions{}}
	plugin, _, _, err := app.ReleaseSelection(nil, authoringcli.NewReleasePluginKitRoot)
	if err != nil {
		return nil, err
	}
	installer, _, _, err := app.ReleaseSelection(nil, func(factories ...authoringcli.Factory) (*cobra.Command, error) {
		root := agentpluginscli.NewRoot(agentpluginscli.App{})
		author, err := authoringcli.NewReleaseAuthorCommand(factories...)
		if err != nil {
			return nil, err
		}
		root.AddCommand(author)
		return root, nil
	})
	if err != nil {
		return nil, err
	}
	for _, c := range installer.Commands() {
		if c.Name() == "author" {
			return []*cobra.Command{plugin, c}, nil
		}
	}
	return nil, fmt.Errorf("shared factory did not construct author subtree")
}

func visible(c *cobra.Command) bool {
	return !c.Hidden && c.Annotations[authoringcli.RejectionKey] == "" && !strings.HasPrefix(c.Name(), "__")
}

func flags(fs *pflag.FlagSet) []flagFact {
	out := []flagFact{}
	fs.VisitAll(func(f *pflag.Flag) {
		if !f.Hidden {
			out = append(out, flagFact{f.Name, f.Shorthand, f.Value.Type(), f.DefValue, f.NoOptDefVal, f.Usage, f.Deprecated})
		}
	})
	return out
}

// Drop excluded descendants before Markdown generation so SEE ALSO cannot leak
// them. Keep author attached to its genuine installer ancestor for inheritance.
func prepare(c *cobra.Command) {
	c.InitDefaultHelpCmd()
	c.InitDefaultHelpFlag()
	c.DisableAutoGenTag = true
	c.SetOut(io.Discard)
	c.SetErr(io.Discard)
	for _, child := range c.Commands() {
		if !visible(child) {
			c.RemoveCommand(child)
		} else {
			prepare(child)
		}
	}
}

func render(sha string, pins []sourcePin, roots []*cobra.Command) (map[string][]byte, error) {
	result := map[string][]byte{}
	m := manifest{Schema: "authoring-docs-manifest-v1", Namespace: namespace, Status: "prepared-not-release", SourceSHA: sha, FactoryBaseline: factoryBaselineSHA, Sources: pins}
	for _, root := range roots {
		prepare(root)
		s := surface{Identity: namespace + ":" + root.CommandPath(), CommandPath: root.CommandPath(), Commands: []entry{}}
		var walk func(*cobra.Command) error
		walk = func(c *cobra.Command) error {
			if !visible(c) {
				return nil
			}
			path := c.CommandPath()
			name := namespace + "/" + strings.ReplaceAll(path, " ", "_") + ".md"
			slug := namespace + "-" + strings.ReplaceAll(path, " ", "-")
			e := entry{Identity: namespace + ":" + path, CommandPath: path, Slug: slug, FileName: name, Use: c.Use, Short: c.Short, Long: c.Long, Example: c.Example, Aliases: append([]string(nil), c.Aliases...), Deprecated: c.Deprecated, LocalFlags: flags(c.LocalFlags()), InheritedFlags: flags(c.InheritedFlags())}
			s.Commands = append(s.Commands, e)
			var body bytes.Buffer
			fmt.Fprintf(&body, "<!-- namespace: %s; status: prepared-not-release; source-sha: %s -->\n\nPrepared reference only; not a public release.\n\n", namespace, sha)
			// The author-only surface has no installer root page. Its parent link is an
			// explicit source link rather than an invented local installer reference.
			link := func(target string) string {
				if target == "agentplugins.md" {
					return "https://github.com/777genius/universal-agent-plugins/blob/" + sha + "/cli/plugin-kit-ai/internal/agentpluginscli/root.go"
				}
				return target
			}
			if err := doc.GenMarkdownCustom(c, &body, link); err != nil {
				return err
			}
			result[name] = body.Bytes()
			for _, child := range c.Commands() {
				if err := walk(child); err != nil {
					return err
				}
			}
			return nil
		}
		if err := walk(root); err != nil {
			return nil, err
		}
		m.Surfaces = append(m.Surfaces, s)
	}
	body, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	result[namespace+"/manifest.json"] = append(body, '\n')
	return result, nil
}

func export(checkout, sha, out string) error {
	return exportTrees(checkout, sha, out, trees)
}

func exportTrees(checkout, sha, out string, load func() ([]*cobra.Command, error)) error {
	if out == "" {
		return fmt.Errorf("--out-dir is required")
	}
	pins, err := validateSource(checkout, sha)
	if err != nil {
		return err
	}
	roots, err := load()
	if err != nil {
		return err
	}
	if err := validateProjection(roots); err != nil {
		return err
	}
	files, err := render(sha, pins, roots)
	if err != nil {
		return err
	}
	// Exclusive directory creation refuses v1 outputs and all existing exports.
	// No replacement, cleanup, or recursive deletion is performed.
	if err := os.Mkdir(out, 0755); err != nil {
		return err
	}
	if err := os.Mkdir(filepath.Join(out, namespace), 0755); err != nil {
		return err
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(out, filepath.FromSlash(name)), body, 0644); err != nil {
			return err
		}
	}
	return nil
}

// Hash the actual deterministic rendered facts, with fixed provenance rather
// than caller SHA/host paths. Length framing and sorted names bind every byte.
// This is a reviewed golden, never regenerated automatically during export.
const reviewedProjection = "4e8f28f214097d3a6a7e314da872b277735c9b397624fac2a1065c847c6efb8a"

func projectionFingerprint(roots []*cobra.Command) (string, error) {
	files, err := render("SOURCE_SHA", nil, roots)
	if err != nil {
		return "", err
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	h := sha256.New()
	for _, name := range names {
		fmt.Fprintf(h, "%d:%s%d:", len(name), name, len(files[name]))
		h.Write(files[name])
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
func validateProjection(roots []*cobra.Command) error {
	got, err := projectionFingerprint(roots)
	if err != nil {
		return err
	}
	if got != reviewedProjection {
		return fmt.Errorf("loaded documentation projection mismatch: %s", got)
	}
	return nil
}
