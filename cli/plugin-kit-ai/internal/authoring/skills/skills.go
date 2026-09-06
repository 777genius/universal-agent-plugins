// Package skills composes bounded standard-package Skills authoring. It has no
// legacy loader, npm lifecycle, runtime, client or ambient profile dependency.
package skills

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/report"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/scaffold"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packageview"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// ErrChecks means captured checks failed or are incomplete, not a filesystem
// operation error. Keep their existing policy layers in the public report.
var ErrChecks = errors.New("package skills checks incomplete or failed")

type Service struct {
	Projects project.Service
	Revision string
}
type Request struct {
	Root, Name, Description string
	IncludeRoot             bool
}

// ExactRoot defaults to the current directory only. Prefix relative spelling
// without cleaning away a symlink/.. intermediary; the rooted services enforce
// their own clean-path contract. No ancestor or profile discovery is performed.
func ExactRoot(root string) (string, error) {
	// Leading current-directory markers are safe and conventional in examples;
	// retain all interior traversal for the rooted service to reject.
	for strings.HasPrefix(root, "."+string(os.PathSeparator)) {
		root = strings.TrimPrefix(root, "."+string(os.PathSeparator))
	}
	if root == "" || root == "." {
		return os.Getwd()
	}
	if filepath.IsAbs(root) {
		return root, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return cwd + string(os.PathSeparator) + root, nil
}

func (s Service) read(ctx context.Context, command string, req Request) (project.Result, report.Report, error) {
	p, err := s.Projects.Read(ctx, req.Root)
	r := report.Build(command, s.Revision, p, false)
	if req.IncludeRoot {
		r.Root = req.Root
	}
	return p, r, err
}

// Validate retains shared failure isolation, profile provenance and unavailable
// coverage. It reports package-wide readiness alongside each immediate Skill;
// nested SKILL.md, scripts, references and assets are inert captured content.
func (s Service) Validate(ctx context.Context, req Request) (report.Report, error) {
	_, r, err := s.read(ctx, "skills validate", req)
	if err == nil && !r.Successful() {
		err = ErrChecks
	}
	return r, err
}

func (s Service) Init(ctx context.Context, req Request) (report.Report, error) {
	r := report.New("skills init", s.Revision)
	r.Mode = "local_mutation"
	if req.IncludeRoot {
		r.Root = req.Root
	}
	plan, err := scaffold.BuildSkillPlan(ctx, req.Name, req.Description)
	validate := func(ctx context.Context, body []byte) error { return checkSkill(ctx, req.Name, body) }
	if err == nil {
		err = pathpolicy.ValidatePortablePathSegment(req.Name)
	}
	if err == nil {
		err = validate(ctx, plan.File().Bytes)
	}
	if err != nil {
		r.AddError("skill_options_invalid", "Supply an exact portable Skill name and --description (1–1024 characters); names are never normalized.")
		return r, err
	}
	result, err := scaffold.ApplySkill(ctx, plan, req.Root, func(ctx context.Context, root string) (func(context.Context) error, error) {
		var p project.Result
		var e error
		p, r, e = s.read(ctx, "skills init", req)
		r.Mode = "local_mutation"
		if e != nil {
			return nil, e
		}
		coreOK := p.Facts.Package != nil && p.Facts.Package.FormatID == domain.FormatIDAgentPluginsV1 && p.Facts.Coverage.Plugin == conformance.Pass
		for _, f := range p.Facts.Findings {
			if f.Boundary == domain.BoundaryPlugin && f.Layer == conformance.Normative {
				coreOK = false
			}
		}
		if !coreOK || r.HostSafety.Status != report.Pass || !p.Input.Coverage.InventoryComplete || p.Facts.Coverage.Filesystem != conformance.Pass {
			r.AddError("skill_source_gate_failed", "Repair the exact standard package core and establish complete host containment before adding a Skill.")
			return nil, errors.New("source gate failed")
		}
		core := append([]byte(nil), p.Input.Plugin.Bytes...)
		return func(ctx context.Context) (err error) {
			// Capture only the core on recheck: our own private stage is not source
			// evidence. Reader.Open retains no-follow/nonblocking and legacy alias
			// protection, unlike a raw os.ReadFile compare-and-swap check.
			lease, err := (packageview.Reader{TempDir: s.Projects.Scratch, Limits: s.Projects.Limits}).Open(ctx, root)
			if err != nil {
				return err
			}
			defer func() { err = errors.Join(err, lease.Close()) }()
			input := lease.Data()
			if input.Plugin.State != packageview.Present || !bytes.Equal(input.Plugin.Bytes, core) {
				return errors.New("source core changed or became unavailable")
			}
			return nil
		}, nil
	}, validate)
	if result.Committed && err == nil {
		// Post-commit read supplies the actual resulting package identity. A failed
		// read or new concurrent invalidity must not hide the completed publication.
		_, r, err = s.read(ctx, "skills init", req)
		if err == nil && !r.Successful() {
			err = ErrChecks
		}
	}
	r.Mode = "local_mutation"
	r.Committed = result.Committed
	if result.Committed {
		r.Paths = []string{"skills/" + req.Name + "/SKILL.md"}
	}
	return r, err
}

// One shared decoder validates both the pure plan and the actual private stage.
// The scaffold seam never imports a parser or supplies its own profile rules.
func checkSkill(ctx context.Context, name string, body []byte) error {
	f, err := (conformance.Decoder{}).DecodeSkill(ctx, conformance.SkillInput{Directory: name,
		Document: conformance.NewDocument("skills/"+name+"/SKILL.md", conformance.Present, body)})
	if err != nil {
		return err
	}
	if f.Package == nil || f.Coverage.Skills != conformance.Pass || len(f.Findings) != 0 {
		return errors.New("skill does not pass the embedded normative profile")
	}
	return nil
}
