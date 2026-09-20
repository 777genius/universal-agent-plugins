package filesystem

import (
	"bytes"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/777genius/plugin-kit-ai/cli/internal/skills/domain"
	"gopkg.in/yaml.v3"
)

func RenderTemplate(name string, data TemplateData) ([]byte, error) {
	raw, err := tmplFS.ReadFile(filepath.Join("templates", name))
	if err != nil {
		return nil, err
	}
	tpl, err := template.New(name).Funcs(template.FuncMap{
		"yamlString": yamlString,
	}).Parse(string(raw))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func CommandLine(spec domain.SkillSpec) string {
	command := strings.TrimSpace(spec.Command)
	if len(spec.Args) == 0 {
		return command
	}
	parts := []string{command}
	for _, arg := range spec.Args {
		parts = append(parts, quoteArg(arg))
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func quoteArg(arg string) string {
	if arg == "" {
		return "''"
	}
	if !strings.ContainsAny(arg, " \t\n'\"`$&|;<>*?()[]{}\\") {
		return arg
	}
	return "'" + strings.ReplaceAll(arg, "'", `'\''`) + "'"
}

func yamlString(v string) string {
	body, err := yaml.Marshal(v)
	if err != nil {
		return `""`
	}
	return strings.TrimSpace(string(body))
}
