"""Closed, inert contracts for the separate pinned Devin Project installation proof."""
import base64
import hashlib
import io
import json
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest.mock import patch

import devin_project_auto_install_e2e as subject


class DevinProjectAutoInstallContracts(unittest.TestCase):
    def test_source_and_binary_are_pinned_to_exact_selected_version(self):
        self.assertEqual(subject.VERSION, "3000.12.57")
        self.assertEqual(subject.PACKAGE_VERSION, "3000.12.57-linux-x64")
        self.assertEqual(subject.METADATA_URL,
                         "https://registry.npmjs.org/devin/3000.12.57-linux-x64")
        self.assertEqual(subject.TARBALL_URL,
                         "https://registry.npmjs.org/devin/-/devin-3000.12.57-linux-x64.tgz")
        self.assertRegex(subject.BINARY_SHA, r"^[0-9a-f]{64}$")

    @staticmethod
    def fixture_package(binary=b"fake-binary", symlink=False, traversal=False):
        manifest = {"name":"devin","version":subject.PACKAGE_VERSION,"os":["linux"],
                    "cpu":["x64"],"repository":{"url":"git+https://github.com/CognitionAI/devin-cli.git"}}
        items = {"package/package.json":json.dumps(manifest).encode(),
                 "package/distribution":b"npm","package/bin/devin":binary}
        archive = io.BytesIO()
        with tarfile.open(fileobj=archive, mode="w:gz") as output:
            for name, contents in items.items():
                member = tarfile.TarInfo(name)
                member.size = len(contents)
                output.addfile(member, io.BytesIO(contents))
            if traversal:
                member = tarfile.TarInfo("package/../escape")
                member.size = 3
                output.addfile(member, io.BytesIO(b"bad"))
            if symlink:
                member = tarfile.TarInfo("package/bad-link")
                member.type, member.linkname = tarfile.SYMTYPE, "../../outside"
                output.addfile(member)
        payload = archive.getvalue()
        metadata = {"name":"devin","version":subject.PACKAGE_VERSION,"os":["linux"],"cpu":["x64"],
                    "dist":{"tarball":subject.TARBALL_URL,
                            "integrity":"sha512-"+base64.b64encode(hashlib.sha512(payload).digest()).decode()}}
        return json.dumps(metadata).encode(), payload

    def test_pinned_npm_package_extracts_and_verifies_original_binary(self):
        metadata, archive = self.fixture_package()
        with tempfile.TemporaryDirectory() as directory, \
             patch.object(subject, "official_npm_bytes", side_effect=[metadata, archive]), \
             patch.object(subject, "BINARY_SHA", hashlib.sha256(b"fake-binary").hexdigest()):
            binary, integrity = subject.install_genuine_devin(Path(directory))
            self.assertEqual(binary.read_bytes(), b"fake-binary")
            self.assertTrue(integrity.startswith("sha512-"))

    def test_npm_platform_rejects_corrupt_tarball_and_unsafe_members(self):
        for label, metadata, archive in (
            ("integrity", *self.fixture_package()),
            ("symlink", *self.fixture_package(symlink=True)),
            ("traversal", *self.fixture_package(traversal=True)),
            ("binary", *self.fixture_package(binary=b"tampered")),
        ):
            with self.subTest(label=label), tempfile.TemporaryDirectory() as directory:
                if label == "integrity":
                    archive += b"tampered"
                with patch.object(subject, "official_npm_bytes", side_effect=[metadata, archive]):
                    expected = {
                        "integrity": "tarball integrity differs",
                        "symlink": "unsafe Devin npm platform member",
                        "traversal": "unsafe Devin npm platform member",
                        "binary": "independently pinned original",
                    }[label]
                    with self.assertRaisesRegex(RuntimeError, expected):
                        subject.install_genuine_devin(Path(directory))

    def test_npm_metadata_is_exact_not_any_platform_or_registry_tarball(self):
        metadata, archive = self.fixture_package()
        for field, value in (("version", "3000.12.58-linux-x64"), ("os", ["darwin"])):
            changed = json.loads(metadata)
            changed[field] = value
            with self.subTest(field=field):
                with patch.object(subject, "official_npm_bytes",
                                  side_effect=[json.dumps(changed).encode(), archive]):
                    with self.assertRaises(RuntimeError):
                        subject.npm_archive()

    def test_native_skill_inventory_requires_exact_source_and_closed_shape(self):
        project = Path("/tmp/project-a")
        def row(**changed):
            result = {"name":subject.SKILL, "description":"Inert Project installation proof",
                      "triggers":["model", "user"], "provider":"Devin",
                      "base_dir":str(project / ".devin" / "skills" / subject.SKILL),
                      "display_name":subject.SKILL, "warnings":[], "errors":[]}
            result.update(changed)
            return result

        original = row()
        self.assertTrue(subject.available_skill(
            json.dumps([original, {"name": "foreign"}]), subject.SKILL, project))
        self.assertTrue(subject.available_skill(
            json.dumps({"skills": [original]}), subject.SKILL, project))
        self.assertTrue(subject.available_skill(
            json.dumps([row(name=f"{project}:{subject.SKILL}")]), subject.SKILL, project))
        self.assertFalse(subject.available_skill("[]", subject.SKILL, project))
        self.assertFalse(subject.available_skill(
            json.dumps([{"name": "foreign"}]), subject.SKILL, project))

        missing = row()
        missing.pop("base_dir")
        for label, bad in (
            ("foreign-provider", [row(provider="Builtin")]),
            ("foreign-source", [row(base_dir="/tmp/foreign/skills/" + subject.SKILL)]),
            ("unrelated-scope", [row(name="/tmp/project-b:" + subject.SKILL)]),
            ("inconsistent-name", [row(name="not-owned")]),
            ("inconsistent-display", [row(display_name="not-owned")]),
            ("warnings", [row(warnings=["not loaded"])]),
            ("errors", [row(errors=["broken"])]),
            ("unknown-schema", [row(extra="unexpected")]),
            ("missing-source", [missing]),
            ("bad-triggers", [row(triggers="user")]),
            ("duplicate-display", [row(), row(name=f"{project}:{subject.SKILL}")]),
            ("nonobject-row", [row(), "foreign"]),
            ("bare-name", [{"name": subject.SKILL}]),
        ):
            with self.subTest(label=label):
                with self.assertRaises(RuntimeError):
                    subject.available_skill(json.dumps(bad), subject.SKILL, project)
        for inventory in ('{}', 'null', '"'+subject.SKILL+'"',
                          '{"skills":"'+subject.SKILL+'"}', '{broken'):
            with self.subTest(inventory=inventory):
                with self.assertRaises((RuntimeError, ValueError)):
                    subject.available_skill(inventory, subject.SKILL, project)

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

    def test_mcp_inventory_requires_structured_selected_enabled_source(self):
        config = Path("/tmp/project-a/.devin/mcp_config.json")
        def row(name=subject.STDIO, source=str(config), enabled=True, transport="stdio"):
            return {"name":name, "enabled":enabled,
                    "disabled_reason":None if enabled else "disabled",
                    "transport":transport, "command":"python3",
                    "args":[], "url":None, "disabled_tools":[], "source":source}
        body = json.dumps([row(), row(name="foreign")])
        self.assertTrue(subject.available_mcp(body, subject.STDIO, config, "stdio"))
        self.assertFalse(subject.available_mcp(body, subject.HTTP, config, "http"))
        for bad in (
            json.dumps([row(source="/tmp/foreign/mcp_config.json")]),
            json.dumps([row(enabled=False)]),
            json.dumps([row(transport="http")]),
            json.dumps([row(), row()]),
            json.dumps({"message":subject.STDIO}),
            "Skipped " + subject.STDIO + ": invalid configuration",
        ):
            with self.subTest(body=bad), self.assertRaises((RuntimeError, ValueError)):
                subject.available_mcp(bad, subject.STDIO, config, "stdio")
        self.assertFalse(subject.available_mcp("[]", subject.STDIO, config, "stdio"))


if __name__ == "__main__":
    unittest.main()
