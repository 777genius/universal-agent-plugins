package generator

import (
	"fmt"
	"strings"
)

func renderCompletenessTest(m model) string {
	var b strings.Builder
	b.WriteString("package gen\n\n")
	b.WriteString("import (\n")
	b.WriteString("\t\"github.com/777genius/plugin-kit-ai/sdk/internal/descriptors/defs\"\n")
	b.WriteString("\t\"testing\"\n")
	b.WriteString(")\n\n")
	b.WriteString("func TestGeneratedRegistryCompleteness(t *testing.T) {\n")
	b.WriteString("\tprofiles := defs.Profiles()\n")
	b.WriteString("\tevents := defs.Events()\n")
	b.WriteString(fmtString("profiles", len(m.profiles)))
	b.WriteString(fmtString("events", len(m.events)))
	b.WriteString("\tentries := AllSupportEntries()\n")
	b.WriteString("\tif len(entries) != len(events) { t.Fatalf(\"support entries = %d want %d\", len(entries), len(events)) }\n")
	b.WriteString("\tfor _, event := range events {\n")
	b.WriteString("\t\tif _, ok := Lookup(event.Platform, event.Event); !ok { t.Fatalf(\"missing descriptor %s/%s\", event.Platform, event.Event) }\n")
	b.WriteString("\t\tif event.Invocation.Kind == \"\" { t.Fatalf(\"missing invocation kind for %s/%s\", event.Platform, event.Event) }\n")
	b.WriteString("\t\tif event.Contract.Maturity == \"\" { t.Fatalf(\"missing contract maturity for %s/%s\", event.Platform, event.Event) }\n")
	b.WriteString("\t\tif event.DecodeFunc == \"\" || event.EncodeFunc == \"\" { t.Fatalf(\"missing codec refs for %s/%s\", event.Platform, event.Event) }\n")
	b.WriteString("\t\tif event.Registrar.MethodName == \"\" || event.Registrar.WrapFunc == \"\" { t.Fatalf(\"missing registrar metadata for %s/%s\", event.Platform, event.Event) }\n")
	b.WriteString("\t}\n")
	b.WriteString("\tfor _, profile := range profiles {\n")
	b.WriteString("\t\tif profile.Status == \"\" { t.Fatalf(\"missing status for %s\", profile.Platform) }\n")
	b.WriteString("\t\tif len(profile.TransportModes) == 0 { t.Fatalf(\"missing transport modes for %s\", profile.Platform) }\n")
	b.WriteString("\t\tif profile.Status != \"deferred\" && !profile.RuntimeOnly {\n")
	b.WriteString("\t\t\tif len(profile.Scaffold.RequiredFiles) == 0 || len(profile.Scaffold.TemplateFiles) == 0 { t.Fatalf(\"missing scaffold metadata for %s\", profile.Platform) }\n")
	b.WriteString("\t\t\tif len(profile.Validate.RequiredFiles) == 0 { t.Fatalf(\"missing validate metadata for %s\", profile.Platform) }\n")
	b.WriteString("\t\t}\n")
	b.WriteString("\t}\n")
	b.WriteString("}\n")
	renderSupportEntriesOrderTest(&b)
	return b.String()
}

func renderSupportEntriesOrderTest(b *strings.Builder) {
	b.WriteString("\n")
	b.WriteString("func TestSupportEntriesPreserveEventOrder(t *testing.T) {\n")
	b.WriteString("\tt.Parallel()\n\n")
	b.WriteString("\tentries := AllSupportEntries()\n")
	b.WriteString("\tevents := defs.Events()\n")
	b.WriteString("\tif len(entries) != len(events) { t.Fatalf(\"support entries = %d want %d\", len(entries), len(events)) }\n")
	b.WriteString("\tfor i, event := range events {\n")
	b.WriteString("\t\tentry := entries[i]\n")
	b.WriteString("\t\tif entry.Platform != event.Platform || entry.Event != event.Event {\n")
	b.WriteString("\t\t\tt.Fatalf(\"entry[%d] = %s/%s want %s/%s\", i, entry.Platform, entry.Event, event.Platform, event.Event)\n")
	b.WriteString("\t\t}\n")
	b.WriteString("\t}\n")
	b.WriteString("}\n")
}

func fmtString(name string, count int) string {
	return fmt.Sprintf("\tif len(%s) != %d { t.Fatalf(\"%s count = %%d\", len(%s)) }\n", name, count, name, name)
}
