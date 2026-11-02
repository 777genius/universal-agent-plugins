"""Inert parser controls; these never execute or qualify the native client."""
import base64
import copy
import json
import unittest

import project_native_qoder_cycle as cycle
import project_native_qoder_cycle_logs as logs
import project_native_qoder_cycle_proof as proof
import project_native_runtime as runtime
from test_project_native_qoder_cycle_fixtures import LAYOUT, NONCE, SOURCE, container, raw, stream


def aggregate(bodies, index, change):
    name = cycle.MEMBERS[index]
    rows = [json.loads(line) for line in bodies[name].splitlines()]
    change(rows)
    bodies[name] = b"".join(raw(row) + b"\n" for row in rows)


def summary_change(bodies, **change):
    value = json.loads(bodies["runtime-attempt.json"])
    value.update(change)
    bodies["runtime-attempt.json"] = raw(value)
    return value


class QoderCycleContracts(unittest.TestCase):
    def test_digest_framing_matches_independent_actual_production_staging(self):
        # Literal hashes from the actual production planner/stager/Prepared
        # adapter export 8da61784...; its absent public receipts are not invented.
        base = "0123456789abcdef0123456789abcdef"
        expected = {
            "a": ("sha256:dfd0babe6a02cd29f1427e44c4b12a7515dfaac0f646380102dab35f65931f62", "sha256:9f7dbd302ca44e1ecb60cb7ddb94330877191378393ef8a3454e87b1dd965418"),
            "b": ("sha256:eb4fea6357e08d9ccad93947938bce06cb4d10f664b907d2d5696abaf0d9b769", "sha256:b2470dadb2c9402e7885e9e7eb1507d645cc0765c336736c5a9032af34096391"),
        }
        manifest = b'{\n  "description": "Disposable Qoder project cycle",\n  "name": "uap-qoder-cycle",\n  "skills": "./skills/",\n  "version": "1.0.0"\n}\n'
        for key, (source, installed) in expected.items():
            self.assertEqual(proof.tree_digest(proof.authored(base, key, key + "-one"), True), source)
            projected = proof.projected(base, key, key + "-one")
            self.assertEqual(projected[".agentplugins-qoder-native/.qoder-plugin/plugin.json"], manifest)
            self.assertEqual(proof.tree_digest(projected), installed)

    def test_authored_complete_container_closes_only_supported_two_flags(self):
        bodies = container()
        result = runtime.cli_attempt_members(bodies, SOURCE, "qoder")
        self.assertEqual(len(bodies), 4)
        self.assertEqual((len(result["cycle_public_stages"]), len(result["cycle_catalog_stages"]), result["cycle_log_runs"]), (14, 20, 21))
        self.assertEqual({key for key, value in result["qualification"].items() if value}, set(cycle.FLAGS[:2]))
        self.assertFalse(result["vendor_authentication_qualified"])
        self.assertFalse(result["real_model_qualified"])
        self.assertEqual([len(cycle.records(bodies[name], order)) for name, order in zip(cycle.MEMBERS, (cycle.PUBLIC, cycle.CATALOG, cycle.PROOF))], [14, 20, 25])

    def test_failed_prefix_keeps_unknown_native_body_all_false_and_missing_empty_aggregates(self):
        bodies = container()
        public = [json.loads(line) for line in bodies[cycle.MEMBERS[0]].splitlines()][:4]
        catalog = [json.loads(bodies[cycle.MEMBERS[1]].splitlines()[0])]
        catalog[0]["value"]["stdout"] = stream(b'{"unknownShippingShape":true}')
        proofs = [json.loads(line) for line in bodies[cycle.MEMBERS[2]].splitlines()][:10]
        for name, rows in zip(cycle.MEMBERS, (public, catalog, proofs)):
            bodies[name] = b"".join(raw(row) + b"\n" for row in rows)
        expected = summary_change(bodies, runtime_attempt_complete=False, test_failed=True, cycle_observed_complete=False,
                                  cycle_stage="catalog:empty-outside", cycle_public_stages=list(cycle.PUBLIC[:4]), cycle_catalog_stages=["empty-outside"], cycle_log_runs=2,
                                  qualification={key: False for key in cycle.FLAGS})
        self.assertEqual(cycle.members(bodies, SOURCE), expected)
        self.assertEqual(cycle.stream(catalog[0]["value"]["stdout"]), b'{"unknownShippingShape":true}')
        for name in cycle.MEMBERS[:2]: bodies.pop(name)
        expected = summary_change(bodies, cycle_stage="prepare", cycle_public_stages=[], cycle_catalog_stages=[], cycle_log_runs=0)
        self.assertEqual(cycle.members(bodies, SOURCE), expected)

    def test_late_failure_keeps_existing_cleanup_envelope_without_promoting_success(self):
        bodies = container()
        expected = summary_change(bodies, elapsed_ms=180001, runtime_attempt_complete=False, test_failed=True,
                                  qualification={key: False for key in cycle.FLAGS})
        self.assertEqual(cycle.members(bodies, SOURCE), expected)
        self.assertFalse(any(expected["qualification"].values()))
        summary_change(bodies, elapsed_ms=195001)
        with self.assertRaises(ValueError): cycle.members(bodies, SOURCE)
        bodies = container()
        summary_change(bodies, elapsed_ms=150000)
        with self.assertRaises(ValueError): cycle.members(bodies, SOURCE)

    def test_summary_members_stages_types_flags_and_deadline_refuse(self):
        for fault in ("mode", "client", "source", "auth", "model", "mcp", "bool-int", "late", "unknown", "updater", "log-count", "missing", "extra", "skipped", "reordered", "oversize", "duplicate-json"):
            bodies = container()
            value = json.loads(bodies["runtime-attempt.json"])
            if fault == "mode": value["capture_mode"] = "unknown"
            elif fault == "client": value["client"] = "grok"
            elif fault == "source": value["source_identity"]["installer_tree"] = "0" * 40
            elif fault == "auth": value["vendor_authentication_qualified"] = True
            elif fault == "model": value["real_model_qualified"] = True
            elif fault == "mcp": value["qualification"]["mcp-tools-list"] = True
            elif fault == "bool-int": value["qualification"]["mcp-tools-call"] = 0
            elif fault == "late": value["elapsed_ms"] = 150001
            elif fault == "unknown": value["unapproved"] = False
            elif fault == "updater": value["fixture_authored_updater_config"] = "/foreign/settings.json"
            elif fault == "log-count": value["cycle_log_runs"] = 20
            elif fault == "missing": bodies.pop(cycle.MEMBERS[2])
            elif fault == "extra": bodies["unexpected.log"] = b"unknown"
            elif fault == "oversize": bodies[cycle.MEMBERS[1]] = b"x" * ((1 << 20) + 1)
            elif fault == "skipped": aggregate(bodies, 1, lambda rows: rows.pop(1))
            elif fault == "reordered": aggregate(bodies, 0, lambda rows: rows.reverse())
            bodies["runtime-attempt.json"] = raw(value)
            if fault == "duplicate-json": bodies["runtime-attempt.json"] = raw(value).replace(b'"client":', b'"client":"qoder","client":', 1)
            with self.subTest(fault=fault), self.assertRaises(ValueError): cycle.members(bodies, SOURCE)

    def test_source_state_payload_absence_retention_and_noop_bytes_refuse(self):
        for fault in ("source", "caller", "foreign", "registry", "profile-extra", "anonymous-mode", "binding", "view", "digest", "data-allocated", "owner", "missing-path", "missing-receipt", "noop-bytes", "noop-retained-bytes", "state-extra", "receipt"):
            bodies = container()
            def change(rows):
                p = {row["stage"]: row["value"] for row in rows}
                if fault == "source": p["source-a-a-two"]["digest"] = "sha256:" + "0" * 64
                elif fault == "caller": p["caller-a"]["body"] = stream(b"foreign")
                elif fault == "foreign": p["original-user"]["path"] = "/foreign/plugin"
                elif fault == "registry": p["original-registry"]["settings"] = stream(b"{}")
                elif fault == "profile-extra": p["original-profile"]["inventory"]["unknown"] = "directory:20000000700"
                elif fault == "anonymous-mode": p["original-profile"]["inventory"]["installation_id"] = "file:600:36:" + "0" * 64
                elif fault == "binding": p["update-a"]["binding"]["scope"] = "user"
                elif fault == "view": p["update-a"]["delivery"]["view"] = "/foreign/view"
                elif fault == "digest": p["add-a"]["binding"]["native_objects"][0]["managed_digest"] = "sha256:" + "0" * 64
                elif fault == "data-allocated": p["add-b"]["data"]["allocated"] = True
                elif fault == "owner": p["remove-a"]["data"]["owner"] = stream(b"{}")
                elif fault == "missing-path": p["missing-a"]["path"] += "/skills"
                elif fault == "missing-receipt": p["missing-a"]["object"]["path"] = "/foreign"
                elif fault == "noop-bytes": p["repeat-add-a"]["state"] = stream(base64.b64decode(p["repeat-add-a"]["state"]["body"]) + b" ")
                elif fault == "noop-retained-bytes":
                    data = p["repeat-remove-a"]["data"]
                    owner = base64.b64decode(data["owner"]["body"]) + b" "
                    data["owner"] = stream(owner)
                    data["files"][".agentplugins-data-owner.json"] = "file:600:" + cycle.sha(owner)
                elif fault == "state-extra":
                    value = json.loads(base64.b64decode(p["add-a"]["state"]["body"]))
                    value["environment_receipts"] = [dict(unapproved=True)]
                    p["add-a"]["state"] = stream(raw(value))
                else: p["intact-repair-a"]["binding"]["receipts"][-1]["sequence"] = True
            aggregate(bodies, 2, change)
            with self.subTest(fault=fault), self.assertRaises(ValueError): cycle.members(bodies, SOURCE)

    def test_log_overlay_held_stat_manifest_time_identity_and_chain_refuse(self):
        for fault in ("uid", "gid", "mode", "hardlink", "noncanonical", "target-hash", "target-length", "timestamp", "run-pid", "provenance", "argv", "manifest-hash", "log-empty", "before", "after", "delta-count", "delta-path", "extra", "overlap", "ledger"):
            bodies = container()
            def change(rows):
                value = rows[0]["value"]
                log = value["log"]
                if fault in ("uid", "gid", "mode", "hardlink", "noncanonical", "target-hash", "target-length"):
                    pieces = log["latest_record"].split(":")
                    index, replacement = {"uid": (5, "1001"), "gid": (18, "1001"), "mode": (15, "40777"), "hardlink": (4, "2"), "noncanonical": (1, "01"), "target-hash": (25, "0" * 64), "target-length": (24, "1")}[fault]
                    pieces[index] = replacement
                    log["latest_record"] = ":".join(pieces)
                elif fault == "timestamp": log["run"]["started_at"] = "2026-10-27T00:00:00.000Z"
                elif fault == "run-pid": log["run"]["pid"] += 1
                elif fault == "provenance": log["run"]["pid_provenance"] = "supervisor"
                elif fault == "argv": log["run"]["argv_matches"] = not log["run"]["argv_matches"]
                elif fault == "manifest-hash": log["run"]["manifest_sha256"] = "0" * 64
                elif fault == "log-empty": log["log_bytes"] = 0
                elif fault == "before": log["before_sha256"] = "0" * 64
                elif fault == "after": log["after_sha256"] = "0" * 64
                elif fault == "delta-count": log["delta"]["changed"] += 1
                elif fault == "delta-path": log["delta"]["entries"][0]["key_sha256"] = "0" * 64
                elif fault == "extra": log["adopted"] = True
                elif fault == "overlap": value["started_at"] = "2026-10-27T00:00:00Z"
            if fault == "ledger": aggregate(bodies, 2, lambda rows: rows[-1]["value"].update(expected_sha256="0" * 64))
            else: aggregate(bodies, 1, change)
            with self.subTest(fault=fault), self.assertRaises(ValueError): cycle.members(bodies, SOURCE)

    def test_whole_catalog_disabled_controls_scope_and_same_removed_view_refuse(self):
        for fault in ("foreign-missing", "foreign-enabled", "project-path", "resources", "stale-owned", "removed-argv", "extra"):
            bodies = container()
            def change(rows):
                row = rows[0] if fault not in ("stale-owned", "removed-argv") else next(row for row in rows if row["stage"] == "removed-a")
                value = row["value"]
                actual = json.loads(base64.b64decode(value["stdout"]["body"]))
                if fault == "foreign-missing": actual.pop()
                elif fault == "foreign-enabled": actual[0]["enabled"] = True
                elif fault == "project-path": actual[0]["projectPath"] = LAYOUT["b"]
                elif fault == "resources": actual[0]["resources"]["mcpServers"] = [dict(name="foreign")]
                elif fault == "stale-owned": actual.insert(0, dict(name=cycle.NAME))
                elif fault == "removed-argv": value["command"]["argv"][-1] = "/substitute-empty-view"
                else: actual[0]["displayName"] = "unknown"
                value["stdout"] = stream(raw(actual))
            aggregate(bodies, 1, change)
            with self.subTest(fault=fault), self.assertRaises(ValueError): cycle.members(bodies, SOURCE)

    def test_public_scope_source_mutation_and_operation_joins_refuse(self):
        for fault in ("scope", "source", "mutated", "dry-run", "operation", "selected", "retention"):
            bodies = container()
            def change(rows):
                stage = "update-a" if fault == "selected" else "remove-a" if fault == "retention" else "add-a"
                value = next(row["value"] for row in rows if row["stage"] == stage)
                actual = json.loads(base64.b64decode(value["stdout"]["body"]))
                target = actual["data"]["targets"][0]
                if fault == "scope": target["output"]["result"]["scope"] = "user"
                elif fault == "source": target["output"]["tree_digest"] = "sha256:" + "0" * 64
                elif fault == "mutated": target["output"]["result"]["mutated"] = False
                elif fault == "dry-run": actual["data"]["dry_run"] = True
                elif fault == "operation": target["output"]["operation_id"] = "op-" + "0" * 32
                elif fault == "selected": target["selected"] = False
                else: actual["data"]["retained_data"] = []
                value["stdout"] = stream(raw(actual))
            aggregate(bodies, 0, change)
            with self.subTest(fault=fault), self.assertRaises(ValueError): cycle.members(bodies, SOURCE)

    def test_measured_single_noop_and_grouped_lifecycle_sources_do_not_mix(self):
        for fault in ("grouped-repeat", "single-add", "single-operation", "single-nochange", "repeat-source", "repair-source", "remove-source"):
            bodies = container()
            def change(rows):
                stage = "add-a" if fault == "single-add" else "intact-repair-a" if fault == "repair-source" else "remove-a" if fault == "remove-source" else "repeat-add-a"
                value = next(row["value"] for row in rows if row["stage"] == stage)
                actual = json.loads(base64.b64decode(value["stdout"]["body"]))
                if fault == "grouped-repeat":
                    output = actual["data"]
                    operation = "op-" + "0" * 32
                    output["operation_id"] = operation
                    actual["data"] = dict(operation_id=operation, batch=True, status="completed", succeeded=1, failed=0, plugin=cycle.NAME, dry_run=False,
                                          targets=[dict(target="qoder", status="external_completed", output=output)])
                elif fault == "single-add": actual["data"] = actual["data"]["targets"][0]["output"]
                elif fault == "single-operation": actual["data"]["operation_id"] = "op-" + "0" * 32
                elif fault == "single-nochange": actual["data"]["result"]["no_change"] = False
                elif fault == "repeat-source": actual["data"]["source"] = "local"
                else: actual["data"]["targets"][0]["output"]["source"] = "direct local source"
                value["stdout"] = stream(raw(actual))
            aggregate(bodies, 0, change)
            with self.subTest(fault=fault), self.assertRaises(ValueError): cycle.members(bodies, SOURCE)

    def test_raw_encoded_privacy_canonical_base64_and_duplicate_keys_refuse(self):
        for body in (b"uap-qoder-private-inert", b'{"key":"\\u0075ap-qoder-private-inert"}', b'{}\n{"key":"\\u0075ap-qoder-private-inert"}\n'):
            with self.assertRaises(ValueError): cycle.stream(stream(body))
            bodies = container()
            aggregate(bodies, 2, lambda rows: rows[2]["value"].update(body=stream(body)))
            with self.assertRaises(ValueError): cycle.members(bodies, SOURCE)
        for field, value in (("bytes", True), ("body", "Zh=="), ("sha256", "0" * 64), ("extra", False)):
            with self.subTest(field=field), self.assertRaises(ValueError): cycle.stream({**stream(b"f"), field: value})

    def test_exact_selector_mode_dispatch_and_cline_default_marker_preserved(self):
        case = runtime.CASES["qoder"]
        self.assertEqual((case["test"], case["file"], case["seconds"], case["optin"]), ("TestAgentpluginsQoderNativeProjectCompleteSkillsCycle", "repotests/agentplugins_qoder_cycle_lifecycle_test.go", 180, "AGENTPLUGINS_QODER_PROJECT_RUNTIME_ATTEMPT"))
        with self.assertRaises(ValueError): runtime.cli_attempt_members(container(), SOURCE, "grok")
        bodies = container()
        summary_change(bodies, capture_mode="unknown")
        with self.assertRaises(ValueError): runtime.cli_attempt_members(bodies, SOURCE, "qoder")
        self.assertTrue(runtime.cline_cycle_private(b"uap-cline-private-inert"))
        self.assertFalse(runtime.cline_cycle_private(b"uap-qoder-private-inert"))
        self.assertTrue(runtime.cline_cycle_private(b"uap-qoder-private-inert", marker=b"uap-qoder-private-"))


if __name__ == "__main__":
    unittest.main()
