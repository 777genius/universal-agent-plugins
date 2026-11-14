package generator

import (
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/sdk/internal/descriptors/defs"
)

func renderSupportBucket(m model, platform string) string {
	var b strings.Builder
	b.WriteString("package gen\n\n")
	b.WriteString("import \"github.com/777genius/plugin-kit-ai/sdk/internal/runtime\"\n\n")
	b.WriteString(fmt.Sprintf("func %s() []runtime.SupportEntry {\n", supportBucketFuncName(platform)))
	b.WriteString("\treturn []runtime.SupportEntry{\n")
	for _, e := range supportBucketEvents(m, platform) {
		b.WriteString(renderSupportBucketEntry(m.profiles[e.Platform], e))
	}
	b.WriteString("\t}\n")
	b.WriteString("}\n")
	return b.String()
}

func supportBucketEvents(m model, platform string) []defs.EventDescriptor {
	events := make([]defs.EventDescriptor, 0, len(m.events))
	for _, e := range m.events {
		if string(e.Platform) == platform {
			events = append(events, e)
		}
	}
	return events
}
