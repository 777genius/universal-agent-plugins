package platformmeta

// TemplateFile describes a scaffolded output file and its template source.
type TemplateFile struct {
	// Path is the destination path inside the generated project.
	Path string
	// Template is the template file used to generate the destination.
	Template string
	// Extra marks optional scaffold material that is not required by default.
	Extra bool
}

// ScaffoldMeta describes the generated file set for `plugin-kit-ai init`.
type ScaffoldMeta struct {
	// RequiredFiles must exist in a scaffolded target.
	RequiredFiles []string
	// OptionalFiles may be added for richer scaffolds.
	OptionalFiles []string
	// ForbiddenFiles must be absent for a valid target layout.
	ForbiddenFiles []string
	// TemplateFiles maps scaffold output files to their generate templates.
	TemplateFiles []TemplateFile
}

// ValidateMeta describes the contract enforced by `plugin-kit-ai validate`.
type ValidateMeta struct {
	// RequiredFiles must exist for the target to validate successfully.
	RequiredFiles []string
	// ForbiddenFiles must not exist for the target to validate successfully.
	ForbiddenFiles []string
	// BuildTargets lists buildable artifacts that validation should check.
	BuildTargets []string
}
