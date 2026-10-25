"""Inert Copilot collector contracts; no vendor or hosted execution."""
import json
from pathlib import Path
import tempfile
import unittest

from project_native_collectors import copilot


def encoded(value):
    return json.dumps(value).encode()


class FakeContext:
    def __init__(self, root):
        self.root = Path(root)
        self.home = self.root / "home"
        self.projects = {scope: self.root / scope for scope in ("a", "b", "user")}
        self.env, self.argv, self.calls, self.replace = {}, ["/not/executed/copilot"], [], {}
        for path in (self.home, *self.projects.values()):
            path.mkdir()

    def probe(self, label, arguments, project="a", extra_env=None, timeout=30):
        self.calls.append((label, arguments, project, extra_env, timeout))
        if label in self.replace:
            replacement = self.replace[label]
            return replacement() if callable(replacement) else replacement
        record = {"exit": 0, "error": None}
        if label == "version":
            return record, b"GitHub Copilot CLI 1.0.103.\n", b""
        if label.endswith("help"):
            return record, b"copilot --json COPILOT_PROVIDER_BASE_URL", b""
        if label.startswith("empty-"):
            return record, b"[]" if label.endswith("skills") else b'{"mcpServers":{}}', b""
        if label.endswith("skills"):
            rows = [{"name": "builtin", "source": "builtin", "enabled": True}]
            if project != "user":
                rows.append({"name": copilot.NAMES[project], "source": "project", "enabled": True,
                             "path": str(self.projects[project] / ".github/skills" / copilot.NAMES[project]),
                             "description": "Inert source marker"})
            return record, encoded(rows), b""
        rows = {copilot.NAMES["user"]: {"source": "user", "enabled": True, "type": "http", "url": copilot.URLS["user"]}}
        if label.startswith("trusted-") and project != "user":
            rows[copilot.NAMES[project]] = {"source": "workspace", "enabled": True, "type": "http", "url": copilot.URLS[project]}
        return record, encoded({"mcpServers": rows}), b""


class CopilotCollectorTests(unittest.TestCase):
    def context(self):
        directory = tempfile.TemporaryDirectory()
        self.addCleanup(directory.cleanup)
        return FakeContext(directory.name)

    def test_fixed_native_source_contract_never_claims_runtime(self):
        ctx = self.context()
        result = copilot.collect(ctx)
        self.assertEqual({row["id"] for row in result["cases"]}, {"copilot.help", "copilot.scope-observation"})
        facts = result["observations"]
        self.assertTrue(all(row["status"] == "passed" for row in result["cases"]))
        self.assertFalse(any(value for key, value in facts.items() if key.endswith("_qualified")))
        self.assertEqual(set(facts["scopes"]["a"]["mcp"]), {copilot.NAMES["user"], copilot.NAMES["a"]})
        self.assertEqual(set(facts["scopes"]["b"]["mcp"]), {copilot.NAMES["user"], copilot.NAMES["b"]})
        self.assertEqual(set(facts["untrusted_a"]["mcp"]), {copilot.NAMES["user"]})
        self.assertEqual(facts["scopes"]["user"]["skills"], {})
        self.assertEqual(len(ctx.calls), 18)
        self.assertEqual(len({call[0] for call in ctx.calls}), 18)
        for _, arguments, _, extra_env, timeout in ctx.calls:
            self.assertIn(arguments, [["--version"], *[list(args) for _, args, _ in copilot.HELP],
                                      ["skill", "list", "--json"], ["mcp", "list", "--json"]])
            self.assertEqual(extra_env, {"COPILOT_OFFLINE": "true"})
            self.assertEqual(timeout, 30)
        trust = json.loads((ctx.home / ".copilot/config.json").read_text())
        self.assertEqual(trust, {"trustedFolders": [str(ctx.projects[scope]) for scope in ("a", "b")]})

    def test_process_auth_and_wrong_version_fail_instead_of_boundary(self):
        for replacement in [({"exit": 1}, b"Authentication required", b""),
                            ({"exit": 0}, b"GitHub Copilot CLI 1.0.103.", b"Authentication required"),
                            ({"exit": 0, "error": "overflow"}, b"GitHub Copilot CLI 1.0.103.", b""),
                            ({}, b"GitHub Copilot CLI 1.0.103.", b""),
                            ({"exit": 0}, b"GitHub Copilot CLI 1.0.104.", b"")]:
            with self.subTest(replacement=replacement):
                ctx = self.context()
                ctx.replace["version"] = replacement
                with self.assertRaises(ValueError):
                    copilot.collect(ctx)
                self.assertEqual(len(ctx.calls), 1)

    def test_wrong_path_or_source_fails(self):
        for source, selected in [("personal", "a"), ("project", "b")]:
            ctx = self.context()
            ctx.replace["untrusted-a-skills"] = ({"exit": 0}, encoded([
                {"name": copilot.NAMES["a"], "source": source, "enabled": True,
                 "path": str(ctx.projects[selected] / ".github/skills" / copilot.NAMES["a"])}]), b"")
            with self.subTest(source=source, selected=selected), self.assertRaisesRegex(ValueError, "source/path"):
                copilot.collect(ctx)

    def test_untrusted_workspace_mcp_is_not_accepted(self):
        ctx = self.context()
        ctx.replace["untrusted-a-mcp"] = ({"exit": 0}, encoded({"mcpServers": {
            copilot.NAMES[scope]: {"source": "user" if scope == "user" else "workspace", "enabled": True,
                                  "type": "http", "url": copilot.URLS[scope]} for scope in ("a", "user")}}), b"")
        with self.assertRaisesRegex(ValueError, "MCP source"):
            copilot.collect(ctx)

    def test_shadow_config_and_cross_project_source_fail(self):
        for extra in ("shadow", "b"):
            ctx = self.context()
            ctx.replace["trusted-a-mcp"] = ({"exit": 0}, encoded({"mcpServers": {
                copilot.NAMES[scope]: {"source": "user" if scope == "user" else "workspace", "enabled": True,
                                      "type": "http", "url": copilot.URLS[scope]} for scope in ("a", "user", extra)}}), b"")
            with self.subTest(extra=extra), self.assertRaisesRegex(ValueError, "MCP source"):
                copilot.collect(ctx)

    def test_native_file_drift_fails(self):
        ctx = self.context()
        def mutated():
            (ctx.home / ".copilot/mcp-config.json").write_text("{}\n")
            return {"exit": 0}, encoded([]), b""
        ctx.replace["untrusted-a-skills"] = mutated
        with self.assertRaisesRegex(ValueError, "authored source"):
            copilot.collect(ctx)

    def test_trust_drift_fails(self):
        ctx = self.context()
        def mutated():
            (ctx.home / ".copilot/config.json").write_text('{"trustedFolders":["/"]}')
            return {"exit": 0}, encoded([]), b""
        ctx.replace["trusted-a-skills"] = mutated
        with self.assertRaisesRegex(ValueError, "folder trust"):
            copilot.collect(ctx)

    def test_home_cannot_be_project(self):
        ctx = self.context()
        ctx.projects["a"] = ctx.home
        with self.assertRaisesRegex(ValueError, "original user home"):
            copilot.collect(ctx)
        self.assertEqual(ctx.calls, [])

    def test_native_parent_alias_is_refused_without_outside_write(self):
        ctx = self.context()
        outside = self.context().root
        sentinel = outside / "mcp-config.json"
        def aliased():
            original = ctx.home / ".copilot"
            sentinel.write_bytes((original / "mcp-config.json").read_bytes())
            sentinel.chmod(0o600)
            original.rename(ctx.home / ".copilot-original")
            original.symlink_to(outside, target_is_directory=True)
            return {"exit": 0}, encoded([]), b""
        ctx.replace["untrusted-a-skills"] = aliased
        with self.assertRaisesRegex(ValueError, "parent alias"):
            copilot.collect(ctx)
        self.assertEqual(sentinel.read_bytes(), (ctx.home / ".copilot-original/mcp-config.json").read_bytes())
        self.assertFalse((outside / "config.json").exists())

    def test_dangling_trust_leaf_cannot_create_outside_content(self):
        ctx = self.context()
        outside = self.context().root
        target, sentinel = outside / "must-not-create.json", outside / "keep.txt"
        sentinel.write_text("outside retained\n")
        trust = ctx.home / ".copilot/config.json"
        trust.parent.mkdir()
        trust.symlink_to(target)
        with self.assertRaisesRegex(ValueError, "unsafe Copilot native trust"):
            copilot.collect(ctx)
        self.assertFalse(target.exists())
        self.assertEqual(sentinel.read_text(), "outside retained\n")


class CopilotParserTests(unittest.TestCase):
    def test_duplicate_json_keys_nonfinite_and_trailing_data_fail(self):
        for body in [b'{"mcpServers":{},"mcpServers":{}}', b'{"mcpServers":{},"x":NaN}',
                     b'{"mcpServers":{}} trailing', b'\xff']:
            with self.subTest(body=body), self.assertRaises(ValueError):
                copilot.parse_mcp(body)

    def test_wrong_shapes_and_duplicate_skills_fail(self):
        row = {"name": "name", "source": "project", "path": "/path", "enabled": True}
        for body in [encoded({"skills": []}), encoded([row, row]), encoded([{**row, "enabled": 1}]),
                     encoded([{"name": "name", "enabled": True}])]:
            with self.subTest(body=body), self.assertRaises(ValueError):
                copilot.parse_skills(body)
        for document in [[], {}, {"mcpServers": []}, {"mcpServers": {"name": {"source": "workspace", "enabled": 1}}}]:
            with self.subTest(document=document), self.assertRaises(ValueError):
                copilot.parse_mcp(encoded(document))


if __name__ == "__main__":
    unittest.main()
