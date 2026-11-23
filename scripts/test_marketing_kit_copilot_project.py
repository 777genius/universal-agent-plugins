"""Contract controls; these simulated receipts are never native E2E evidence."""

import copy
import json
import os
from pathlib import Path
import runpy
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[1]
AUDIT = runpy.run_path(str(ROOT / "scripts/live-install-audit.py"))
RUNNER = runpy.run_path(str(ROOT / "scripts/marketing-kit-project-release-e2e.py"))
CONTRACT = runpy.run_path(str(ROOT / "scripts/marketing_kit_copilot_project.py"))
ProjectTrustObservation = CONTRACT["ProjectTrustObservation"]


class CopilotProjectConsentContracts(unittest.TestCase):
    def fixture(self, root):
        environment, project, *_ = RUNNER["run_case"]("unused", "copilot", root, "B")
        return environment, project, Path(environment["HOME"]) / ".copilot/config.json"

    def receipt(self, project, *, unchanged=False):
        actions, results, bindings, installed = [], [], [], {}
        for index, (alias, distribution) in enumerate(AUDIT["MARKETING_KIT_DISTRIBUTIONS"].items()):
            pin = AUDIT["MARKETING_KIT_SIGNED_CLOSURE"][distribution]
            identity = f"contract-installation-{index}"
            installed[distribution] = identity
            actions.append({"alias": alias, "client_id": "copilot",
                "action": "unchanged" if unchanged else "install",
                "to_version": pin["package_version"], "package_digest": pin["tree_digest"],
                "revision": pin["resolved_revision"], **({"installation_id": identity} if unchanged else {})})
            if not unchanged:
                results.append({"installation_id": identity, "client_id": "copilot", "operation": "install",
                    "outcome": "incomplete", "reason": CONTRACT["TRUST_REFUSAL"],
                    "manual_actions": ["retry client activation and verify client visibility"],
                    "next_actions": [{"kind": "activate", "reason": CONTRACT["TRUST_REFUSAL"]}]})
                bindings.append({"alias": alias, "client_id": "copilot", "status": "incomplete", "code": "FAILED"})
        envelope = {"schema_version": 1, "command": "agentplugins install", "scope": "project",
            "scope_selection_reason": "explicit", "result": "success" if unchanged else "failure",
            "outcome": "completed" if unchanged else "partial", "actions": actions, "results": results,
            "selection": {"scope": "project", "scope_selection_reason": "explicit",
                "project_root": str(project), "manifest_path": str(project / "uap.toml"),
                "lock_path": str(project / "uap.lock")}}
        if not unchanged:
            envelope["bindings"] = bindings
        result = {"exit_code": 0 if unchanged else 1, "timeout": False, "_json": envelope,
            "stdout_full": json.dumps(envelope), "stderr_full": "" if unchanged else
            "agentplugins: dependency sync stopped; inspect binding results and next actions: "
            "4 of 4 bindings failed: " + CONTRACT["TRUST_REFUSAL"] + "\n"}
        return result, installed

    def test_observer_authors_no_configuration_and_requires_exact_real_add_effect(self):
        with tempfile.TemporaryDirectory() as directory:
            env, project, config = self.fixture(Path(directory).resolve())
            observer = ProjectTrustObservation(AUDIT, env, project)
            observer.unchanged()
            self.assertFalse(config.exists())
            with self.assertRaisesRegex(AssertionError, "exactly the selected"):
                observer.owner_add_completed()
            # Simulate the adapter's effect only for this filesystem contract.
            config.write_text(json.dumps({"trustedFolders": [str(project)]}))
            with self.assertRaisesRegex(AssertionError, "outside explicit owner add"):
                observer.unchanged()
            proof = observer.owner_add_completed()
            self.assertFalse(proof["fixture_authored_native_trust"])
            self.assertFalse(proof["copied_native_trust"])
            observer.unchanged()
            held = config.with_name("original.json")
            config.rename(held)
            config.write_bytes(held.read_bytes())
            with self.assertRaisesRegex(AssertionError, "outside explicit owner add"):
                observer.unchanged()

    def test_remembered_foreign_entries_are_preserved_and_selected_trust_cannot_be_preseeded(self):
        for selected in (False, True):
            with self.subTest(selected=selected), tempfile.TemporaryDirectory() as directory:
                env, project, config = self.fixture(Path(directory).resolve())
                before = {"theme": "keep", "trustedFolders": [str(project) if selected else "/foreign"]}
                config.write_text(json.dumps(before))
                if selected:
                    with self.assertRaisesRegex(AssertionError, "already has remembered trust"):
                        ProjectTrustObservation(AUDIT, env, project)
                else:
                    observer = ProjectTrustObservation(AUDIT, env, project)
                    config.write_text(json.dumps(dict(before, trustedFolders=["/foreign", str(project)])))
                    observer.owner_add_completed()
                    observer.unchanged()

    def test_owner_add_cannot_grant_broad_alternate_or_unrelated_configuration(self):
        for mutation in ("parent", "extra-project", "extra-field", "alternate"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as directory:
                env, project, config = self.fixture(Path(directory).resolve())
                observer = ProjectTrustObservation(AUDIT, env, project)
                value = {"trustedFolders": [str(project)]}
                if mutation == "parent": value["trustedFolders"] = [str(project.parent)]
                elif mutation == "extra-project": value["trustedFolders"].append("/foreign")
                elif mutation == "extra-field": value["allowAll"] = True
                else: config = Path(env["HOME"]) / ".config/copilot/config.json"
                config.write_text(json.dumps(value))
                with self.assertRaisesRegex(AssertionError, "exactly the selected"):
                    observer.owner_add_completed()

    def test_unsafe_or_replaced_profile_and_configuration_refuse(self):
        for mutation in ("symlink", "hardlink", "oversized", "duplicate", "malformed", "root"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as directory:
                env, project, config = self.fixture(Path(directory).resolve())
                observer = ProjectTrustObservation(AUDIT, env, project)
                if mutation in ("symlink", "hardlink"):
                    foreign = project.parent / "foreign-config"
                    foreign.write_text('{}')
                    if mutation == "symlink": config.symlink_to(foreign)
                    else: os.link(foreign, config)
                elif mutation == "oversized": config.write_bytes(b"x" * ((1 << 20) + 1))
                elif mutation == "duplicate": config.write_text('{"trustedFolders":[],"trustedFolders":[]}')
                elif mutation == "malformed": config.write_text('{"trustedFolders":null}')
                else:
                    profile = config.parent
                    profile.rename(profile.with_name("previous-copilot"))
                    profile.mkdir()
                with self.assertRaises((AssertionError, ValueError)):
                    observer.unchanged()

    def test_exact_cold_refusal_never_claims_absent_materialization_or_cold_success(self):
        project = Path('/selected/B/project')
        result, _ = self.receipt(project)
        proof = CONTRACT["verify_cold_refusal"](result, project, AUDIT)
        self.assertEqual(proof["status"], "refused_as_expected")
        self.assertEqual(proof["refused_bindings"], 4)
        self.assertFalse(proof["materialization_absence_claimed"])
        self.assertNotIn("native_trust_unchanged", proof)  # requires the independent filesystem observer

    def test_cold_refusal_rejects_every_unrelated_receipt_or_source_result(self):
        project = Path('/selected/B/project')
        original, _ = self.receipt(project)
        mutations = {
            "success": lambda r: r.update(exit_code=0),
            "bool-exit": lambda r: r.update(exit_code=True),
            "timeout": lambda r: r.update(timeout=True),
            "stderr": lambda r: r.update(stderr_full="unrelated transport failure\n"),
            "scope": lambda r: r['_json'].update(scope="user"),
            "schema": lambda r: r['_json'].update(schema_version=True),
            "command": lambda r: r['_json'].update(command="agentplugins add"),
            "selection": lambda r: r['_json']['selection'].update(project_root="/other"),
            "outcome": lambda r: r['_json'].update(outcome="completed"),
            "alias": lambda r: r['_json']['actions'][0].update(alias="unknown"),
            "target": lambda r: r['_json']['actions'][0].update(client_id="vscode"),
            "source": lambda r: r['_json']['actions'][0].update(package_digest="sha256:" + "0" * 64),
            "revision": lambda r: r['_json']['actions'][0].update(revision="0" * 40),
            "result": lambda r: r['_json']['results'][0].update(outcome="completed"),
            "reason": lambda r: r['_json']['results'][0].update(reason="scanner failed"),
            "id": lambda r: r['_json']['results'][0].update(installation_id=r['_json']['results'][1]['installation_id']),
            "next-action": lambda r: r['_json']['results'][0].update(next_actions=[]),
            "binding": lambda r: r['_json']['bindings'][0].update(status="success"),
            "missing": lambda r: r['_json']['bindings'].pop(),
        }
        for name, mutate in mutations.items():
            with self.subTest(name=name):
                result = copy.deepcopy(original)
                mutate(result)
                result['stdout_full'] = json.dumps(result['_json'])
                with self.assertRaises((AssertionError, ValueError)):
                    CONTRACT["verify_cold_refusal"](result, project, AUDIT)
        for body in ('{"result":"success","result":"failure"}', original['stdout_full'] + '{}'):
            with self.assertRaises((AssertionError, ValueError)):
                CONTRACT["verify_cold_refusal"](dict(original, stdout_full=body), project, AUDIT)

    def test_post_consent_noop_requires_all_existing_ids_and_exact_signed_pins(self):
        project = Path('/selected/B/project')
        result, installed = self.receipt(project, unchanged=True)
        envelope = CONTRACT["verified_unchanged_locked_result"](result, project, installed, AUDIT)
        self.assertEqual(envelope, result['_json'])
        # The existing cold-install success oracle still refuses this no-op.
        self.assertFalse(RUNNER['valid_legacy_dependency_result'](envelope, 'install', 'copilot', 'project'))
        for field, value in (('installation_id', 'different'), ('action', 'install'),
                             ('client_id', 'claude'), ('revision', '0' * 40), ('to_version', '99')):
            altered = copy.deepcopy(result)
            altered['_json']['actions'][0][field] = value
            altered['stdout_full'] = json.dumps(altered['_json'])
            with self.subTest(field=field), self.assertRaises(AssertionError):
                CONTRACT["verified_unchanged_locked_result"](altered, project, installed, AUDIT)
        with self.assertRaises((AssertionError, ValueError)):
            CONTRACT["verified_unchanged_locked_result"](dict(result, stdout_full=''), project, installed, AUDIT)
        with self.assertRaises(AssertionError):
            CONTRACT["verified_unchanged_locked_result"](result, Path('/other/project'), installed, AUDIT)


if __name__ == '__main__':
    unittest.main()
