package pluginmanifest

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/platformexec"
	"github.com/777genius/plugin-kit-ai/cli/internal/scaffold"
)

func buildRootClaudeBoundaryArtifact(root string, layout authoredLayout) (*Artifact, error) {
	body, _, err := scaffold.RenderTemplate("ROOT.CLAUDE.md.tmpl", scaffold.Data{
		AuthoredRoot:       layout.Path(""),
		AuthoredReadmePath: layout.Path("README.md"),
	})
	if err != nil {
		return nil, err
	}
	merged, err := mergeManagedGuidanceFile(root, "CLAUDE.md", string(body))
	if err != nil {
		return nil, err
	}
	return &Artifact{RelPath: "CLAUDE.md", Content: []byte(merged)}, nil
}

func buildRootAgentsBoundaryArtifact(root string, layout authoredLayout, graph PackageGraph) (*Artifact, error) {
	body, _, err := scaffold.RenderTemplate("ROOT.AGENTS.md.tmpl", scaffold.Data{
		AuthoredRoot:       layout.Path(""),
		AuthoredReadmePath: layout.Path("README.md"),
	})
	if err != nil {
		return nil, err
	}
	withCursor, err := injectCursorAgentsSection(root, string(body), graph.Targets["cursor-workspace"])
	if err != nil {
		return nil, err
	}
	merged, err := mergeManagedGuidanceFile(root, "AGENTS.md", withCursor)
	if err != nil {
		return nil, err
	}
	return &Artifact{RelPath: "AGENTS.md", Content: []byte(merged)}, nil
}

func injectCursorAgentsSection(root, body string, state TargetState) (string, error) {
	rel := strings.TrimSpace(state.DocPath("agents_markdown"))
	if rel == "" {
		return body, nil
	}
	authored, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return "", err
	}
	content := strings.TrimSpace(string(authored))
	if content == "" {
		return body, nil
	}
	cursorSection := "\n\n" + platformexec.CursorAgentsSectionStart() + "\n" + content + "\n" + platformexec.CursorAgentsSectionEnd() + "\n"
	idx := strings.Index(body, managedGuidanceEnd)
	if idx < 0 {
		return body + cursorSection, nil
	}
	return body[:idx] + cursorSection + body[idx:], nil
}

func mergeManagedGuidanceFile(root, relPath, generated string) (string, error) {
	generated = strings.TrimRight(normalizeManagedText(generated), "\n") + "\n"
	full := filepath.Join(root, relPath)
	body, err := os.ReadFile(full)
	if err != nil {
		if os.IsNotExist(err) {
			return generated, nil
		}
		return "", err
	}
	existing := normalizeManagedText(string(body))
	start := strings.Index(existing, managedGuidanceStart)
	end := strings.Index(existing, managedGuidanceEnd)
	if start >= 0 && end > start {
		end += len(managedGuidanceEnd)
		updated := existing[:start] + generated + existing[end:]
		return strings.TrimRight(updated, "\n") + "\n", nil
	}
	if strings.TrimSpace(existing) == "" {
		return generated, nil
	}
	return strings.TrimRight(existing, "\n") + "\n\n" + generated, nil
}

func boundaryDocsForLayout(layout authoredLayout) []string {
	if strings.TrimSpace(layout.Path("")) == "" {
		return nil
	}
	return []string{"CLAUDE.md", "AGENTS.md"}
}

func generatedGuideForLayout(layout authoredLayout) string {
	if strings.TrimSpace(layout.Path("")) == "" {
		return ""
	}
	return "GENERATED.md"
}
