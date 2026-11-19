"""Inert Qoder acquisition/help contracts; no vendor payload is acquired or run."""

import ast
import hashlib
import importlib.util
import tempfile
from pathlib import Path
import subprocess
import unittest
from unittest.mock import patch
import urllib.request


SCRIPTS = Path(__file__).parent


def load_leaf():
    spec = importlib.util.spec_from_file_location(
        "qoder_acquisition_contract", SCRIPTS / "project_native_collectors" / "qoder.py")
    leaf = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(leaf)
    return leaf


class FakeContext:
    def __init__(self, leaf):
        self.leaf, self.calls, self.replace = leaf, [], {}

    def probe(self, label, arguments, project="a", timeout=30):
        self.calls.append((label, arguments, project, timeout))
        if label in self.replace:
            return self.replace[label]
        if label == "version":
            return {"exit": 0, "error": None}, b"1.1.78\n", b""
        usage = next(row[2] for row in self.leaf.HELP if row[0] == label)
        return {"exit": 0, "error": None}, (usage + "\n\nOptions:\n  -h, --help  display help for command\n").encode(), b""


class QoderAcquisitionContract(unittest.TestCase):
    def skills_context(self):
        leaf = load_leaf()
        ctx = FakeContext(leaf)
        bodies = {
            "top-help": (leaf.HELP[0][2] + "\n\nOptions:\n  -h, --help  Help\n\nCommands:\n"
                         "  skills|skill [options] [command]  Manage Skills\n"),
            "skills-help": ("Usage: qodercli skills|skill [options] [command]\n\nOptions:\n"
                            "  -h, --help  Help\n\nCommands:\n  list [options]  List discovered Skills\n"
                            "  install <source>  Install a Skill\n"),
            "skills-list-help": ("Usage: qodercli skills list [options]\n\nOptions:\n"
                                 "  --json  Output JSON\n  -h, --help  Help\n"),
        }
        ctx.replace.update({label: ({"exit": 0, "error": None}, body.encode(), b"")
                            for label, body in bodies.items()})
        for stage in ("empty", "populated"):
            for project in ("a", "b", "user"):
                ctx.replace["skills-" + stage + "-" + project] = (
                    {"exit": 0, "error": None}, b'{"unreviewed_native_schema":true}\n', b"")
        return leaf, ctx

    def test_skills_native_help_gates_every_catalog_vector_without_ready(self):
        leaf, ctx = self.skills_context()
        result = leaf.collect(ctx)["observations"]
        self.assertEqual(ctx.calls[5:], [
            ("skills-help", ["skills", "--help"], "a", 30),
            ("skills-list-help", ["skills", "list", "--help"], "a", 30),
            *(("skills-empty-" + project, ["skills", "list", "--json"], project, 30)
              for project in ("a", "b", "user")),
        ])
        research = result["skills_research"]
        self.assertEqual(research["status"], "opaque-catalog-captured")
        self.assertEqual(set(research["baseline"]), {"a", "b", "user"})
        self.assertFalse(research["native_skill_discovery_qualified"])
        self.assertFalse(research["public_ready_qualified"])
        self.assertFalse(result["scope_discovery_qualified"])
        self.assertFalse(result["authentication_qualified"])

    def test_skills_prose_alias_prefix_and_duplicate_commands_never_authorize(self):
        for replacement in (
                "Examples:\n  skills --help  Example only\n",
                "Commands:\n  skills-extra [options]  Other command\n",
                "Commands:\n  skills|skill [options]  Manage\n  skills <source>  Install\n",
                "Commands:\n  skills [options]  Manage\n\nCommands:\n  skills [options]  Other\n"):
            with self.subTest(replacement=replacement):
                leaf, ctx = self.skills_context()
                body = ctx.replace["top-help"][1].split(b"\nCommands:")[0]
                ctx.replace["top-help"] = {"exit": 0, "error": None}, body + b"\n" + replacement.encode(), b""
                result = leaf.collect(ctx)["observations"]["skills_research"]
                self.assertEqual(len(ctx.calls), 5)
                self.assertEqual(result["status"], "not-advertised")

    def test_skills_only_native_read_only_list_and_bare_json_option_authorize(self):
        for label, replacement, expected_calls in (
                ("skills-help", b"  list <source>  List selected Skill\n", 6),
                ("skills-help", b"  list [options]  Install and list Skills\n", 6),
                ("skills-help", b"  list [options]  List Skills\n  list <source>  Mutate\n", 6),
                ("skills-list-help", b"  --json-file <path>  Write JSON\n", 7),
                ("skills-list-help", b"  --json <path>  Write JSON\n", 7),
                ("skills-list-help", b"  --json  Output JSON\n  --json <path>  Write JSON\n", 7)):
            with self.subTest(replacement=replacement):
                leaf, ctx = self.skills_context()
                usage = ctx.replace[label][1].splitlines()[0]
                section = b"Commands" if label == "skills-help" else b"Options"
                ctx.replace[label] = ({"exit": 0, "error": None},
                    usage + b"\n\nOptions:\n  -h, --help  Help\n\n" + section + b":\n" + replacement, b"")
                if label == "skills-list-help":
                    ctx.replace[label] = ({"exit": 0, "error": None},
                        usage + b"\n\nOptions:\n  -h, --help  Help\n" + replacement, b"")
                result = leaf.collect(ctx)["observations"]["skills_research"]
                self.assertEqual(len(ctx.calls), expected_calls)
                self.assertNotIn("baseline", result)
                self.assertFalse(result["public_ready_qualified"])

    def test_skills_parent_help_fallback_and_capture_errors_stop_before_next_command(self):
        for label, record, stdout, stderr, calls in (
                ("skills-help", {"exit": 0, "error": None}, b"Usage: qodercli [options] [command] [query...]\n  -h, --help  Help\n", b"", 6),
                ("skills-help", {"exit": 1, "error": None}, b"", b"Auth required\n", 6),
                ("skills-list-help", {"exit": 0, "error": None}, b"Usage: qodercli skills [options]\n  -h, --help  Help\n", b"", 7),
                ("skills-empty-a", {"exit": 0, "error": "overflow"}, b"partial", b"", 8),
                ("skills-empty-a", {"exit": 0, "error": None}, b"[]", b"warning", 8),
                ("skills-empty-a", {"exit": 0, "error": None}, b"[]", bytearray(), 8),
                ("skills-empty-a", {"exit": None, "error": "deadline"}, b"partial", b"", 8)):
            with self.subTest(label=label, stdout=stdout, stderr=stderr):
                leaf, ctx = self.skills_context()
                ctx.replace[label] = record, stdout, stderr
                with self.assertRaises(ValueError):
                    leaf.collect(ctx)
                self.assertEqual(len(ctx.calls), calls)

    def test_inert_skill_fixture_refuses_redirected_parent_or_existing_skill(self):
        for redirected in (False, True):
            with self.subTest(redirected=redirected), tempfile.TemporaryDirectory() as temporary:
                leaf, ctx = self.skills_context()
                root = Path(temporary)
                ctx.home, ctx.identities = root / "home", {}
                ctx.projects = {label: root / label for label in ("a", "b")}
                for base in (ctx.home, *ctx.projects.values()):
                    base.mkdir()
                foreign = root / "foreign"
                foreign.mkdir()
                (foreign / "marker").write_bytes(b"foreign")
                if redirected:
                    (ctx.projects["a"] / ".qoder").symlink_to(foreign, target_is_directory=True)
                    expected_error = ValueError
                else:
                    selected = ctx.projects["a"] / ".qoder/skills/uap-qoder-inert-a"
                    selected.mkdir(parents=True)
                    (selected / "SKILL.md").write_bytes(b"prior")
                    expected_error = FileExistsError
                with self.assertRaises(expected_error):
                    leaf._author_inert_skills(ctx)
                self.assertEqual((foreign / "marker").read_bytes(), b"foreign")
                self.assertEqual([p.name for p in foreign.iterdir()], ["marker"])
                if not redirected:
                    self.assertEqual((selected / "SKILL.md").read_bytes(), b"prior")
                self.assertEqual(ctx.identities, {})

    def test_populated_skills_fixture_preserves_mcp_oracle_and_registers_exact_bytes(self):
        leaf, ctx = self.skills_context()
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            ctx.home = root / "home"
            ctx.home.mkdir()
            ctx.phase, ctx.identities = "observe", {}
            ctx.projects = {label: root / label for label in ("a", "b", "user")}
            for path in ctx.projects.values():
                path.mkdir()
            for label in ("a", "b"):
                (ctx.projects[label] / ".git").mkdir()
            for label in ("a", "b"):
                ctx.replace["mcp-list-" + label] = ({"exit": 0, "error": None},
                    b"uap_qoder_a_stdio\nuap_qoder_a_http\n" if label == "a" else b"No MCP servers configured\n", b"")
            result = leaf.collect(ctx)
            research = result["observations"]["skills_research"]
            self.assertEqual([row["id"] for row in result["cases"]], ["qoder.help", "qoder.scope-observation"])
            self.assertTrue(result["observations"]["scope_discovery_qualified"])
            self.assertFalse(research["native_skill_discovery_qualified"])
            self.assertEqual(len(ctx.identities), 3)
            for row in research["authored_inert_skills"]:
                path = Path(row["path"])
                base = ctx.home if row["project"] == "user" else ctx.projects[row["project"]]
                self.assertEqual(path, base / ".qoder/skills" / row["name"] / "SKILL.md")
                self.assertEqual(hashlib.sha256(path.read_bytes()).hexdigest(), row["sha256"])
                self.assertEqual(ctx.identities[path], row["sha256"])
                self.assertEqual(path.stat().st_mode & 0o777, 0o600)
            self.assertEqual([row[0] for row in ctx.calls[-3:]],
                             ["skills-populated-a", "skills-populated-b", "skills-populated-user"])

    def test_import_and_fake_collection_cannot_launch_or_acquire_a_vendor(self):
        with (
            patch.object(subprocess, "Popen", side_effect=AssertionError("vendor process")) as process,
            patch.object(urllib.request, "urlopen", side_effect=AssertionError("network download")) as download,
        ):
            leaf = load_leaf()
            self.assertEqual(leaf.AVAILABLE_PHASES, ("acquire", "help", "observe"))
            self.assertFalse(getattr(leaf, "ACQUISITION_ONLY", False))
            self.assertEqual(leaf.ACQUISITION["entrypoint"], "qodercli")
            leaf.collect(FakeContext(leaf))
            process.assert_not_called()
            download.assert_not_called()

    def test_only_five_fixed_probes_and_help_case_are_emitted(self):
        leaf = load_leaf()
        ctx = FakeContext(leaf)
        result = leaf.collect(ctx)
        self.assertEqual(ctx.calls, [
            ("version", ["--version"], "a", 30),
            ("top-help", ["--help"], "a", 30),
            ("plugins-help", ["plugins", "--help"], "a", 30),
            ("plugins-list-help", ["plugins", "list", "--help"], "a", 30),
            ("mcp-help", ["mcp", "--help"], "a", 30),
        ])
        self.assertEqual(result["cases"], [{"id": "qoder.help", "status": "passed",
                         "reason": "Pinned fixed version/help table observed; no plugin operation, scope or runtime qualification"}])
        self.assertEqual(result["observations"]["client_version_measured"], "1.1.78")
        self.assertTrue(all(value is False for key, value in result["observations"].items() if key.endswith("_qualified")))

    def test_observe_reuses_collector_and_refuses_cross_project_mcp_names(self):
        leaf = load_leaf()
        for borrowed in (False, True):
            with self.subTest(borrowed=borrowed), tempfile.TemporaryDirectory() as temporary:
                ctx = FakeContext(leaf)
                ctx.phase = "observe"
                ctx.projects = {name: Path(temporary) / name for name in ("a", "b")}
                for project in ctx.projects.values():
                    (project / ".git").mkdir(parents=True)
                # The collector may inspect existing roots but must never
                # repeat the shared harness's git-init-a/b capture labels.
                ctx.replace["mcp-list-a"] = ({"exit": 0, "error": None},
                    b"uap_qoder_a_stdio\nuap_qoder_a_http\n", b"")
                ctx.replace["mcp-list-b"] = ({"exit": 0, "error": None},
                    b"uap_qoder_a_stdio\n" if borrowed else b"No MCP servers configured\n", b"")
                if borrowed:
                    with self.assertRaisesRegex(ValueError, "not isolated"):
                        leaf.collect(ctx)
                else:
                    result = leaf.collect(ctx)
                    self.assertTrue(result["observations"]["scope_discovery_qualified"])
                    self.assertEqual([row["id"] for row in result["cases"]],
                                     ["qoder.help", "qoder.scope-observation"])
                    self.assertFalse(result["observations"]["mcp_tools_call_qualified"])

    def test_process_capture_errors_and_malformed_stream_types_fail_at_first_probe(self):
        for record, stdout, stderr in [
            ({"exit": 1, "error": None}, b"Authentication required", b""),
            ({"exit": 0, "error": "overflow"}, b"1.1.78\n", b""),
            ({"exit": 0, "error": None}, b"1.1.78\n", b"warning\n"),
            ({"exit": 0, "error": None}, b"1.1.78\n", b"\n"),
            ({"exit": None, "error": None}, b"1.1.78\n", b""),
            ({"exit": False, "error": None}, b"1.1.78\n", b""),
            ({"exit": 0}, b"1.1.78\n", b""), ({}, b"1.1.78\n", b""),
            (None, b"1.1.78\n", b""),
            ({"exit": 0, "error": None}, "1.1.78\n", b""),
            ({"exit": 0, "error": None}, b"1.1.78\n", ""),
        ]:
            with self.subTest(record=record, stdout=stdout, stderr=stderr):
                leaf = load_leaf()
                ctx = FakeContext(leaf)
                ctx.replace["version"] = record, stdout, stderr
                with self.assertRaisesRegex(ValueError, "probe did not finish cleanly"):
                    leaf.collect(ctx)
                self.assertEqual(len(ctx.calls), 1)

    def test_wrong_version_or_unknown_help_never_becomes_a_boundary(self):
        for label, stdout, calls in [("version", b"1.1.79\n", 1),
                                     ("top-help", b"Authentication required\n", 2),
                                     ("mcp-help", b"Usage: qodercli mcp-new [options]\n  -h, --help  help\n", 5)]:
            with self.subTest(label=label):
                leaf = load_leaf()
                ctx = FakeContext(leaf)
                ctx.replace[label] = {"exit": 0, "error": None}, stdout, b""
                with self.assertRaises(ValueError):
                    leaf.collect(ctx)
                self.assertEqual(len(ctx.calls), calls)

    def test_exact_retained_version_bytes(self):
        leaf = load_leaf()
        self.assertEqual(leaf.parse_version(b"1.1.78\n"), "1.1.78")
        for body in [b"1.1.78", b"1.1.78\r\n", b"1.1.79\n", b" 1.1.78\n", b"1.1.78\nextra\n"]:
            with self.subTest(body=body), self.assertRaisesRegex(ValueError, "version differs"):
                leaf.parse_version(body)

    def test_command_usage_is_anchored_and_help_is_required(self):
        leaf = load_leaf()
        parent = "Usage: qodercli plugins|plugin [options] [command]"
        listing = "Usage: qodercli plugins list [options]"
        for usage in (parent, listing):
            self.assertEqual(leaf.parse_help((usage + "\n\n  -h, --help  display help for command\n").encode(), usage), usage)
        good = (listing + "\n  -h, --help  display help for command\n").encode()
        for body in [b"prefix\n" + good, good.replace(b"plugins list", b"plugins listings"),
                     good.replace(b"plugins list", b"plugins"), good + (listing + "\n").encode(),
                     good.replace(b"--help", b"--helper"), good.replace(b"--help", b"--hel"),
                     good + b"\x1b[0m", good + b"\x00", good + b"\xff"]:
            with self.subTest(body=body), self.assertRaises(ValueError):
                leaf.parse_help(body, listing)

    def test_measured_main_usage_requires_exact_query_suffix(self):
        leaf = load_leaf()
        usage = "Usage: qodercli [options] [command] [query...]"
        body = (usage + "\n\nOptions:\n  -h, --help  display help for command\n").encode()
        self.assertEqual(leaf.HELP[0][2], usage)
        self.assertEqual(leaf.parse_help(body, usage), usage)
        for wrong in [body.replace(b" [query...]", b""),
                      body.replace(b" [query...]", b" [query...] [extra]"),
                      body.replace(b"[command] [query...]", b"[query...] [command]"),
                      body.replace(b"qodercli ", b"qodercli-other ")]:
            with self.subTest(body=wrong), self.assertRaises(ValueError):
                leaf.parse_help(wrong, usage)

    def test_reuses_preserved_reviewed_archive_and_binary_identities(self):
        # Read literal pins without importing the legacy acquisition machinery.
        tree = ast.parse((SCRIPTS / "research-qoder-native.py").read_text(encoding="utf-8"))
        preserved = {
            node.targets[0].id: ast.literal_eval(node.value)
            for node in tree.body
            if isinstance(node, ast.Assign) and len(node.targets) == 1
            and isinstance(node.targets[0], ast.Name)
            and node.targets[0].id in {"VERSION", "URL", "ARCHIVE_PIN", "BINARY_SHA256"}
        }
        leaf = load_leaf()
        self.assertEqual(leaf.CLIENT, "qoder")
        self.assertEqual(leaf.VERSION, preserved["VERSION"])
        self.assertEqual(leaf.ACQUISITION["url"], preserved["URL"])
        self.assertEqual(leaf.ACQUISITION["integrity"], preserved["ARCHIVE_PIN"])
        self.assertEqual(leaf.ACQUISITION["binary_sha256"], preserved["BINARY_SHA256"])
        self.assertIs(type(leaf.ACQUISITION["expected_bytes"]), int)
        self.assertGreater(leaf.ACQUISITION["expected_bytes"], 0)
        self.assertLess(leaf.ACQUISITION["expected_bytes"], leaf.ACQUISITION["max_bytes"])
        self.assertLessEqual(leaf.ACQUISITION["max_bytes"], 96 << 20)


if __name__ == "__main__":
    unittest.main()
