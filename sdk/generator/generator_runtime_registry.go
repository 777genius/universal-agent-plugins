package generator

import (
	"fmt"
	"strings"
)

func renderRegistry(m model) string {
	var b strings.Builder
	b.WriteString("package gen\n\n")
	b.WriteString("import (\n")
	for _, p := range runtimeProfiles(m) {
		b.WriteString(fmt.Sprintf("\t%s %q\n", internalAlias(p.Platform), p.InternalImport))
	}
	b.WriteString("\t\"github.com/777genius/plugin-kit-ai/sdk/internal/runtime\"\n")
	b.WriteString(")\n\n")
	b.WriteString("type key struct { platform runtime.PlatformID; event runtime.EventID }\n\n")
	b.WriteString("var registry = map[key]runtime.Descriptor{\n")
	for _, e := range m.events {
		p := m.profiles[e.Platform]
		b.WriteString(fmt.Sprintf("\t{platform: %q, event: %q}: {\n", e.Platform, e.Event))
		b.WriteString(fmt.Sprintf("\t\tPlatform: %q,\n", e.Platform))
		b.WriteString(fmt.Sprintf("\t\tEvent: %q,\n", e.Event))
		b.WriteString(fmt.Sprintf("\t\tCarrier: %s,\n", carrierExpr(e.Carrier)))
		b.WriteString(fmt.Sprintf("\t\tDecode: %s.%s,\n", internalAlias(p.Platform), e.DecodeFunc))
		b.WriteString(fmt.Sprintf("\t\tEncode: %s.%s,\n", internalAlias(p.Platform), e.EncodeFunc))
		b.WriteString("\t},\n")
	}
	b.WriteString("}\n\n")
	b.WriteString("func Lookup(platform runtime.PlatformID, event runtime.EventID) (runtime.Descriptor, bool) {\n")
	b.WriteString("\td, ok := registry[key{platform: platform, event: event}]\n")
	b.WriteString("\treturn d, ok\n")
	b.WriteString("}\n")
	return b.String()
}
