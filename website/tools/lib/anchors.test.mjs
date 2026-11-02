import test from "node:test";
import assert from "node:assert/strict";
import { authoredAnchors } from "./anchors.mjs";

test("only visible explicit authored anchors can survive a language switch", () => {
  const source = [
    '## Build from source {#build-from-source}', '<a id="restore-project-dependencies"></a>',
    '<!-- <a id="hidden-legacy"></a> -->', '<!--', '<a id="hidden-multiline"></a>', '-->',
    '```html', '<a id="example-only"></a>', '```',
    '## Automatic heading slug', '<a hidden id="hidden"></a>',
    '<!-- locale-historical-source:start', '<a id="historical"></a>', 'locale-historical-source:end -->'
  ].join("\n");
  assert.deepEqual(authoredAnchors(source), ["build-from-source", "restore-project-dependencies"]);
});
