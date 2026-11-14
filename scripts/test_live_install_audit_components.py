"""Regression proof for released-CLI native component attribution and negative outcomes."""
import copy
import importlib.util
import json
from pathlib import Path
from tempfile import TemporaryDirectory
import unittest
from unittest.mock import patch


def load_audit():
    file = Path(__file__).resolve().parent / "live-install-audit.py"
    spec = importlib.util.spec_from_file_location("live_install_audit", file)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class PublishedCatalogNativeProofTest(unittest.TestCase):
    def test_native_component_identity_must_match_real_changed_client_file(self):
        audit = load_audit()
        parsed = {"data": {"actions": [{"action": "install", "component_support": [
            {"kind": "skill", "name": "gcp-playbooks", "support": "native"},
            {"kind": "extension", "name": "external-extension", "support": "unsupported"},
            {"kind": "mcp_server", "name": "no-credentials", "support": "prepared"},
        ]}]}}
        names = audit.native_components_from_add(parsed)
        self.assertEqual(names, ("gcp-playbooks",))
        with TemporaryDirectory() as tmp:
            root = Path(tmp) / ".gemini"
            skill = root / "skills/gcp-playbooks/SKILL.md"
            skill.parent.mkdir(parents=True)
            skill.write_text("---\nname: gcp-playbooks\n---\n")
            other = root / "foreign-neighbor.txt"
            other.write_text("unrelated content")
            changes = {
                (".gemini/skills/gcp-playbooks/SKILL.md", "sha256:owned"),
                (".gemini/foreign-neighbor.txt", "sha256:foreign"),
            }
            observed = audit.attributable_native_changes(
                [root], changes, "sre-extension", "gemini", names)
            self.assertEqual(observed, [".gemini/skills/gcp-playbooks/SKILL.md"])
            self.assertEqual(audit.attributable_native_changes(
                [root], changes, "sre-extension", "gemini"), [])
            self.assertEqual(audit.native_identity_residuals(
                [root], "sre-extension", [c[0] for c in changes],
                "gemini", names), observed)
            skill.unlink()
            self.assertEqual(audit.native_identity_residuals(
                [root], "sre-extension", [c[0] for c in changes],
                "gemini", names), [])

    def test_mcp_server_native_registration_under_declared_component_id(self):
        audit = load_audit()
        with TemporaryDirectory() as tmp:
            root = Path(tmp) / ".cline"
            settings = root / "data/settings/cline_mcp_settings.json"
            settings.parent.mkdir(parents=True)
            settings.write_text('{"mcpServers":{"dockerhub":{"command":"node"}}}')
            changed = {(".cline/data/settings/cline_mcp_settings.json", "owned")}
            self.assertEqual(audit.attributable_native_changes(
                [root], changed, "docker-hub", "cline", ("dockerhub",)),
                [".cline/data/settings/cline_mcp_settings.json"])
            self.assertEqual(audit.attributable_native_changes(
                [root], changed, "docker-hub", "cline"), [])
            self.assertEqual(audit.native_identity_residuals(
                [root], "docker-hub", [".cline/data/settings/cline_mcp_settings.json"],
                "cline", ("dockerhub",)),
                [".cline/data/settings/cline_mcp_settings.json"])

    def test_prepared_support_is_never_mislabeled_native(self):
        audit = load_audit()
        prepared = {"data": {"actions": [{"component_support": [
            {"kind": "mcp_server", "name": "dockerhub", "support": "prepared"}]}]}}
        self.assertTrue(audit.prepared_only_add(prepared))
        self.assertEqual(audit.native_components_from_add(prepared), ())
        for corrupt in (None, {"data": {"actions": [{"component_support": None}]}},
                        {"data": {"actions": [{"component_support": [
                            {"kind": "mcp_server", "name": "dockerhub", "support": "native"}]}]}}):
            self.assertFalse(audit.prepared_only_add(corrupt))

    def test_expected_policy_refusal_is_negative_proof_not_native_pass(self):
        audit = load_audit()
        row = {"status": "documented-or-source-unsupported", "kind": "plugin",
               "target": "cline", "source": "marketing-kit", "native_file_delta": 0,
               "native_identity_files": [], "listed_identity": False,
               "failed_install_registered": False, "failed_install_residue": False,
               "sentinels_preserved": True,
               "install": {"exit_code": 1, "stderr_tail":
                           "release does not support complete target set; missing cline"}}
        self.assertTrue(audit.expected_negative_coverage(row))
        for key, value in (("native_file_delta", 1), ("failed_install_residue", True),
                           ("sentinels_preserved", False), ("target", "gemini"),
                           ("status", "passed")):
            self.assertFalse(audit.expected_negative_coverage({**row, key: value}))

    def test_prepared_negative_requires_full_lifecycle_and_never_native_pass(self):
        audit = load_audit()
        case = {"status": "prepared-lifecycle-passed", "kind": "plugin",
                "target": "opencode", "source": "docker-hub",
                "prepared_components_only": True, "native_component_candidates": [],
                "native_identity_files": [], "native_residuals": [],
                "listed_identity": True, "repeat_verified": True,
                "removal_verified": True, "sentinels_preserved": True,
                "post_remove_identity_registered": False,
                "failed_install_residue": False,
                "install": {"exit_code": 0}, "cleanup": {"exit_code": 0}}
        self.assertTrue(audit.expected_negative_coverage(case))
        # A fully verified prepared lifecycle is a valid *non-native* result
        # regardless of source, including a harmless OpenCode config write.
        self.assertTrue(audit.expected_negative_coverage({
            **case, "source": "atlassian",
            "native_identity_files": ["opencode/opencode.json"]}))
        self.assertTrue(audit.expected_negative_coverage({
            **case, "source": "cloudflare",
            "native_identity_files": ["opencode/opencode.json",
                                      "opencode/skills/cloudflare/SKILL.md",
                                      "opencode/skills/cloudflare/references/readme.md"]}))
        for key, value in (("status", "passed"), ("target", "claude"),
                           ("native_identity_files", ["native.json"]),
                           ("post_remove_identity_registered", True),
                           ("native_component_candidates", ["dockerhub"]),
                           ("listed_identity", False),
                           ("native_residuals", ["opencode/active.json"]),
                           ("install", {"exit_code": 1}),
                           ("cleanup", {"exit_code": 1}),
                           ("repeat_verified", False), ("removal_verified", False),
                           ("prepared_components_only", False)):
            self.assertFalse(audit.expected_negative_coverage({**case, key: value}))


    def test_known_unqualified_opencode_sources_are_negative_not_native(self):
        audit = load_audit()
        sources = (
            "github:itential/builder-skills@2ccdf77358b018ad4df125422a067fe84e63de1d",
            "github:GoogleCloudPlatform/google-cloud-storage-plugin@b346e39698fcdfdc711ca3ee35e41c8dbaa1aa2b",
            "github:cometchat/cometchat-skills@8652f1ea41d12e5058123db1e4e6babef9c918f1",
            "github:uizze/uizze@fc401e9e654cbafe427b0c98142fc8bccd0c4e7e",
        )
        row = {"status": "package-installed-native-unqualified",
               "kind": "plugin", "target": "opencode",
               "source": sources[0],
               "prepared_components_only": False, "native_component_candidates": [],
               "native_identity_files": [], "listed_identity": True,
               "repeat_verified": True, "removal_verified": True,
               "post_remove_identity_registered": False,
               "native_residuals": [], "sentinels_preserved": True,
               "failed_install_residue": False,
               "install": {"exit_code": 0}, "cleanup": {"exit_code": 0}}
        # Only four exact frozen sources with independently observed clean
        # package lifecycles can qualify as non-native negative coverage.
        for source in sources:
            for version in ("agentplugins 0.1.95", "agentplugins 0.1.101",
                            "agentplugins 0.1.102"):
                with self.subTest(source=source, version=version):
                    self.assertTrue(audit.expected_negative_coverage(
                        {**row, "source": source}, version))
        for version in (None, "agentplugins 0.1.103", "agentplugins 0.2.0"):
            self.assertFalse(audit.expected_negative_coverage(row, version))
        for key, bad in (
            ("source", "github:unknown/plugin@" + "a" * 40),
            ("status", "passed"),
            ("target", "claude"),
            ("native_identity_files", ["opencode/opencode.json"]),
            ("native_component_candidates", ["builder-agent"]),
            ("listed_identity", False),
            ("repeat_verified", False),
            ("removal_verified", False),
            ("native_residuals", ["opencode/registry.json"]),
            ("post_remove_identity_registered", True),
            ("sentinels_preserved", False),
            ("failed_install_residue", True),
            ("prepared_components_only", True),
            ("install", {"exit_code": 1}),
            ("cleanup", {"exit_code": 1}),
        ):
            with self.subTest(key=key):
                self.assertFalse(audit.expected_negative_coverage(
                    {**row, key: bad}, "agentplugins 0.1.102"))

    def test_pi_explicit_unsupported_refusals_never_become_native_pass(self):
        audit = load_audit()
        row = {"kind": "plugin", "target": "pi", "source": "atlassian",
               "status": "documented-or-source-unsupported",
               "native_file_delta": 0, "native_identity_files": [],
               "listed_identity": False, "failed_install_registered": False,
               "failed_install_residue": False, "sentinels_preserved": True,
               "install": {"exit_code": 1, "stderr_tail":
                           "release does not support complete target set; missing pi"}}
        self.assertTrue(audit.expected_negative_coverage(row))
        self.assertTrue(audit.expected_negative_coverage({
            **row, "install": {"exit_code": 1, "stderr_tail":
                              "pi package manifest overrides selected extension discovery"}}))
        for key, bad in (("target", "codex"), ("status", "passed"),
                         ("native_file_delta", 1), ("listed_identity", True),
                         ("failed_install_registered", True),
                         ("native_identity_files", ["pi/active.json"]),
                         ("sentinels_preserved", False),
                         ("install", {"exit_code": 0, "stderr_tail": "missing pi"}),
                         ("install", {"exit_code": 1, "stderr_tail": "network timeout"})):
            with self.subTest(key=key, bad=bad):
                self.assertFalse(audit.expected_negative_coverage({**row, key: bad}))

    def test_grok_vendor_user_guide_is_not_an_active_plugin_registration(self):
        audit = load_audit()
        with TemporaryDirectory() as tmp:
            root = Path(tmp) / ".grok"
            docs = root / "docs/user-guide"
            docs.mkdir(parents=True)
            ignored = ["05-configuration.md", "16-subagents.md"]
            for name in ignored:
                (docs / name).write_text("name: documentation\\n", encoding="utf-8")
            active = root / "plugins/documentation/plugin.json"
            active.parent.mkdir(parents=True)
            active.write_text('{"name":"documentation"}')
            observed = [".grok/docs/user-guide/" + name for name in ignored]
            observed.append(".grok/plugins/documentation/plugin.json")
            self.assertEqual(audit.native_identity_residuals(
                [root], "documentation", observed, "grok"),
                [".grok/plugins/documentation/plugin.json"])
            self.assertFalse(audit.passive_native_path(
                "grok", "docs/user-guide/custom-plugin.json"))
            self.assertFalse(audit.passive_native_path(
                "grok", "plugins/documentation/plugin.json"))


class KiroPreparedLifecycleTest(unittest.TestCase):
    def fixture(self, audit):
        pins = json.loads(audit.FIXTURES.read_text())["signed_release_pins"]
        pin = pins["atlassian"]
        client = {"client_id": "kiro", "scope": "user", "install_intent": "prepare",
                  "materialization": "materialized", "activation": "prepared",
                  "authentication": "not_checked", "policy": "allowed",
                  "verification": "package_validated", "package_revision": {
                      "resolved_revision": pin["revision"], "distribution_id": pin["distribution"],
                      "release_sequence": pin["sequence"], "tree_digest": pin["tree_digest"],
                      "manifest_digest": pin["manifest_digest"]}}
        node = {"installation_id": "owned-original-id", "name": "atlassian",
                "source": pin["repository"].strip("/") + "/" + pin["path"].strip("/"),
                "clients": [client], "directory": {"product_id": "atlassian",
                    "recorded_distribution": pin["distribution"], "recorded_revision": pin["revision"],
                    "recorded_release_sequence": pin["sequence"]}}
        return node, pins

    def test_prepared_match_requires_exact_user_tuple_and_original_source(self):
        audit = load_audit()
        node, pins = self.fixture(audit)
        original = copy.deepcopy(node)
        def matches(value, target="kiro"):
            return audit.matching_installations({"data": {"installations": [value]}},
                                               "atlassian", target, "plugin", "atlassian", pins)
        self.assertEqual(matches(node), [node])
        self.assertEqual(node, original)
        for field, value in (("scope", "project"), ("install_intent", "automatic"),
                             ("materialization", "absent"), ("activation", "active"),
                             ("authentication", "authenticated"), ("policy", "blocked"),
                             ("verification", None), ("client_id", "grok")):
            with self.subTest(field=field):
                changed = copy.deepcopy(node)
                changed["clients"][0][field] = value
                self.assertEqual(matches(changed), [])
        for fault in ("duplicate", "source", "revision", "tree", "sequence"):
            changed = copy.deepcopy(node)
            if fault == "duplicate":
                changed["clients"].append(copy.deepcopy(changed["clients"][0]))
            elif fault == "source":
                changed["source"] = "unrelated/repository/entry"
            else:
                key = {"revision": "resolved_revision", "tree": "tree_digest", "sequence": "release_sequence"}[fault]
                changed["clients"][0]["package_revision"][key] = "wrong"
            self.assertEqual(matches(changed), [], fault)
        changed = copy.deepcopy(node)
        changed["clients"][0]["client_id"] = "grok"
        self.assertEqual(matches(changed, "grok"), [])

    def test_actual_case_continues_six_public_calls_without_native_credit(self):
        for fault in (None, "source", "duplicate", "repeat-active", "repeat-revision",
                      "repeat-native", "foreign", "remove"):
            with self.subTest(fault=fault), TemporaryDirectory() as directory:
                audit = load_audit()
                node, pins = self.fixture(audit)
                if fault == "source":
                    node["source"] = "unrelated/repository/entry"
                if fault == "duplicate":
                    node["clients"].append(copy.deepcopy(node["clients"][0]))
                calls, removed = [], False
                def call(argv, env, cwd, timeout, **kwargs):
                    nonlocal removed
                    calls.append((argv, timeout))
                    role = argv[1]
                    owned = Path(env["HOME"]) / ".kiro/skills/fixture/SKILL.md"
                    outcome = {"exit_code": 0, "stdout_tail": "", "stderr_tail": "", "timeout": False}
                    if role == "add":
                        outcome["_json"] = {"data": {"actions": [{"component_support": [
                            {"kind": "skill", "name": "fixture", "support": "native"}]}]}}
                        if len(calls) == 1:
                            owned.parent.mkdir(parents=True)
                            owned.write_text("---\nname: fixture\n---\nowned bytes\n")
                            data = Path(env["AGENTPLUGINS_HOME"])
                            (data / "uap.toml").write_text("original agreement\n")
                            (data / "uap.lock").write_text("original pin\n")
                        elif fault == "repeat-active":
                            node["clients"][0].update(activation="active", verification="installation_verified")
                        elif fault == "repeat-revision":
                            node["clients"][0]["package_revision"]["tree_digest"] = "sha256:" + "0" * 64
                        elif fault == "repeat-native":
                            owned.write_text("changed during repeat\n")
                        elif fault == "foreign":
                            (Path(env["HOME"]) / "foreign-untouched.txt").write_text("changed foreign\n")
                    elif role == "list":
                        outcome["_json"] = {"schema_version": 1, "command": "list", "result": "success",
                                            "data": {"installations": [] if removed else [copy.deepcopy(node)]}}
                    elif role == "remove":
                        self.assertEqual(argv[2], "owned-original-id")
                        if fault == "remove":
                            outcome.update(exit_code=1, stderr_tail="refused removal")
                        else:
                            owned.unlink()
                            removed = True
                    else:
                        self.fail("unexpected public/native command: " + repr(argv))
                    return outcome
                with patch.object(audit, "call", side_effect=call):
                    result = audit.do_case("inert-uap", "kiro", "plugin", "atlassian", 0,
                                           Path(directory) / "run", {}, pins)
                self.assertLessEqual(len(calls), 6)
                self.assertIsNone(result["native_uninstall"])
                self.assertFalse(audit.expected_negative_coverage(result))
                self.assertNotEqual(result["status"], "passed")
                self.assertFalse(result["prepared_components_only"])
                if fault is None:
                    self.assertEqual([argv[1] for argv, _ in calls], ["add", "list", "add", "list", "remove", "list"])
                    self.assertEqual(result["status"], "incomplete-native-proof")
                    for field in ("repeat_verified", "removal_verified", "sentinels_preserved", "kiro_prepared_lifecycle_verified"):
                        self.assertIs(result[field], True)
                        self.assertIs(audit.case_observation(result)[field], True)
                else:
                    self.assertIs(result["kiro_prepared_lifecycle_verified"], False)
                    if fault in ("source", "duplicate"):
                        self.assertEqual(len(calls), 2)
                        self.assertIsNone(result["repeat_add"])
                        self.assertIsNone(result["cleanup"])
                    elif fault.startswith("repeat-"):
                        self.assertEqual(result["status"], "repeat-add-unqualified")
                    else:
                        self.assertEqual(result["status"], "cleanup-failed")


    def test_console_observation_keeps_types_and_has_no_native_authority(self):
        audit = load_audit()
        flags = ("kiro_prepared_lifecycle_verified", "repeat_verified", "removal_verified",
                 "sentinels_preserved", "failed_install_registered", "failed_install_residue")
        row = dict.fromkeys(flags, True)
        row.update(status="incomplete-native-proof", install={"exit_code": 1, "timeout": False,
                   "stderr_tail": "private command text"}, list={"exit_code": 0, "timeout": False})
        original = copy.deepcopy(row)
        observed = audit.case_observation(row)
        self.assertEqual(row, original)
        self.assertEqual(set(observed), set(flags) | {"install", "list"})
        self.assertTrue(all(observed[name] is True for name in flags))
        self.assertEqual(observed["install"], {"exit_code": 1, "timeout": False})
        self.assertEqual(observed["list"], {"exit_code": 0, "timeout": False})
        self.assertNotIn("private command text", json.dumps(observed))
        self.assertFalse(audit.expected_negative_coverage(row))
        for invalid in (None, True, 1, [], "private command text"):
            with self.subTest(row=invalid):
                observed = audit.case_observation(invalid)
                self.assertTrue(all(observed[name] is None for name in flags))
                self.assertEqual(observed["install"], {"exit_code": None, "timeout": None})
                self.assertEqual(observed["list"], {"exit_code": None, "timeout": None})
        for value in (False, 0, 1, "true", None):
            row = dict.fromkeys(flags, value)
            observed = audit.case_observation(row)
            self.assertTrue(all(observed[name] is (value if type(value) is bool else None) for name in flags))
        for code in (True, False, -(2**31) - 1, 2**32, "1", None):
            observed = audit.case_observation({"install": {"exit_code": code, "timeout": "false"}})
            self.assertEqual(observed["install"], {"exit_code": None, "timeout": None})
        for code in (-(2**31), -1, 0, 2**32 - 1):
            observed = audit.case_observation({"install": {"exit_code": code, "timeout": True}})
            self.assertEqual(observed["install"], {"exit_code": code, "timeout": True})



if __name__ == "__main__":
    unittest.main()
