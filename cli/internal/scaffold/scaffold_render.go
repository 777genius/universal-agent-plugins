package scaffold

import (
	"bytes"
	"fmt"
	"io/fs"
	"path"
	"text/template"
)

func RenderTemplate(tplName string, d Data) ([]byte, fs.FileMode, error) {
	raw, err := tmplFS.ReadFile(path.Join("templates", tplName))
	if err != nil {
		return nil, 0, err
	}
	t, err := template.New(tplName).Parse(string(raw))
	if err != nil {
		return nil, 0, fmt.Errorf("parse %s: %w", tplName, err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, d); err != nil {
		return nil, 0, fmt.Errorf("execute %s: %w", tplName, err)
	}
	mode := fs.FileMode(0o644)
	return buf.Bytes(), modeForPath("", tplName, mode), nil
}
