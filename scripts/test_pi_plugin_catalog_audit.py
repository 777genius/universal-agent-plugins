"""Native output substitutions exercise refusal logic, not genuine native E2E."""
import json
from copy import deepcopy
from pathlib import Path
from runpy import run_path
import tempfile
import unittest

from pi_plugin_catalog_audit import ACCEPTANCE, PiPluginCatalogAudit, summary

API = run_path(str(Path(__file__).with_name("live-install-audit.py")))


class PiPluginCatalogTests(unittest.TestCase):
    def setup_case(self, root, mutation=None, *, source="fixture", index=0):
        home, project, data = (root / name for name in ("home", "project", "data"))
        for path in (home, project, data):
            path.mkdir()
        env = API["isolated_client_env"](home, root, data)
        profile = home / ".pi/agent"
        def probe(argv, cwd, env, output, label, **options):
            skill = profile / "skills/fixture/SKILL.md"
            config = profile / "mcp.json"
            if "--mode" in argv:
                nonce = json.loads(options["input_bytes"])["id"]
                rows = ([{"name": "skill:fixture", "source": "skill", "description": "Fixture",
                          "sourceInfo": {"path": str(skill), "source": "auto", "scope": "user",
                                         "origin": "top-level", "baseDir": str(profile)}}] if skill.exists() else [])
                value = {"id": nonce, "type": "response", "command": "get_commands", "success": True,
                         "data": {"commands": rows}}
            else:
                rows = ([{"name": "fixture", "scope": "global", "source": str(config), "enabled": True,
                          "exposure": "codemode", "transport": "http://127.0.0.1:1/mcp",
                          "state": "failed", "tools": [], "error": "fetch failed"}] if config.exists() else [])
                value = {"servers": rows, "errors": []}
            if mutation and "after-add" in label:
                mutation(value, profile)
            body = (json.dumps(value) + "\n").encode()
            return {"error": None, "exit": int("servers" in value and bool(rows)), "cwd": str(cwd),
                    "stdin_written": True, "stdin_response_confirmed": True}, body, b""
        api = dict(API, AGENT_BINARIES={"pi": "/fixture/pi"}, COPILOT_CATALOG_PROBE=probe)
        audit = PiPluginCatalogAudit(home, project, data, env,
            {"status": "reported", "value": "1.5.0", "exit_code": 0}, api, source=source, index=index)
        audit.checkpoint("before-add")
        skill = profile / "skills/fixture/SKILL.md"
        skill.parent.mkdir(parents=True)
        skill.write_text("---\nname: fixture\ndescription: Fixture\n---\n")
        (profile / "mcp.json").write_text(json.dumps({"mcpServers": {"fixture": {"url": "http://127.0.0.1:1/mcp"}}}))
        audit.select({"data": {"actions": [{"client_id": "pi", "component_support": [
            {"kind": kind, "name": "fixture", "support": "native"} for kind in ("skill", "mcp_server")]}]}})
        return audit, profile, skill

    def test_registration_lifecycle_preserves_failed_connection(self):
        with tempfile.TemporaryDirectory() as tmp:
            audit, profile, skill = self.setup_case(Path(tmp))
            audit.checkpoint("after-add")
            audit.checkpoint("repeat-add")
            skill.unlink()
            (profile / "mcp.json").unlink()
            audit.checkpoint("after-remove")
            self.assertTrue(audit.verified())
            self.assertFalse(audit.evidence["native_ready"])
            self.assertFalse(audit.evidence["tools_call_qualified"])
            self.assertEqual(audit.evidence["checkpoints"]["after-add"]["mcp"]["exit"], 1)

    def test_wrong_or_missing_native_source_refused(self):
        def change(value, _profile, fault):
            if "servers" in value:
                if fault == "missing": value["servers"] = []
                elif fault == "duplicate": value["servers"] *= 2
                elif fault == "parser-error": value["errors"] = ["invalid configuration"]
                elif fault in ("source", "scope", "transport"): value["servers"][0][fault] = "foreign"
            elif fault == "skill-path": value["data"]["commands"][0]["sourceInfo"]["path"] = "/foreign/SKILL.md"
        for fault in ("missing", "duplicate", "parser-error", "source", "scope", "transport", "skill-path"):
            with self.subTest(fault=fault), tempfile.TemporaryDirectory() as tmp:
                audit, _, _ = self.setup_case(Path(tmp), lambda value, profile: change(value, profile, fault))
                with self.assertRaises(ValueError): audit.checkpoint("after-add")
                self.assertFalse(audit.verified())

    def test_probe_cannot_modify_existing_profile(self):
        def modify(_value, profile):
            (profile / "mcp.json").write_text("{}")
        with tempfile.TemporaryDirectory() as tmp:
            audit, _, _ = self.setup_case(Path(tmp), modify)
            with self.assertRaises(ValueError): audit.checkpoint("after-add")

    def test_repeated_source_and_checkpoint_sequence_are_exact(self):
        with tempfile.TemporaryDirectory() as tmp:
            audit, _, skill = self.setup_case(Path(tmp))
            with self.assertRaises(ValueError): audit.checkpoint("repeat-add")
            audit.checkpoint("after-add")
            skill.write_text("changed")
            with self.assertRaises(ValueError): audit.checkpoint("repeat-add")

    def test_checked_keeps_partial_native_failure_evidence(self):
        with tempfile.TemporaryDirectory() as tmp:
            audit, _, skill = self.setup_case(Path(tmp))
            skill.write_text("changed")
            with self.assertRaises(ValueError) as caught:
                audit.checked("checkpoint", "after-add")
            self.assertIs(caught.exception.pi_plugin_registration_proof, audit.evidence)
            self.assertEqual(audit.evidence["refusal"]["stage"], "checkpoint")
            self.assertFalse(audit.verified())

    def test_summary_requires_complete_ordered_sources_and_checkpoints(self):
        sources = ["fixture-" + str(index) for index in range(30)]
        sources[26] = "github:obra/the-elements-of-style@05fc4f0d2b97b7c042dd9949ad658568e4a1324e"
        fixtures = {"plugin_aliases": sources, "plugin_declared_names": {}}
        template = {"status": "package-installed-native-unqualified", "target": "pi", "kind": "plugin",
                    "native_residuals": []}
        template.update(dict.fromkeys(("listed_identity", "repeat_verified", "repeat_native_unchanged",
            "repeat_lock_unchanged", "removal_verified", "post_list_schema_valid", "sentinels_preserved"), True))
        template.update(dict.fromkeys(("failed_install_registered", "failed_install_residue", "post_remove_identity_registered"), False))
        template.update({key: {"exit_code": 0, "timeout": False} for key in (
            "install", "list", "repeat_add", "repeat_list", "cleanup", "post_remove_list")})
        rows = [{**deepcopy(template), "source": source, "identity": source, "index": index}
                for index, source in enumerate(sources)]
        rows[26]["status"] = "documented-or-source-unsupported"
        with tempfile.TemporaryDirectory() as tmp:
            for row in rows:
                if row["index"] == 26:
                    continue
                root = Path(tmp) / f'plugin-{row["index"]:02d}'
                root.mkdir()
                audit, profile, skill = self.setup_case(root, source=row["source"], index=row["index"])
                audit.checkpoint("after-add")
                audit.checkpoint("repeat-add")
                skill.unlink()
                (profile / "mcp.json").unlink()
                audit.checkpoint("after-remove")
                row["pi_plugin_registration_proof"] = audit.evidence
        report = {"acceptance": ACCEPTANCE, "target": "pi", "kind": "plugin", "platform": "linux",
                  "architecture": "x86_64", "passed": 0, "source_selection_complete": True,
                  "fixture_count": 30, "cases_attempted": 30, "results": rows,
                  "client_version": {"status": "reported", "value": "1.5.0", "exit_code": 0},
                  "cli_version": {"status": "reported", "value": "agentplugins 0.1.115", "exit_code": 0}}
        negative = lambda row: row["status"] == "documented-or-source-unsupported"
        observed = summary(report, fixtures, negative)
        self.assertTrue(observed["pi_native_registration_qualified"])
        self.assertEqual(observed["pi_native_registration_passed"], 29)
        for fault in ("checkpoint", "source", "cleanup", "model", "version", "native-pass",
                      "proof-replay", "capture-replay", "wrong-negative"):
            with self.subTest(fault=fault):
                broken = deepcopy(report)
                first = broken["results"][0]
                if fault == "checkpoint": del first["pi_plugin_registration_proof"]["checkpoints"]["before-add"]
                elif fault == "source": first["source"] = "different"
                elif fault == "cleanup": first["cleanup"]["exit_code"] = 1
                elif fault == "model": first["pi_plugin_registration_proof"]["model_calls"] = True
                elif fault == "version": broken["client_version"]["value"] = "unqualified"
                elif fault == "proof-replay": first["pi_plugin_registration_proof"] = broken["results"][1]["pi_plugin_registration_proof"]
                elif fault == "capture-replay": first["pi_plugin_registration_proof"]["checkpoints"] = broken["results"][1]["pi_plugin_registration_proof"]["checkpoints"]
                elif fault == "wrong-negative": first["status"] = "documented-or-source-unsupported"
                else: broken["passed"] = 1
                self.assertFalse(summary(broken, fixtures, negative)["pi_native_registration_qualified"])


if __name__ == "__main__":
    unittest.main()
