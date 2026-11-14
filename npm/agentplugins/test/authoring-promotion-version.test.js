"use strict";
const assert = require("node:assert/strict");
const test = require("node:test");
const promotion = require("../scripts/authoring-promotion");

test("GitHub CLI compatibility accepts supported 2.x updates only", () => {
  assert.equal(promotion.compatibleGhVersion(`gh version ${promotion.GH_VERSION} (minimum)\nhttps://github.com/cli/cli/releases\n`), true);
  assert.equal(promotion.compatibleGhVersion("gh version 2.84.0 (newer minor)\n"), true);
  assert.equal(promotion.compatibleGhVersion("gh version 2.83.3 (newer patch)\n"), true);
  assert.equal(promotion.compatibleGhVersion("gh version 2.83.1 (too old)\n"), false);
  assert.equal(promotion.compatibleGhVersion("gh version 3.0.0 (unsupported major)\n"), false);
  assert.equal(promotion.compatibleGhVersion("gh version latest (malformed)\n"), false);
});
