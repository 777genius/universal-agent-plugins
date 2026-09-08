import assert from "node:assert/strict";
import test from "node:test";

import {
  normalizeGeneratedMarkdown,
  renderFrontmatter,
  stripMarkdownLinks
} from "./frontmatter.mjs";

test("frontmatter quotes untrusted scalar and list values without YAML injection", () => {
  const rendered = renderFrontmatter({
    title: 'quoted "value"\\path\nnext: true',
    tags: ["safe", "line\r\nbreak"]
  });
  assert.equal(
    rendered,
    '---\ntitle: "quoted \\"value\\"\\\\path\\nnext: true"\ntags:\n  - "safe"\n  - "line\\r\\nbreak"\n---\n'
  );
});

test("markdown links are stripped in linear scans while malformed input is preserved", () => {
  assert.equal(
    stripMarkdownLinks("Use [plain](https://example.com) and [angle](<https://example.com/a>)."),
    "Use plain and angle."
  );
  assert.equal(stripMarkdownLinks("Keep [broken](target\nand [open"), "Keep [broken](target\nand [open");
  assert.equal(stripMarkdownLinks("Keep [label] without a target"), "Keep [label] without a target");
  assert.equal(stripMarkdownLinks("[escaped\\] label](target)"), "escaped\\] label");
});

test("generated markdown removes complete comments and escapes incomplete HTML", () => {
  assert.equal(normalizeGeneratedMarkdown("before\n<!-- hidden -->\nafter"), "before\nafter");
  assert.equal(normalizeGeneratedMarkdown("before <!-- incomplete"), "before &lt;!-- incomplete");
  assert.equal(normalizeGeneratedMarkdown("<script>alert(1)</script>"), "&lt;script&gt;alert(1)&lt;/script&gt;");
});
