package platformexec

import (
	"regexp"

	"github.com/777genius/plugin-kit-ai/cli/internal/geminimanifest"
)

type geminiPackageMeta = geminimanifest.PackageMeta

type importedGeminiExtension = geminimanifest.ImportedExtension

type geminiSetting struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description"`
	EnvVar      string `yaml:"env_var" json:"envVar"`
	Sensitive   bool   `yaml:"sensitive" json:"sensitive"`
}

type geminiContextSelection struct {
	ArtifactName string
	SourcePath   string
}

var geminiYAMLFileRe = regexp.MustCompile(`(?i)\.(yaml|yml)$`)

var geminiSettingEnvVarRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

var geminiThemeObjectKeys = map[string]struct{}{
	"background": {},
	"text":       {},
	"status":     {},
	"ui":         {},
}

var geminiThemeStringArrayKeys = map[string]struct{}{
	"GradientColors": {},
	"gradient":       {},
}
