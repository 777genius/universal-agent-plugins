package frontmatter

import "testing"

func TestParseAcceptsCRLFFrontmatter(t *testing.T) {
	t.Parallel()
	body := []byte("---\r\nname: crlf\r\ndescription: crlf skill\r\nexecution_mode: docs_only\r\nsupported_agents:\r\n  - claude\r\n---\r\n\r\n# CRLF\r\n\r\n## What it does\r\n\r\nx\r\n\r\n## When to use\r\n\r\ny\r\n\r\n## How to run\r\n\r\nz\r\n\r\n## Constraints\r\n\r\n- c\r\n")
	doc, err := Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Spec.Name != "crlf" {
		t.Fatalf("name = %q", doc.Spec.Name)
	}
}

func TestParseAcceptsUTF8BOM(t *testing.T) {
	t.Parallel()
	body := []byte("\ufeff---\nname: bom\ndescription: bom skill\nexecution_mode: docs_only\nsupported_agents:\n  - claude\n---\n\n# BOM\n\n## What it does\n\nx\n\n## When to use\n\ny\n\n## How to run\n\nz\n\n## Constraints\n\n- c\n")
	doc, err := Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Spec.Name != "bom" {
		t.Fatalf("name = %q", doc.Spec.Name)
	}
}

func TestParseAcceptsTerminatorAtEOF(t *testing.T) {
	t.Parallel()
	body := []byte("---\nname: eof\ndescription: eof skill\nexecution_mode: docs_only\nsupported_agents:\n  - claude\n---")
	doc, err := Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Spec.Name != "eof" {
		t.Fatalf("name = %q", doc.Spec.Name)
	}
	if doc.Body != "" {
		t.Fatalf("body = %q", doc.Body)
	}
}
