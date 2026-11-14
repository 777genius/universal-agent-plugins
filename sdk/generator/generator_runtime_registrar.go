package generator

import (
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/sdk/internal/descriptors/defs"
	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

func renderRegistrar(m model, p defs.PlatformProfile) string {
	var b strings.Builder
	b.WriteString("package " + p.PublicPackage + "\n\n")
	for _, e := range eventsForPlatform(m, p.Platform) {
		b.WriteString(fmt.Sprintf("// %s registers a handler for the %s %s.\n", e.Registrar.MethodName, strings.TrimSpace(platformLabel(e.Platform)), strings.TrimSpace(eventLabel(e.Event))))
		b.WriteString(fmt.Sprintf("func (r *Registrar) %s(fn func(%s) %s) {\n", e.Registrar.MethodName, e.Registrar.EventType, e.Registrar.ResponseType))
		b.WriteString(fmt.Sprintf("\tr.backend.Register(%q, %q, %s(fn))\n", e.Platform, e.Event, e.Registrar.WrapFunc))
		b.WriteString("}\n\n")
	}
	return b.String()
}

func platformLabel(platform runtime.PlatformID) string {
	switch platform {
	case "claude":
		return "Claude"
	case "codex":
		return "Codex"
	default:
		return string(platform)
	}
}

func eventLabel(event runtime.EventID) string {
	return string(event)
}
