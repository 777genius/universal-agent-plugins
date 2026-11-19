"""Closed, inert contracts for the separate pinned Devin Project installation proof."""
import base64
import hashlib
import io
import json
from pathlib import Path
import shutil
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

    def test_raw_project_skill_requires_exact_provider_source_and_metadata(self):
        selected = Path("/tmp/project-a/.devin/skills") / subject.RAW_SKILL
        row = {"name": subject.RAW_SKILL, "description": subject.RAW_DESCRIPTION,
               "provider": "Devin", "base_dir": str(selected), "display_name": subject.RAW_SKILL,
               "triggers": ["user", "model"], "warnings": [], "errors": []}
        self.assertTrue(subject.raw_skill_present(json.dumps([row]), selected))
        self.assertFalse(subject.raw_skill_present("[]", selected))
        mutations = {"provider": "Windsurf", "base_dir": "/tmp/project-b/.devin/skills/" + subject.RAW_SKILL,
                     "description": "different", "display_name": "different", "triggers": ["user"],
                     "warnings": ["unknown policy"], "errors": ["failed"], "extra": True}
        for key, value in mutations.items():
            with self.subTest(field=key), self.assertRaises(RuntimeError):
                subject.raw_skill_present(json.dumps([{**row, key: value}]), selected)
        for body in (json.dumps([row, row]), json.dumps([{}] * 129), "{}", "null", '["name-only"]'):
            with self.subTest(body=body), self.assertRaises(RuntimeError):
                subject.raw_skill_present(body, selected)

    def test_raw_ready_output_requires_ready_unchanged_repeat(self):
        output = {"schema_version": 1, "command": "skills add", "result": "success",
                  "status": "succeeded", "readiness": "ready", "next_actions": [],
                  "items": [{"alias": subject.RAW_SKILL, "targetId": "windsurf", "status": "unchanged"}]}
        self.assertEqual(subject.raw_ready_output(json.dumps(output), repeated=True), output)
        for key, value in (("schema_version", 2), ("command", "add"), ("result", "failure"),
                           ("status", "partial"), ("readiness", "needs_action"),
                           ("next_actions", ["confirm setup"]), ("items", [])):
            with self.subTest(field=key), self.assertRaises(RuntimeError):
                subject.raw_ready_output(json.dumps({**output, key: value}), repeated=True)
        output["items"][0]["status"] = "succeeded"
        with self.assertRaises(RuntimeError):
            subject.raw_ready_output(json.dumps(output), repeated=True)
        self.assertEqual(subject.raw_ready_output(json.dumps(output)), output)

    def test_raw_case_repeats_same_argv_and_retains_checkable_evidence(self):
        skill = (Path(__file__).resolve().parents[1] /
                 "tests/fixtures/uap_apm/skills/benchmark-note/SKILL.md").read_bytes()
        for fault in (None, "identity", "binding", "payload", "lock", "foreign", "scope"):
            with self.subTest(fault=fault), tempfile.TemporaryDirectory() as directory:
                root = Path(directory).resolve()
                case = root / "raw-project-skills"
                project, outside = case / "project-a", case / "outside"
                state_root = project / ".uap"
                managed = state_root / "managed/clients/windsurf/fixture"
                native = project / ".devin/skills" / subject.RAW_SKILL
                state_path = state_root / "state-v2.json"
                binding = {"client_id": "windsurf", "scope": "project", "activation": "active",
                           "authentication": "not_required", "policy": "allowed", "verification": "installation_verified",
                           "native_profile_root": str(project / ".devin"), "target_locator": str(managed)}
                installation = {"installation_id": "held-fixture-id", "clients": {"held": binding},
                                "source": {"resolved_revision": "b5d80336b53397be5bbb72c84f680e5885a6ac0d"}}
                state = {"schema_version": 5, "installations": [installation]}
                additions = 0
                def fake_command(argv, cwd, env, timeout=90):
                    nonlocal additions
                    if argv[1:3] == ["skills", "add"]:
                        self.assertEqual((Path(cwd), timeout), (outside, 90))
                        additions += 1
                        if additions == 1:
                            for path in (managed, native):
                                path.mkdir(parents=True)
                                (path / "SKILL.md").write_bytes(skill)
                            for name in ("uap.toml", "uap.lock"):
                                (project / name).write_text("held lock or manifest\n")
                            state_path.write_text(json.dumps(state))
                        elif fault in ("identity", "binding"):
                            if fault == "identity": installation["installation_id"] = "replacement-id"
                            else: binding["activation"] = "prepared"
                            state_path.write_text(json.dumps(state))
                        elif fault == "payload": (native / "SKILL.md").write_bytes(b"changed\n")
                        elif fault == "lock": (project / "uap.lock").write_text("changed\n")
                        elif fault == "foreign": (case / "config/devin/mcp_config.json").write_text("changed\n")
                        return json.dumps({"schema_version": 1, "command": "skills add", "result": "success",
                            "status": "succeeded", "readiness": "ready", "items": [{"alias": subject.RAW_SKILL,
                            "targetId": "windsurf", "status": "succeeded" if additions == 1 else "unchanged"}]})
                    if argv[1] == "remove":
                        shutil.rmtree(managed)
                        shutil.rmtree(native)
                        installation["clients"] = {}
                        state_path.write_text(json.dumps(state))
                        return json.dumps({"result": "success"})
                    self.assertEqual((argv[1:], timeout), (["skills", "list", "--json"], 45))
                    present = native.exists() and (Path(cwd) == project or (fault == "scope" and additions == 2))
                    return json.dumps([{"name": subject.RAW_SKILL, "description": subject.RAW_DESCRIPTION,
                        "provider": "Devin", "base_dir": str(native), "display_name": subject.RAW_SKILL,
                        "triggers": ["user", "model"], "warnings": [], "errors": []}] if present else [])
                report = {}
                with patch.object(subject, "command", side_effect=fake_command):
                    if fault:
                        with self.assertRaises(RuntimeError):
                            subject.raw_project_skills(root, report, Path("/selected/devin"), Path("/selected/uap"), {})
                        self.assertNotEqual(report.get("raw_project_skills_native_ready"), True)
                        continue
                    subject.raw_project_skills(root, report, Path("/selected/devin"), Path("/selected/uap"), {})
                evidence = report["raw_project_skills"]
                self.assertTrue(report["raw_project_skills_native_ready"])
                self.assertEqual(set(evidence["public"]), {"add", "repeat", "remove"})
                self.assertEqual(evidence["public"]["repeat"]["items"][0]["status"], "unchanged")
                commands = [row for row in evidence["commands"] if row["stage"] in ("skills-add", "skills-repeat")]
                self.assertEqual(commands[0]["argv"], commands[1]["argv"])
                self.assertEqual(evidence["repeat_continuity"]["before"], evidence["repeat_continuity"]["after"])
                self.assertEqual(len(evidence["inventories"]), 12)
                self.assertEqual({row["stage"] for row in evidence["inventories"]},
                                 {"before-add", "after-add", "after-repeat", "after-remove"})

    def test_raw_case_is_required_without_replacing_original_mixed_oracle(self):
        for mixed, raw in ((False, False), (True, False), (False, True), (True, True)):
            with self.subTest(mixed=mixed, raw=raw):
                self.assertEqual(subject.qualified({"installation_discovery_e2e": mixed,
                    "raw_project_skills_native_ready": raw}), mixed and raw)
        self.assertFalse(subject.qualified({"installation_discovery_e2e": True}))
        self.assertEqual(subject.RAW_SOURCE,
            "https://github.com/777genius/universal-agent-plugins/tree/b5d80336b53397be5bbb72c84f680e5885a6ac0d/tests/fixtures/uap_apm/skills")
        fixture = Path(__file__).resolve().parents[1] / "tests/fixtures/uap_apm/skills/benchmark-note/SKILL.md"
        self.assertEqual(hashlib.sha256(fixture.read_bytes()).hexdigest(), subject.RAW_SKILL_SHA)

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


class RawSnapshotProjectLayoutTests(unittest.TestCase):
    def test_snapshot_uses_real_project_dependency_root_without_legacy_fallback(self):
        with tempfile.TemporaryDirectory() as temporary:
            project = Path(temporary).resolve() / "project"
            state = project / ".uap"
            managed, native = state / "managed", project / ".devin/skills/owned"
            for directory in (managed, native):
                directory.mkdir(parents=True)
            (state / "state-v2.json").write_text("state\n")
            for name in ("uap.toml", "uap.lock"):
                (project / name).write_text("held dependency file\n")
            value = subject.raw_repeat_snapshot(managed, native, project)
            self.assertTrue(all(str(path) in value for path in
                            (state / "state-v2.json", project / "uap.toml", project / "uap.lock")))
            (project / "uap.lock").unlink()
            (state / "uap.lock").write_text("wrong legacy location\n")
            with self.assertRaises(FileNotFoundError):
                subject.raw_repeat_snapshot(managed, native, project)


if __name__ == "__main__":
    unittest.main()
