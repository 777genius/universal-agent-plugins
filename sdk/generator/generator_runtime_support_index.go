package generator

import "strings"

func renderSupportIndex() string {
	var b strings.Builder
	b.WriteString("package gen\n\n")
	b.WriteString("import \"github.com/777genius/plugin-kit-ai/sdk/internal/runtime\"\n\n")
	b.WriteString("func AllSupportEntries() []runtime.SupportEntry {\n")
	b.WriteString("\tentries := make([]runtime.SupportEntry, 0, len(claudeSupportEntries())+len(geminiSupportEntries())+len(codexSupportEntries())+len(cursorSupportEntries())+len(vscodeLocalSupportEntries()))\n")
	b.WriteString("\tentries = append(entries, claudeSupportEntries()...)\n")
	b.WriteString("\tentries = append(entries, geminiSupportEntries()...)\n")
	b.WriteString("\tentries = append(entries, codexSupportEntries()...)\n")
	b.WriteString("\tentries = append(entries, cursorSupportEntries()...)\n")
	b.WriteString("\tentries = append(entries, vscodeLocalSupportEntries()...)\n")
	b.WriteString("\treturn entries\n")
	b.WriteString("}\n")
	return b.String()
}
