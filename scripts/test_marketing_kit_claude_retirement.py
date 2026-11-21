"""Real filesystem controls for signed four-package Claude cache custody."""
import copy
import hashlib
import json
import os
from pathlib import Path
import runpy
import shutil
import sys
from tempfile import TemporaryDirectory
import unittest
from unittest.mock import patch

from marketing_kit_claude_retirement import SignedClosureRetirement

ROOT = Path(__file__).resolve().parents[1]


@unittest.skipUnless(sys.platform == "linux", "Linux signed cache custody")
class SignedClosureRetirementContracts(unittest.TestCase):
    def fixture(self, root):
        audit = runpy.run_path(str(ROOT / "scripts/live-install-audit.py"))
        home, project = root / "home", root / "project"
        profile = home / ".claude"
        profile.mkdir(parents=True)
        (project / ".uap").mkdir(parents=True)
        (profile / "foreign-neighbor.txt").write_text("unmanaged native neighbor\n")
        pins = copy.deepcopy(audit["MARKETING_KIT_SIGNED_CLOSURE"])
        rows, records, owners, actions, results = [], [], [], [], []
        installed = {"version": 2, "plugins": {"foreign@vendor": [{"scope": "user", "foreign": True}]}}
        markets = {"vendor": {"source": {"source": "github", "repo": "foreign/vendor"}, "foreign": 1}}
        for index, (alias, distribution) in enumerate(audit["MARKETING_KIT_DISTRIBUTIONS"].items()):
            pin = pins[distribution]
            physical, installation_id, binding_id = f"physical-{index}", f"installation-{index}", f"binding-{index}"
            active = project / ".claude/skills" / physical
            bundle = active / ".agentplugins-plugin"
            payload = bundle / ".agentplugins-runtime" / "plugin.json"
            payload.parent.mkdir(parents=True)
            payload.write_text(json.dumps({"name": alias, "version": pin["package_version"]}))
            if alias == "playwright":
                # The ordinary scanner must retain its large-file diagnostic;
                # the custody reader hashes the complete bounded binary.
                (payload.parent / "runtime.bin").write_bytes(b"x" * ((1 << 20) + 1))
            marketplace = "agentplugins-" + hashlib.sha256((physical + "\0" + str(project)).encode()).hexdigest()[:12]
            cache = profile / "plugins/cache" / marketplace / alias / pin["package_version"]
            cache.parent.mkdir(parents=True)
            shutil.copytree(bundle, cache)
            revision = {"distribution_id": distribution, "version": pin["package_version"],
                        **{key: pin[key] for key in ("release_sequence", "resolved_revision", "tree_digest", "manifest_digest")}}
            client = {"client_id": "claude", "scope": "project", "activation": "active",
                      "materialization": "materialized", "authentication": "not_required",
                      "policy": "allowed", "verification": "installation_verified", "package_revision": revision}
            public = {"installation_id": installation_id, "name": alias,
                      "source": pin["repository"] + "/" + pin["package_path"], "clients": [copy.deepcopy(client)],
                      "directory": {"recorded_distribution": distribution,
                                    "recorded_revision": pin["resolved_revision"],
                                    "recorded_release_sequence": pin["release_sequence"]}}
            receipt = {"kind": "claude_project_plugin_cache", "logical_name": alias,
                       "object_id": "claude-project-cache:" + alias + "@" + marketplace,
                       "path": str(bundle), "source_relative": cache.relative_to(profile).as_posix(),
                       "protection_class": "managed", "managed_digest": "sha256:" + str(index + 1) * 64}
            binding = {**client, "client_binding_id": binding_id, "physical_artifact_id": physical,
                       "target_locator": str(active), "native_objects": [receipt],
                       "receipts": [{"phase": "committed", "client_binding_id": binding_id,
                                     "active_path": str(active), "operation_id": f"operation-{index}"}]}
            record = {"installation_id": installation_id, "declared_name": alias, "origin_mode": "directory",
                      "source": {"repository": pin["repository"], "package_subpath": pin["package_path"],
                                 "resolved_revision": pin["resolved_revision"], "tree_digest": pin["tree_digest"]},
                      "package": {"declared_name": alias, "version": pin["package_version"],
                                  "manifest_digest": pin["manifest_digest"]},
                      "directory": {"distribution_id": distribution, "desired_release_sequence": pin["release_sequence"]},
                      "clients": {binding_id: binding}}
            key = alias + "@" + marketplace
            installed["plugins"][key] = [{"scope": "project", "projectPath": str(project), "installPath": str(cache)}]
            markets[marketplace] = {"source": {"source": "directory", "path": str(active)}, "installLocation": str(active)}
            actions.append({"alias": alias, "action": "remove", "client_id": "claude",
                            "installation_id": installation_id, "binding_id": binding_id})
            results.append({"operation": "remove", "client_id": "claude", "installation_id": installation_id, "outcome": "completed"})
            rows.append(public); records.append(record)
            owners.append({"cache": cache, "active": active, "key": key, "market": marketplace})
        state = project / ".uap/state-v2.json"
        state.write_text(json.dumps({"schema_version": 5, "installations": records}))
        (profile / "plugins/installed_plugins.json").write_text(json.dumps(installed))
        (profile / "plugins/known_marketplaces.json").write_text(json.dumps(markets))
        return {"root": root, "home": home, "project": project, "profile": profile, "audit": audit,
                "pins": pins, "public": rows, "owners": owners, "state": state,
                "cleanup": {"result": "success", "outcome": "completed", "scope": "project",
                            "actions": actions, "results": results}}

    def capture(self, f):
        return SignedClosureRetirement(f["root"], f["home"], f["project"], f["public"], f["pins"], f["audit"])

    def retire(self, f, marker=True):
        for index, owner in enumerate(f["owners"]):
            quarantine = owner["cache"].parent / (".agentplugins-cache-retire-" + f"{index + 1:032x}")
            quarantine.mkdir(mode=0o700)
            owner["moved"] = quarantine / "retired"
            owner["cache"].rename(owner["moved"])
            if marker:
                (owner["moved"] / ".orphaned_at").write_text("1794880808341")
            shutil.rmtree(owner["active"])
        installed_path = f["profile"] / "plugins/installed_plugins.json"
        installed = json.loads(installed_path.read_text())
        for owner in f["owners"]:
            del installed["plugins"][owner["key"]]
        installed_path.write_text(json.dumps(installed))
        markets_path = f["profile"] / "plugins/known_marketplaces.json"
        markets = json.loads(markets_path.read_text())
        for owner in f["owners"]:
            del markets[owner["market"]]
        markets_path.write_text(json.dumps(markets))

    def scan(self, f):
        audit, profile = f["audit"], f["profile"]
        observed = audit["snapshot"]([profile])
        identities = set(audit["MARKETING_KIT_DISTRIBUTIONS"])
        identities.update(distribution.rsplit("/", 1)[-1] for distribution in f["pins"])
        result = set()
        for alias in identities:
            result.update(audit["native_identity_residuals"]([profile], alias, observed, "claude"))
        for base, directories, _ in os.walk(profile, followlinks=False):
            for name in directories:
                if name in identities:
                    result.add(".claude/" + (Path(base) / name).relative_to(profile).as_posix() + ": directory")
        return result

    def test_exact_four_cache_custody_optional_marker_and_raw_findings_preserved(self):
        for marker in (False, True):
            with self.subTest(marker=marker), TemporaryDirectory() as directory:
                f = self.fixture(Path(directory).resolve())
                subject = self.capture(f)
                self.retire(f, marker)
                residuals = self.scan(f)
                self.assertTrue(any(value.endswith(": unscanned-too-large") for value in residuals))
                proof = subject.finish(f["cleanup"], residuals, lambda: self.scan(f))
                self.assertEqual(proof["status"], "verified-signed-closure-cache-retirement-at-checkpoint")
                self.assertFalse(proof["writer_closure_claimed"])
                self.assertEqual(len(proof["members"]), 4)
                self.assertEqual(residuals, self.scan(f))
                self.assertEqual(subject.fds, [])
                self.assertTrue(all(owner["moved"].is_dir() for owner in f["owners"]))

    def test_signed_state_public_receipt_namespace_and_bundle_mutations_refused(self):
        for mutation in ("source", "digest", "public", "scope", "receipt", "namespace", "bundle", "hardlink", "symlink"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as directory:
                f = self.fixture(Path(directory).resolve())
                state = json.loads(f["state"].read_text())
                record = state["installations"][0]
                binding = next(iter(record["clients"].values()))
                if mutation == "source": record["source"]["repository"] = "foreign/repo"
                elif mutation == "digest": record["source"]["tree_digest"] = "sha256:" + "0" * 64
                elif mutation == "public": f["public"][0]["directory"]["recorded_release_sequence"] = 99
                elif mutation == "scope": binding["scope"] = "user"
                elif mutation == "receipt": binding["receipts"][0]["phase"] = "pending"
                elif mutation == "namespace": binding["native_objects"][0]["source_relative"] = "plugins/cache/foreign/marketing-kit/0.1.0"
                else:
                    payload = f["owners"][0]["cache"] / ".agentplugins-runtime/plugin.json"
                    if mutation == "bundle": payload.write_text("foreign")
                    elif mutation == "hardlink": os.link(payload, payload.with_name("hardlink"))
                    elif mutation == "symlink": payload.with_name("link").symlink_to(payload)
                f["state"].write_text(json.dumps(state))
                with self.assertRaises((ValueError, OSError)):
                    self.capture(f)

    def test_removal_receipts_must_name_every_prior_signed_owner(self):
        for mutation in ("missing", "duplicate", "wrong_id", "wrong_binding", "wrong_client", "wrong_scope", "unchanged", "pending"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as directory:
                f = self.fixture(Path(directory).resolve()); subject = self.capture(f)
                self.retire(f)
                cleanup = f["cleanup"]
                if mutation == "missing": cleanup["actions"].pop()
                elif mutation == "duplicate": cleanup["actions"][0] = copy.deepcopy(cleanup["actions"][1])
                elif mutation == "wrong_id": cleanup["results"][0]["installation_id"] = "foreign"
                elif mutation == "wrong_binding": cleanup["actions"][0]["binding_id"] = "foreign"
                elif mutation == "wrong_client": cleanup["results"][0]["client_id"] = "codex"
                elif mutation == "wrong_scope": cleanup["scope"] = "user"
                elif mutation == "unchanged": cleanup["actions"][0]["action"] = "unchanged"
                elif mutation == "pending": cleanup["results"][0]["outcome"] = "pending"
                with self.assertRaises(ValueError):
                    subject.finish(cleanup, self.scan(f), lambda: self.scan(f))
                self.assertEqual(subject.fds, [])

    def test_foreign_native_registry_rows_and_unowned_residuals_never_qualify(self):
        for mutation in ("installed_foreign", "market_foreign", "active_registration", "extra_residual", "extra_quarantine", "marker", "retired_bytes", "retired_replaced", "parent_replaced", "project_replaced"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as directory:
                f = self.fixture(Path(directory).resolve()); subject = self.capture(f)
                self.retire(f)
                owner = f["owners"][0]
                if mutation in ("installed_foreign", "active_registration"):
                    path = f["profile"] / "plugins/installed_plugins.json"
                    doc = json.loads(path.read_text())
                    if mutation == "installed_foreign": doc["plugins"]["foreign@vendor"][0]["foreign"] = False
                    else: doc["plugins"][owner["key"]] = [{"scope": "project"}]
                    path.write_text(json.dumps(doc))
                elif mutation == "market_foreign":
                    path = f["profile"] / "plugins/known_marketplaces.json"
                    doc = json.loads(path.read_text()); doc["vendor"]["foreign"] = True
                    path.write_text(json.dumps(doc))
                elif mutation == "extra_residual": (f["profile"] / "marketing-kit.json").write_text("{}")
                elif mutation == "extra_quarantine":
                    (owner["cache"].parent / (".agentplugins-cache-retire-" + "f" * 32)).mkdir(mode=0o700)
                elif mutation == "marker": (owner["moved"] / ".orphaned_at").write_text("not-a-timestamp")
                elif mutation == "retired_bytes": (owner["moved"] / ".agentplugins-runtime/plugin.json").write_text("foreign")
                elif mutation == "retired_replaced":
                    old = owner["moved"].with_name("original"); owner["moved"].rename(old)
                    shutil.copytree(old, owner["moved"])
                elif mutation == "parent_replaced":
                    parent = owner["cache"].parent; old = parent.with_name("original"); parent.rename(old)
                    shutil.copytree(old, parent)
                elif mutation == "project_replaced":
                    f["project"].rename(f["project"].with_name("original-project"))
                    f["project"].mkdir()
                with self.assertRaises((ValueError, OSError)):
                    subject.finish(f["cleanup"], self.scan(f), lambda: self.scan(f))
                self.assertEqual(subject.fds, [])

    def test_rescan_cannot_mutate_retained_tree_or_registry_after_first_check(self):
        for mutation in ("tree", "registry", "active_cache", "quarantine_extra"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as directory:
                f = self.fixture(Path(directory).resolve()); subject = self.capture(f)
                self.retire(f)
                residuals = self.scan(f)
                calls = []
                def rescan():
                    calls.append(True)
                    observed = self.scan(f)
                    owner = f["owners"][0]
                    if mutation == "tree": (owner["moved"] / "extra.txt").write_text("foreign")
                    elif mutation == "registry":
                        path = f["profile"] / "plugins/known_marketplaces.json"
                        doc = json.loads(path.read_text()); doc["vendor"]["foreign"] = 2
                        path.write_text(json.dumps(doc))
                    elif mutation == "active_cache": owner["cache"].mkdir()
                    else: (owner["moved"].parent / "extra").mkdir()
                    return observed
                with self.assertRaises(ValueError):
                    subject.finish(f["cleanup"], residuals, rescan)
                self.assertTrue(calls)
                self.assertEqual(subject.fds, [])

    def test_close_is_safe_when_command_refuses_before_finish(self):
        with TemporaryDirectory() as directory:
            f = self.fixture(Path(directory).resolve()); subject = self.capture(f)
            subject.close(); subject.close()
            self.assertEqual(subject.fds, [])
            with self.assertRaises(ValueError):
                subject.finish(f["cleanup"], {"foreign"}, lambda: {"foreign"})

    def test_project_identity_cannot_reset_between_member_captures(self):
        original = SignedClosureRetirement.capture
        with TemporaryDirectory() as directory:
            f = self.fixture(Path(directory).resolve())
            seen = []
            def capture(subject, *args):
                owner = original(subject, *args)
                seen.append(subject)
                if len(seen) == 1:
                    old = f["project"].with_name("original-project")
                    f["project"].rename(old)
                    shutil.copytree(old, f["project"])
                return owner
            with patch.object(SignedClosureRetirement, "capture", capture):
                with self.assertRaises(ValueError):
                    self.capture(f)
            self.assertTrue(seen)
            self.assertEqual(seen[0].fds, [])

    def test_marketing_entry_keeps_strict_default_and_profile_scope_binding(self):
        runner = runpy.run_path(str(ROOT / "scripts/marketing-kit-project-release-e2e.py"))
        check = runner["check_removed_native_components"]
        with TemporaryDirectory() as directory:
            f = self.fixture(Path(directory).resolve()); subject = self.capture(f)
            self.retire(f)
            with self.assertRaisesRegex(AssertionError, "native Marketing Kit remnants"):
                check("claude", f["home"], f["root"], "strict default")
            with self.assertRaisesRegex(AssertionError, "another profile"):
                check("claude", f["home"], f["root"], "wrong scope",
                      project_scope=True, claude_retirement=subject, cleanup=f["cleanup"])
            raw = runner["native_component_residuals"]("claude", f["home"], f["root"], "raw")
            proof = check("claude", f["home"], f["root"], "guarded checkpoint",
                          claude_retirement=subject, cleanup=f["cleanup"])
            self.assertEqual(proof["matched_residual_count"], len(raw))
            self.assertEqual(raw, runner["native_component_residuals"](
                "claude", f["home"], f["root"], "still raw"))
            self.assertEqual(subject.fds, [])

    def test_execute_captures_verified_four_member_receipt_for_real_retirement(self):
        runner = runpy.run_path(str(ROOT / "scripts/marketing-kit-project-release-e2e.py"))
        execute = runner["execute"]
        with TemporaryDirectory() as directory:
            f = self.fixture(Path(directory).resolve()); subject = self.capture(f)
            captured = []
            def invoke(argv, env, project, timeout):
                self.assertEqual(argv, ["/released/agentplugins", "remove", "marketing-kit",
                    "--scope", "project", "--project-root", str(f["project"]), "--format", "json"])
                self.assertEqual(project, f["project"])
                self.retire(f)
                return {"exit_code": 0, "timeout": False, "_json": copy.deepcopy(f["cleanup"])}
            try:
                with patch.dict(execute.__globals__, {"invoke": invoke}):
                    step = execute("/released/agentplugins", {}, f["project"], "claude", "remove B",
                        "remove", "marketing-kit", "--scope", "project", "--project-root", str(f["project"]),
                        capture=captured.append)
                self.assertEqual(step, {"stage": "remove B", "exit_code": 0, "result": "passed"})
                self.assertEqual(captured, [f["cleanup"]])
                proof = runner["check_removed_native_components"](
                    "claude", f["home"], f["root"], "remove B native HOME",
                    claude_retirement=subject, cleanup=captured[0])
                self.assertEqual(len(proof["members"]), 4)
                self.assertNotIn("actions", proof)
                self.assertEqual(subject.fds, [])
            finally:
                subject.close()

    def test_execute_refusal_never_captures_receipt_and_caller_closes_custody(self):
        runner = runpy.run_path(str(ROOT / "scripts/marketing-kit-project-release-e2e.py"))
        execute = runner["execute"]
        for failure in ("process", "pending", "partial_receipts"):
            with self.subTest(failure=failure), TemporaryDirectory() as directory:
                f = self.fixture(Path(directory).resolve()); subject = self.capture(f)
                captured = []
                result = {"exit_code": 0, "timeout": False, "_json": copy.deepcopy(f["cleanup"])}
                if failure == "process": result["exit_code"] = 1
                elif failure == "pending": result["_json"]["outcome"] = "pending"
                else: result["_json"]["results"].pop()
                try:
                    with patch.dict(execute.__globals__, {"invoke": lambda *args: result}):
                        with self.assertRaisesRegex(AssertionError, "CLI did not succeed"):
                            execute("/released/agentplugins", {}, f["project"], "claude", "remove B",
                                "remove", "marketing-kit", "--scope", "project", "--project-root", str(f["project"]),
                                capture=captured.append)
                    self.assertEqual(captured, [])
                finally:
                    subject.close()
                self.assertEqual(subject.fds, [])
                self.assertTrue(all(owner["cache"].is_dir() for owner in f["owners"]))


if __name__ == "__main__":
    unittest.main()
