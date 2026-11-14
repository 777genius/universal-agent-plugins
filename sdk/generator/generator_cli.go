package generator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/sdk/platformmeta"
)

func renderScaffoldPlatforms(m model) string {
	var b strings.Builder
	b.WriteString("package scaffold\n\n")
	b.WriteString("import (\n\t\"strings\"\n)\n\n")
	b.WriteString("var generatedPlatforms = map[string]PlatformDefinition{\n")
	for _, p := range scaffoldTargetProfiles(m) {
		b.WriteString(fmt.Sprintf("\t%q: {\n", p.ID))
		b.WriteString(fmt.Sprintf("\t\tName: %q,\n", p.ID))
		b.WriteString("\t\tFiles: []TemplateFile{\n")
		for _, file := range p.Scaffold.TemplateFiles {
			b.WriteString(fmt.Sprintf("\t\t\t{Path: %q, Template: %q, Extra: %t},\n", file.Path, file.Template, file.Extra))
		}
		b.WriteString("\t\t},\n")
		b.WriteString("\t},\n")
	}
	b.WriteString("}\n\n")
	b.WriteString("func LookupPlatform(name string) (PlatformDefinition, bool) {\n")
	b.WriteString("\tp, ok := generatedPlatforms[normalizePlatform(name)]\n\treturn p, ok\n}\n\n")
	b.WriteString("func normalizePlatform(name string) string {\n")
	b.WriteString("\tname = strings.ToLower(strings.TrimSpace(name))\n")
	b.WriteString("\tif name == \"\" { return \"codex-runtime\" }\n")
	b.WriteString("\treturn name\n")
	b.WriteString("}\n")
	return b.String()
}

func renderValidateRules(m model) string {
	var b strings.Builder
	b.WriteString("package validate\n\n")
	b.WriteString("import \"strings\"\n\n")
	b.WriteString("var generatedRules = map[string]Rule{\n")
	for _, p := range scaffoldTargetProfiles(m) {
		b.WriteString(fmt.Sprintf("\t%q: {\n", p.ID))
		b.WriteString(fmt.Sprintf("\t\tPlatform: %q,\n", p.ID))
		b.WriteString("\t\tRequiredFiles: []string{\n")
		for _, s := range p.Validate.RequiredFiles {
			b.WriteString(fmt.Sprintf("\t\t\t%q,\n", s))
		}
		b.WriteString("\t\t},\n")
		b.WriteString("\t\tForbiddenFiles: []string{\n")
		for _, s := range p.Validate.ForbiddenFiles {
			b.WriteString(fmt.Sprintf("\t\t\t%q,\n", s))
		}
		b.WriteString("\t\t},\n")
		b.WriteString("\t\tBuildTargets: []string{\n")
		for _, s := range p.Validate.BuildTargets {
			b.WriteString(fmt.Sprintf("\t\t\t%q,\n", s))
		}
		b.WriteString("\t\t},\n")
		b.WriteString("\t},\n")
	}
	b.WriteString("}\n\n")
	b.WriteString("func LookupRule(name string) (Rule, bool) {\n")
	b.WriteString("\tr, ok := generatedRules[normalizePlatform(name)]\n\treturn r, ok\n}\n\n")
	b.WriteString("func normalizePlatform(name string) string {\n")
	b.WriteString("\tname = strings.ToLower(strings.TrimSpace(name))\n")
	b.WriteString("\tif name == \"\" { return \"codex-runtime\" }\n")
	b.WriteString("\treturn name\n")
	b.WriteString("}\n")
	return b.String()
}

func scaffoldTargetProfiles(m model) []platformmeta.PlatformProfile {
	var out []platformmeta.PlatformProfile
	for _, p := range m.cliProfiles {
		if p.SDK.Status == platformmeta.StatusRuntimeSupported || p.SDK.Status == platformmeta.StatusScaffoldOnly {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}
