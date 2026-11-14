package scaffold

import "strings"

func planFilesFor(platform, runtime string, extras, typescript, sharedRuntimePackage bool) []TemplateFile {
	files := append([]TemplateFile(nil), filesFor(platform, runtime, extras, typescript, sharedRuntimePackage)...)
	for _, file := range runtimeTestScaffoldFiles(platform) {
		files = appendUniqueTemplateFile(files, file)
	}
	return files
}

func appendUniqueTemplateFile(files []TemplateFile, candidate TemplateFile) []TemplateFile {
	for _, file := range files {
		if file.Path == candidate.Path {
			return files
		}
	}
	return append(files, candidate)
}

func expandTemplateFiles(files []TemplateFile, d Data) []PlannedFile {
	out := make([]PlannedFile, 0, len(files))
	for _, file := range files {
		if file.Extra && !d.WithExtras {
			continue
		}
		out = append(out, PlannedFile{
			RelPath:  expandPathTemplate(file.Path, d),
			Template: file.Template,
		})
	}
	return out
}

func expandPathTemplate(path string, d Data) string {
	path = strings.ReplaceAll(path, "{{.ProjectName}}", d.ProjectName)
	path = strings.ReplaceAll(path, "{{.Platform}}", d.Platform)
	return path
}
