"""Closed, inert contracts for the separate pinned Devin Project installation proof."""
import json
import unittest
from unittest.mock import patch

import devin_project_auto_install_e2e as subject


class DevinProjectAutoInstallContracts(unittest.TestCase):
    def test_source_and_binary_are_pinned_to_exact_selected_version(self):
        self.assertEqual(subject.VERSION, "3000.12.57")
        self.assertEqual(subject.URL,
                         "https://static.devin.ai/cli/3000.12.57/"
                         "devin-3000.12.57-x86_64-unknown-linux.tar.gz")
        self.assertRegex(subject.ARCHIVE_SHA, r"^[0-9a-f]{64}$")
        self.assertRegex(subject.BINARY_SHA, r"^[0-9a-f]{64}$")
        self.assertNotEqual(subject.ARCHIVE_SHA, subject.BINARY_SHA)

    def test_native_skill_inventory_requires_exact_name_and_closed_shape(self):
        valid = json.dumps([{"name": subject.SKILL}, {"name": "foreign"}])
        self.assertTrue(subject.available_skill(valid, subject.SKILL))
        self.assertFalse(subject.available_skill(valid, subject.SKILL + "-foreign"))
        self.assertTrue(subject.available_skill(json.dumps({"skills": [{"name": subject.SKILL}]}), subject.SKILL))
        for inventory in ('{}', 'null', '"'+subject.SKILL+'"',
                          '{"skills":"'+subject.SKILL+'"}', '{broken'):
            with self.subTest(inventory=inventory):
                with self.assertRaises((RuntimeError, ValueError)):
                    subject.available_skill(inventory, subject.SKILL)

    def test_all_native_commands_use_bounded_supervised_capture(self):
        with patch.object(subject, "run_probe", return_value=(
                {"error": None, "exit": 0}, b"observed\\n", b"")) as probe:
            self.assertEqual(subject.command(["devin", "mcp", "list"], "/tmp", {}, timeout=11), "observed\\n")
            self.assertTrue(probe.call_args.kwargs["memory_only"])
            self.assertEqual(probe.call_args.kwargs["limit"], 1 << 20)
            self.assertEqual(probe.call_args.kwargs["timeout"], 11)
        for error, exit_code in (("deadline", None), (None, 1)):
            with self.subTest(error=error, exit_code=exit_code):
                with patch.object(subject, "run_probe", return_value=(
                        {"error": error, "exit": exit_code}, b"", b"private")):
                    with self.assertRaises(RuntimeError):
                        subject.command(["devin", "--version"], "/tmp", {})

    def test_mcp_inventory_does_not_accept_same_name_prefix_or_suffix(self):
        self.assertTrue(subject.available_mcp(subject.STDIO + "\n", subject.STDIO))
        self.assertTrue(subject.available_mcp(subject.HTTP + " (configured)", subject.HTTP))
        self.assertFalse(subject.available_mcp("pre_" + subject.STDIO, subject.STDIO))
        self.assertFalse(subject.available_mcp(subject.HTTP + "_foreign", subject.HTTP))
        self.assertFalse(subject.available_mcp("no server", subject.HTTP))


if __name__ == "__main__":
    unittest.main()
