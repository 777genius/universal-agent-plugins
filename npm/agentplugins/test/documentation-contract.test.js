"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");

test("npm facade metadata exactly names the UAP product endpoints", () => {
  const pkg = JSON.parse(fs.readFileSync(path.resolve(__dirname, "../package.json"), "utf8"));
  assert.equal(pkg.homepage, "https://777genius.github.io/universal-agent-plugins/");
  assert.deepEqual(pkg.repository, {
    type: "git",
    url: "git+https://github.com/777genius/universal-agent-plugins.git",
    directory: "npm/agentplugins"
  });
  assert.deepEqual(pkg.bugs, { url: "https://github.com/777genius/universal-agent-plugins/issues" });
});

test("detached package execution sentinel", (t) => {
  const expectedRoot = process.env.AGENTPLUGINS_DETACHED_ASSERT_ROOT;
  if (!expectedRoot) {
    t.skip("only asserted by the detached staged-package subprocess");
    return;
  }
  assert.equal(
    fs.realpathSync(path.resolve(__dirname, "..")),
    fs.realpathSync(path.resolve(expectedRoot))
  );
  assert.equal(fs.existsSync(path.resolve(__dirname, "../../../README.md")), false);
});

const documents = [
  ["root README", path.resolve(__dirname, "../../../README.md")],
  ["npm README", path.resolve(__dirname, "../README.md")]
];

function packagedDocuments() {
  const available = documents.filter(([, filename]) => fs.existsSync(filename));
  assert.ok(available.length > 0, "no packaged Agentplugins documentation found");
  return available;
}

const allowedTargets = new Set([
  "codex", "chatgpt", "cursor", "copilot", "vscode", "kiro",
  "claude", "gemini", "opencode", "cline", "windsurf"
]);
const exactSourcePattern = /^(?:github:)?[A-Za-z0-9][A-Za-z0-9-]*\/[A-Za-z0-9][A-Za-z0-9._-]*@[0-9a-f]{40}\/\/[A-Za-z0-9._/-]+$/;

function bashCommands(markdown) {
  const commands = [];
  for (const match of markdown.matchAll(/```(?:bash|shell)\s*\n([\s\S]*?)```/g)) {
    for (const line of match[1].split(/\r?\n/)) {
      const command = line.trim();
      if (command && !command.startsWith("#")) commands.push(command);
    }
  }
  return commands;
}

function commandArgument(command, flag) {
  const words = command.split(/\s+/);
  const index = words.indexOf(flag);
  return index < 0 ? "" : words[index + 1] || "";
}

test("public Agentplugins documentation keeps copyable commands within the CLI contract", () => {
  for (const [label, filename] of packagedDocuments()) {
    const markdown = fs.readFileSync(filename, "utf8");
    const commands = bashCommands(markdown).filter(
      (command) => command.startsWith("npx universal-agent-plugins ") || command.startsWith("agentplugins ")
    );
    assert.ok(commands.length > 0, `${label} has no copyable universal-agent-plugins commands`);
    const expectedFirstCommand = label === "root README"
      ? "agentplugins add context7"
      : "npx universal-agent-plugins add context7";
    assert.equal(
      commands[0],
      expectedFirstCommand,
      `${label}: the first command must keep the no-target interactive quick start`
    );

    for (const command of commands) {
      assert.doesNotMatch(command, /(?:^|\s)--yes(?:\s|$)/, `${label}: --yes is not a public option`);
      const target = commandArgument(command, "--target");
      if (target) {
        assert.match(target, /^[a-z]+(?:,[a-z]+)*$/, `${label}: malformed --target value in ${command}`);
        const values = target.split(",");
        assert.equal(new Set(values).size, values.length, `${label}: duplicate target in ${command}`);
        for (const value of values) assert.ok(allowedTargets.has(value), `${label}: unknown target ${value}`);
      }

      const words = command.split(/\s+/);
      for (const word of words) {
        if (!word.includes("@") || !word.includes("//")) continue;
        assert.match(word, exactSourcePattern, `${label}: direct GitHub source must use a full lowercase SHA`);
      }
    }

    for (const verb of ["add", "update", "repair", "remove"]) {
      assert.ok(
        commands.some(
          (command) =>
            command.startsWith(`npx universal-agent-plugins ${verb} `) ||
            command.startsWith(`agentplugins ${verb} `)
        ),
        `${label}: missing ${verb} example`
      );
    }
    assert.ok(commands.some((command) => command.includes("--target codex,cursor,kiro")), `${label}: missing explicit three-target example`);
  }
});

test("root documentation recommends the native Go installer without hiding npm", (t) => {
  if (!fs.existsSync(documents[0][1])) {
    t.skip("root README is intentionally absent from the detached npm package");
    return;
  }
  const markdown = fs.readFileSync(documents[0][1], "utf8");
  assert.match(markdown, /The native CLI does not require Node\.js/i);
  assert.match(markdown, /brew install 777genius\/agentplugins\/agentplugins/);
  assert.match(
    markdown,
    /https:\/\/raw\.githubusercontent\.com\/777genius\/universal-agent-plugins\/main\/install\.sh/
  );
  assert.match(
    markdown,
    /https:\/\/raw\.githubusercontent\.com\/777genius\/universal-agent-plugins\/main\/install\.ps1/
  );
  assert.match(markdown, /npx universal-agent-plugins add context7/);
});

test("public package documentation points to the authoritative product source", () => {
  const markdown = fs.readFileSync(documents[1][1], "utf8");
  assert.match(markdown, /https:\/\/github\.com\/777genius\/universal-agent-plugins(?:\)|\b)/i);
  assert.match(markdown, /versioned Go binary/i);
  assert.doesNotMatch(markdown, /product home[\s\S]{0,100}(?:npm facade|facade source)/i);
});

test("package documentation links current client evidence without stale release claims", () => {
  const markdown = fs.readFileSync(documents[1][1], "utf8");
  assert.match(markdown, /https:\/\/github\.com\/777genius\/universal-agent-plugins\/blob\/main\/docs\/AGENTPLUGINS_CLIENT_E2E\.md/);
  assert.doesNotMatch(markdown, /historical lifecycle evidence collected for/i);
  assert.doesNotMatch(markdown, /0\.1\.22/);
});

test("copyable direct-source examples use a marked replacement full SHA", () => {
  const placeholder = "0123456789abcdef0123456789abcdef01234567";
  for (const [label, filename] of [documents[1]]) {
    const markdown = fs.readFileSync(filename, "utf8");
    assert.match(
      markdown,
      new RegExp(`[A-Za-z0-9-]+/[A-Za-z0-9._-]+@${placeholder}//[A-Za-z0-9._/-]+`),
      `${label}: full-SHA source example missing`
    );
    assert.ok(markdown.includes("add ./my-plugin"), `${label}: local source example missing`);
    assert.match(markdown, /full 40-character commit SHA/i, `${label}: full-SHA replacement instruction missing`);
    assert.doesNotMatch(markdown, /@commit(?:\/\/|\b)/i, `${label}: literal @commit is not copyable`);
  }
});
