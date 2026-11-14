package gen

import "github.com/777genius/plugin-kit-ai/sdk/internal/runtime"

func AllSupportEntries() []runtime.SupportEntry {
	entries := make([]runtime.SupportEntry, 0, len(claudeSupportEntries())+len(geminiSupportEntries())+len(codexSupportEntries())+len(cursorSupportEntries())+len(vscodeLocalSupportEntries())+len(qoderSupportEntries()))
	entries = append(entries, claudeSupportEntries()...)
	entries = append(entries, geminiSupportEntries()...)
	entries = append(entries, codexSupportEntries()...)
	entries = append(entries, cursorSupportEntries()...)
	entries = append(entries, vscodeLocalSupportEntries()...)
	entries = append(entries, qoderSupportEntries()...)
	entries = append(entries, piSupportEntries()...)
	return entries
}
