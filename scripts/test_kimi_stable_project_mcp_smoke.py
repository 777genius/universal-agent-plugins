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
)


class KimiStableProjectMcpSmokeTests(unittest.TestCase):
    def test_native_server_requires_selected_project_owned_command(self):
        with tempfile.TemporaryDirectory() as root:
            selected = Path(root) / "selected"
            config = selected / ".kimi-code" / "mcp.json"
            config.parent.mkdir(parents=True)
            inside = selected / ".uap" / "bin" / "probe"
            outside = Path(root) / "other" / "probe"
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
            config = Path(root) / "mcp.json"
            config.write_text('{"mcpServers":{"same":{},"same":{}}}')
            with self.assertRaisesRegex(ValueError, "duplicate"):
                read_servers(config)

    def test_alias_and_nonregular_file_rejected(self):
        with tempfile.TemporaryDirectory() as root:
            path = Path(root) / "mcp.json"
            real = Path(root) / "real.json"
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
            selected = Path(root) / "selected"
            config = selected / ".kimi-code" / "mcp.json"
            config.parent.mkdir(parents=True)
            for contents in ({"mcpServers": {"unowned": {}}},
                             {"mcpServers": {NAME: {"transport": "http", "url": "http://127.0.0.1:9"}}}):
                with self.subTest(contents=contents), self.assertRaises(ValueError):
                    config.write_text(json.dumps(contents))
                    verify(config)

    def test_native_foreign_mcp_preservation(self):
        with tempfile.TemporaryDirectory() as root:
            selected = Path(root) / "selected"
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
            path = Path(root) / "mcp.json"
            verify_removed(path)  # Removed config is valid.
            path.write_text(json.dumps({"mcpServers": {"foreign": {"transport": "stdio"}}}))
            verify_removed(path)  # A foreign-only config is preserved.
            path.write_text(json.dumps({"mcpServers": {NAME: {"transport": "stdio"}}}))
            with self.assertRaisesRegex(ValueError, "persisted"):
                verify_removed(path)
            path.unlink()
            path.symlink_to(Path(root) / "missing.json")
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
            path = Path(root) / "mcp.json"
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
