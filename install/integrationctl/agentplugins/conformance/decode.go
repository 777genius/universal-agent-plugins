package conformance

import (
	"context"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"sort"
)

// Decode evaluates an already captured package. It does not collect files or
// discover a format. Coverage.Skills is the caller's completeness of immediate
// discovery, and Coverage.Filesystem its rooted containment evidence.
func (d Decoder) Decode(ctx context.Context, input Input) (Facts, error) {
	l := d.Limits.bounded()
	total := len(input.Plugin.body) + len(input.MCP.body)
	if len(input.Skills) > l.Members || len(input.Paths) > l.Members {
		return d.limitFacts("input_member_limit"), ctx.Err()
	}
	for _, s := range input.Skills {
		total += len(s.Document.body)
		if total > l.AggregateBytes {
			return d.limitFacts("input_aggregate_limit"), ctx.Err()
		}
	}
	if total > l.AggregateBytes {
		return d.limitFacts("input_aggregate_limit"), ctx.Err()
	}
	f, err := d.DecodePlugin(ctx, input.Plugin)
	if err != nil {
		return f, err
	}
	// Core failure precludes interpretation of components under a guessed version.
	// Unknown fields/nonobject extensions can still yield an independently usable core.
	if f.Package == nil {
		f.finish(l)
		return f, nil
	}
	pkg := f.Package
	m, err := d.DecodeMCP(ctx, input.MCP, pkg.SchemaURI, input.Paths)
	if err != nil {
		return f, err
	}
	f.merge(m)
	f.Coverage.MCP = m.Coverage.MCP
	if m.Package != nil {
		pkg.MCP = m.Package.MCP
	}
	f.Coverage.Filesystem = validOutcome(input.Coverage.Filesystem)
	switch input.SkillsRoot {
	case Absent:
		f.Coverage.Skills = Pass
	case Present:
		f.Coverage.Skills = NotEvaluated
		if input.Coverage.Skills == Pass {
			f.Coverage.Skills = Pass
		} else {
			f.add("skills_discovery_incomplete", "host/input", "skills", "", "", HostSafety, domain.BoundarySkill)
		} // Failed discovery is unavailable evidence.
	case WrongKind:
		f.Coverage.Skills = Fail
		pkg.Inventory.InvalidSkillsRoot = true
		f.add("skills_root_invalid", "plugins/6.2", "skills", "", "", Normative, domain.BoundarySkill)
	default:
		f.Coverage.Skills = NotEvaluated
		pkg.Inventory.InvalidSkillsRoot = true
		f.add("skills_root_unavailable", "host/input", "skills", "", "", HostSafety, domain.BoundarySkill)
	}
	// A failed/partial directory observation must not erase independently captured
	// Skill facts. Conversely, contradictory absence must never become complete.
	if len(input.Skills) > 0 && (input.SkillsRoot == Absent || input.SkillsRoot == WrongKind) {
		f.add("skills_input_inconsistent", "host/input", "skills", "", "", HostSafety, domain.BoundarySkill)
		f.Coverage.Skills = mergeOutcome(f.Coverage.Skills, NotEvaluated)
	}
	order := make([]int, len(input.Skills))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(i, j int) bool { return input.Skills[order[i]].Directory < input.Skills[order[j]].Directory })
	seen := map[string]bool{}
	for _, i := range order {
		s := input.Skills[i]
		if seen[s.Directory] {
			delete(pkg.Skills, s.Directory)
			f.add("skill_input_duplicate", "host/input", "skills", "", s.Directory, HostSafety, domain.BoundarySkill)
			f.Coverage.Skills = mergeOutcome(f.Coverage.Skills, NotEvaluated)
			continue
		}
		seen[s.Directory] = true
		part, err := d.DecodeSkill(ctx, s)
		if err != nil {
			return f, err
		}
		f.merge(part)
		f.Coverage.Skills = mergeOutcome(f.Coverage.Skills, part.Coverage.Skills)
		if part.Package != nil {
			for name, skill := range part.Package.Skills {
				pkg.Skills[name] = skill
			}
		} else if s.Document.State != Absent && s.Document.State != WrongKind {
			pkg.Inventory.InvalidSkills = append(pkg.Inventory.InvalidSkills, s.Directory)
		}
		// Bound aggregate findings as components are processed, not after accumulation.
		if len(f.Findings) > l.Diagnostics {
			f.finish(l)
		}
	}
	if f.Coverage.Filesystem != Pass {
		layer := HostSafety
		code := "filesystem_coverage_unavailable"
		if f.Coverage.Filesystem == Fail {
			layer = Normative
			code = "filesystem_containment_invalid"
		}
		f.add(code, "plugins/4.1", "", "", "", layer, domain.BoundaryPlugin)
	}
	pkg.Inventory.MCPPresent = pkg.MCP.Present
	pkg.Inventory.MCPEnabled = pkg.MCP.Enabled
	for name := range pkg.MCP.Servers {
		pkg.Inventory.MCPServers = append(pkg.Inventory.MCPServers, name)
	}
	for name := range pkg.MCP.InvalidServer {
		pkg.Inventory.InvalidMCPServer = append(pkg.Inventory.InvalidMCPServer, name)
	}
	sort.Strings(pkg.Inventory.InvalidMCPServer)
	for name := range pkg.Skills {
		pkg.Inventory.Skills = append(pkg.Inventory.Skills, name)
	}
	for name := range pkg.Manifest.Extensions {
		pkg.Inventory.Extensions = append(pkg.Inventory.Extensions, name)
	}
	sort.Strings(pkg.Inventory.MCPServers)
	sort.Strings(pkg.Inventory.Skills)
	sort.Strings(pkg.Inventory.Extensions)
	f.Package = pkg
	if err := ctx.Err(); err != nil {
		return f, err
	}
	f.finish(l)
	return f, nil
}
func validOutcome(o Outcome) Outcome {
	if o == Pass || o == Fail {
		return o
	}
	return NotEvaluated
}
func (f *Facts) merge(other Facts) {
	for _, v := range other.Findings {
		if f.maxFindings > 0 && len(f.Findings) >= f.maxFindings {
			f.truncated = true
			break
		}
		f.Findings = append(f.Findings, v)
	}
	f.truncated = f.truncated || other.truncated
	f.normativeFailure = f.normativeFailure || other.normativeFailure
	f.SchemaIDs = append(f.SchemaIDs, other.SchemaIDs...)
	for p, d := range other.raw {
		f.raw[p] = d
	}
}
func (d Decoder) limitFacts(code string) Facts {
	f := d.newFacts()
	f.add(code, "host/bounds", "", "", "", HostSafety, domain.BoundaryPlugin)
	f.finish(d.Limits.bounded())
	return f
}
