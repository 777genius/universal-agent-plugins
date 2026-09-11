// Package project composes exact-root acquisition and immutable standard facts.
// A Result is private evidence, never a public JSON model or installer input.
package project

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packageview"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
)

type Result struct {
	Input packageview.Input
	Facts conformance.Facts
}

// Service is immutable configuration. Each Read owns its reader, decoder and lease.
type Service struct {
	Scratch string
	Limits  packageview.Limits
}

func (s Service) Read(ctx context.Context, exactRoot string) (Result, error) {
	return s.read(ctx, exactRoot, packageview.GeneratedStaging{})
}

// ReadGeneratedStaging validates a package this process just generated into
// its own private, exclusively owned staging directory, immediately before
// scaffold.Apply's atomic publish -- never external or user-selected content.
//
// dir must be the live handle scaffold's own Validate callback received for
// stagingRoot; it is used only to prove identity (see
// packageview.NewGeneratedStaging), never for a second read path. That proof
// authorizes relaxing only the Darwin profile's read-only-mount requirement
// for this one directory. Ordinary validate/inspect/test/doctor/compat
// requests call Read, which never receives this proof, and they never hold a
// pre-opened handle for a caller-supplied root, so they have no path to this
// method either.
func (s Service) ReadGeneratedStaging(ctx context.Context, exactRoot string, dir *os.Root) (result Result, err error) {
	generated, err := packageview.NewGeneratedStaging(dir)
	if err != nil {
		return result, err
	}
	return s.read(ctx, exactRoot, generated)
}

func (s Service) read(ctx context.Context, exactRoot string, generated packageview.GeneratedStaging) (result Result, err error) {
	exactRoot, err = readRoot(exactRoot)
	if err != nil {
		return result, err
	}
	lease, err := (packageview.Reader{TempDir: s.Scratch, Limits: s.Limits, Generated: generated}).Open(ctx, exactRoot)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, lease.Close()) }()
	result.Input = lease.Data()
	registry, err := specregistry.New()
	if err != nil {
		return result, err
	}
	decoder := conformance.Decoder{Registry: registry}
	result.Facts, err = decoder.DecodePlugin(ctx, document(result.Input.Plugin))
	// Core/schema failure never authorizes a component read. Nonconformant but
	// independently usable core facts (unknown fields/extensions) can continue.
	if err != nil || result.Facts.Package == nil {
		return result, err
	}
	input, err := lease.Capture(ctx)
	if err != nil {
		// Capture invalidates the lease on failure; do not retain an earlier identity.
		return Result{}, err
	}
	result.Input = input
	converted := convert(input)
	// Obtain exact decoded command/cwd facts, without parsing a second JSON model.
	// Preliminary unavailable-path findings are replaced by the final Decode.
	mcp, err := decoder.DecodeMCP(ctx, converted.MCP, result.Facts.Package.SchemaURI, nil)
	if err != nil {
		return result, err
	}
	if mcp.Package != nil {
		for name, server := range mcp.Package.MCP.Servers {
			if server.Type != "stdio" {
				continue
			}
			for _, field := range []string{"command", "cwd"} {
				value, _ := server.Decoded[field].(string)
				if value == "" {
					continue
				}
				if field == "command" && !strings.ContainsAny(value, `/\`) {
					continue
				}
				state := observePath(input, value, field)
				converted.Paths = append(converted.Paths, conformance.PathObservation{Item: name, Field: field, State: state})
			}
		}
	}
	result.Facts, err = decoder.Decode(ctx, converted)
	return result, err
}

func document(d packageview.Document) conformance.Document {
	return conformance.NewDocument(d.Path, conformance.InputState(d.State), d.Bytes)
}
func convert(v packageview.Input) conformance.Input {
	i := conformance.Input{Plugin: document(v.Plugin), MCP: document(v.MCP), SkillsRoot: conformance.InputState(v.SkillsRoot)}
	i.Coverage.Skills = conformance.NotEvaluated
	if v.Coverage.SkillsEnumerated {
		i.Coverage.Skills = conformance.Pass
	}
	i.Coverage.Filesystem = conformance.NotEvaluated
	if v.Coverage.InventoryComplete {
		i.Coverage.Filesystem = conformance.Pass
		for _, o := range v.Inventory {
			// Legacy metadata is intentionally outside portable interpretation. Other
			// withheld entries supply no positive containment evidence.
			if !o.Captured && o.Path != "plugin/plugin.yaml" {
				i.Coverage.Filesystem = conformance.NotEvaluated
			}
		}
	}
	for _, s := range v.Skills {
		i.Skills = append(i.Skills, conformance.SkillInput{Directory: strings.TrimPrefix(s.Directory, "skills/"), Document: document(s.Document)})
	}
	return i
}

// observePath resolves only retained inventory, in traversal order. It never
// cleans away an unobserved intermediary, resolves PATH or probes source paths.
// Missing/blocked/type evidence is unavailable, not an invented standard failure.
func observePath(v packageview.Input, value, field string) conformance.Outcome {
	if strings.HasPrefix(value, "${PLUGIN_DATA}") {
		return conformance.NotEvaluated
	}
	if value == "${PLUGIN_ROOT}" {
		return conformance.Pass
	}
	value = strings.TrimPrefix(value, "${PLUGIN_ROOT}/")
	if strings.HasPrefix(value, "/") || strings.Contains(value, `\`) {
		return conformance.NotEvaluated
	}
	entries := make(map[string]packageview.Observation, len(v.Inventory))
	for _, o := range v.Inventory {
		entries[o.Path] = o
	}
	todo := strings.Split(value, "/")
	var stack []string
	links := 0
	kind := "directory"
	for len(todo) > 0 {
		part := todo[0]
		todo = todo[1:]
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			if len(stack) == 0 {
				return conformance.Fail
			}
			stack = stack[:len(stack)-1]
			kind = "directory"
			continue
		}
		candidate := strings.Join(append(append([]string{}, stack...), part), "/")
		o, ok := entries[candidate]
		if !ok || !o.Captured || o.State != packageview.Present {
			return conformance.NotEvaluated
		}
		if o.Kind == "symlink" {
			links++
			if links > 40 || strings.HasPrefix(o.Target, "/") || strings.Contains(o.Target, `\`) {
				return conformance.NotEvaluated
			}
			todo = append(strings.Split(o.Target, "/"), todo...)
			continue
		}
		kind = o.Kind
		if len(todo) > 0 && kind != "directory" {
			return conformance.NotEvaluated
		}
		stack = append(stack, part)
	}
	if field == "command" && kind != "file" || field == "cwd" && kind != "directory" {
		return conformance.NotEvaluated
	}
	return conformance.Pass
}
