"""No native executable required: rejection and attribution contracts only."""
import json
import os
import stat
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

from kimi_stable_project_mcp_smoke import (
    FOREIGN_NAME, FOREIGN_SERVER, NAME, read_servers, safe_read, verify, verify_removed,
    LIFECYCLE_CASES, validate_lifecycle_json,
)


class KimiStableProjectMcpSmokeTests(unittest.TestCase):

    def test_lifecycle_json_requires_exact_pass_probes_sources_and_cold_pair(self):
        events = []
        for name in LIFECYCLE_CASES:
            for root in ("original", "cold"):
                events.append({"Test": name, "Action": "output", "Output": "KIMI_GENUINE_PROBE " + json.dumps({"version": "2.5.2", "executable": "/owned/kimi", "config_root": root}) + "\\n"})
                events.append({"Test": name, "Action": "output", "Output": "KIMI_PREPARED_SOURCE " + json.dumps({"source_binding_id": "pin", "tree_digest": "digest", "native_profile_root": root}) + "\\n"})
            events.append({"Test": name, "Action": "pass"})
        with tempfile.TemporaryDirectory() as root:
            report = Path(root).resolve() / "go.json"
            def check(rows):
                report.write_text("\n".join(json.dumps(row) for row in rows))
                return validate_lifecycle_json(report, "/owned/kimi")
            self.assertEqual(set(check(events)), set(LIFECYCLE_CASES))
            for rows in ([], events[:-1], [{**row, "Action": "skip"} if row.get("Action") == "pass" else row for row in events], [row for row in events if "KIMI_PREPARED_SOURCE" not in row.get("Output", "")], [row for row in events if '"config_root": "cold"' not in row.get("Output", "")]):
                with self.assertRaises(ValueError):
                    check(rows)
            wrong = [dict(row) for row in events]
            wrong[0]["Output"] = wrong[0]["Output"].replace("2.5.2", "2.3.2")
            with self.assertRaisesRegex(ValueError, "version"):
                check(wrong)
            for replacement in ('"executable": "/other/kimi"',):
                wrong_path = [dict(row) for row in events]
                wrong_path[0]["Output"] = wrong_path[0]["Output"].replace('"executable": "/owned/kimi"', replacement)
                with self.assertRaisesRegex(ValueError, "selection"):
                    check(wrong_path)
            cold_sources = [i for i, row in enumerate(events) if row["Test"] == LIFECYCLE_CASES[2] and "KIMI_PREPARED_SOURCE" in row.get("Output", "")]
            with self.assertRaisesRegex(ValueError, "both source"):
                check([row for i, row in enumerate(events) if i != cold_sources[-1]])
            wrong_source = [dict(row) for row in events]
            wrong_source[cold_sources[-1]]["Output"] = wrong_source[cold_sources[-1]]["Output"].replace('"digest"', '"changed"')
            with self.assertRaisesRegex(ValueError, "agreement"):
                check(wrong_source)

    def test_lifecycle_json_reassembles_chunked_go_log_events(self):
        # test2json splits long t.Logf lines into Output events before the
        # newline. The split can occur within a marker or quoted JSON string.
        events = []
        for name in LIFECYCLE_CASES:
            roots = ("original", "cold") if name == LIFECYCLE_CASES[2] else ("original",)
            for root in roots:
                for marker, record in (
                    ("KIMI_GENUINE_PROBE ", {"version": "2.5.2", "executable": "/owned/kimi", "config_root": root}),
                    ("KIMI_PREPARED_SOURCE ", {"source_binding_id": "pin", "tree_digest": "digest",
                                              "native_profile_root": root, "source": "x" * 2400}),
                ):
                    output = "    project_kimi_fixture_test.go:163: " + marker + json.dumps(record) + "\\n"
                    fragments = [output[:46]] + [output[i:i + 207] for i in range(46, len(output), 207)]
                    events.extend({"Test": name, "Action": "output", "Output": part} for part in fragments)
            events.append({"Test": name, "Action": "pass"})
        with tempfile.TemporaryDirectory() as root:
            report = Path(root).resolve() / "go.json"
            def check(rows):
                report.write_text("\\n".join(json.dumps(row) for row in rows))
                return validate_lifecycle_json(report, "/owned/kimi")
            cases = check(events)
            self.assertEqual(set(cases), set(LIFECYCLE_CASES))
            self.assertEqual(len(cases[LIFECYCLE_CASES[2]]["sources"]), 2)
            # A missing final newline must not silently discard a source tail.
            truncated = [dict(row) for row in events]
            last_source = max(i for i, row in enumerate(truncated)
                              if row.get("Test") == LIFECYCLE_CASES[2] and row.get("Action") == "output")
            truncated[last_source]["Output"] = truncated[last_source]["Output"][:-1]
            with self.assertRaisesRegex(ValueError, "incomplete"):
                check(truncated)
            malformed = [dict(row) for row in events]
            malformed[0]["Output"] = "KIMI_GENUINE_PROBE {invalid}\\n"
            with self.assertRaisesRegex(ValueError, "malformed"):
                check(malformed)

    def test_native_server_requires_selected_project_owned_command(self):
        with tempfile.TemporaryDirectory() as root:
            selected = Path(root).resolve() / "selected"
            config = selected / ".kimi-code" / "mcp.json"
            config.parent.mkdir(parents=True)
            inside = selected / ".uap" / "bin" / "probe"
            outside = Path(root).resolve() / "other" / "probe"
            inside.parent.mkdir(parents=True)
            outside.parent.mkdir(parents=True)
            inside.write_text("owned, inert")
            outside.write_text("foreign, inert")
            alias = selected / ".uap" / "bin" / "linked-probe"
            alias.symlink_to(outside)
            escaped = selected / ".uap" / ".." / ".." / "other" / "probe"
            for command, accepted in ((inside, True), (outside, False),
                                      (alias, False), (escaped, False)):
                with self.subTest(accepted=accepted):
                    config.write_text(json.dumps({"mcpServers": {NAME: {"transport": "stdio", "command": str(command)}}}))
                    if accepted:
                        self.assertEqual(len(verify(config)), 64)
                    else:
                        with self.assertRaisesRegex(ValueError, "escaped"):
                            verify(config)

    def test_duplicate_member_rejected(self):
        with tempfile.TemporaryDirectory() as root:
            config = Path(root).resolve() / "mcp.json"
            config.write_text('{"mcpServers":{"same":{},"same":{}}}')
            with self.assertRaisesRegex(ValueError, "duplicate"):
                read_servers(config)

    def test_alias_and_nonregular_file_rejected(self):
        with tempfile.TemporaryDirectory() as root:
            path = Path(root).resolve() / "mcp.json"
            real = Path(root).resolve() / "real.json"
            real.write_text('{"mcpServers":{}}')
            path.symlink_to(real)
            with self.assertRaisesRegex(ValueError, "alias"):
                safe_read(path)
            path.unlink()
            path.mkdir()
            with self.assertRaisesRegex(ValueError, "type"):
                safe_read(path)

    def test_unknown_server_or_transport_refused(self):
        with tempfile.TemporaryDirectory() as root:
            selected = Path(root).resolve() / "selected"
            config = selected / ".kimi-code" / "mcp.json"
            config.parent.mkdir(parents=True)
            for contents in ({"mcpServers": {"unowned": {}}},
                             {"mcpServers": {NAME: {"transport": "http", "url": "http://127.0.0.1:9"}}}):
                with self.subTest(contents=contents), self.assertRaises(ValueError):
                    config.write_text(json.dumps(contents))
                    verify(config)

    def test_native_foreign_mcp_preservation(self):
        with tempfile.TemporaryDirectory() as root:
            selected = Path(root).resolve() / "selected"
            config = selected / ".kimi-code" / "mcp.json"
            launcher = selected / ".uap" / "bin" / "probe"
            config.parent.mkdir(parents=True)
            launcher.parent.mkdir(parents=True)
            launcher.write_text("inert")
            owned = {"transport": "stdio", "command": str(launcher)}
            config.write_text(json.dumps({"mcpServers": {NAME: owned, FOREIGN_NAME: FOREIGN_SERVER}}))
            self.assertEqual(len(verify(config, foreign=FOREIGN_SERVER)), 64)
            config.write_text(json.dumps({"mcpServers": {NAME: owned, FOREIGN_NAME: {"transport": "http", "url": "https://changed.invalid/mcp"}}}))
            with self.assertRaisesRegex(ValueError, "foreign MCP entry changed"):
                verify(config, foreign=FOREIGN_SERVER)
            config.write_text(json.dumps({"mcpServers": {FOREIGN_NAME: FOREIGN_SERVER}}))
            verify_removed(config, foreign=FOREIGN_SERVER)
            config.unlink()
            with self.assertRaisesRegex(ValueError, "lost the foreign"):
                verify_removed(config, foreign=FOREIGN_SERVER)
            config.write_text(json.dumps({"mcpServers": {FOREIGN_NAME: {"transport": "http", "url": "https://changed.invalid/mcp"}}}))
            with self.assertRaisesRegex(ValueError, "changed the foreign"):
                verify_removed(config, foreign=FOREIGN_SERVER)

    def test_removal_refuses_replaced_or_owned_native_config(self):
        with tempfile.TemporaryDirectory() as root:
            path = Path(root).resolve() / "mcp.json"
            verify_removed(path)  # Removed config is valid.
            path.write_text(json.dumps({"mcpServers": {"foreign": {"transport": "stdio"}}}))
            verify_removed(path)  # A foreign-only config is preserved.
            path.write_text(json.dumps({"mcpServers": {NAME: {"transport": "stdio"}}}))
            with self.assertRaisesRegex(ValueError, "persisted"):
                verify_removed(path)
            path.unlink()
            path.symlink_to(Path(root).resolve() / "missing.json")
            with self.assertRaisesRegex(ValueError, "unexpected"):
                verify_removed(path)
            path.unlink()
            path.mkdir()
            with self.assertRaisesRegex(ValueError, "unexpected"):
                verify_removed(path)


    def test_read_race_to_fifo_is_nonblocking_and_refused(self):
        # Swapping a checked regular file to a FIFO before os.open must
        # never block waiting for a writer or count as valid MCP JSON.
        if not all(hasattr(os, name) for name in ("mkfifo", "O_NOFOLLOW", "O_NONBLOCK")):
            self.skipTest("POSIX no-follow, nonblocking FIFO required")
        with tempfile.TemporaryDirectory() as root:
            path = Path(root).resolve() / "mcp.json"
            path.write_text('{"mcpServers":{}}')
            real_open = os.open

            def replace_with_fifo(name, flags, *args, **kwargs):
                self.assertTrue(flags & os.O_NONBLOCK)
                path.unlink()
                os.mkfifo(path)
                return real_open(name, flags, *args, **kwargs)

            with patch("kimi_stable_project_mcp_smoke.os.open", side_effect=replace_with_fifo):
                with self.assertRaisesRegex(ValueError, "identity changed before read"):
                    safe_read(path)
            self.assertTrue(stat.S_ISFIFO(path.lstat().st_mode))


if __name__ == "__main__":
    unittest.main()
