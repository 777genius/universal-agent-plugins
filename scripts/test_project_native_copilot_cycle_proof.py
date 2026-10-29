"""Actual inert production staging: no native execution or qualification.

The fixed export runs the real planner/stager/adapter with inert executables.
Test-only metadata and events-path normalization are explicit below; they do
not alter the production reader's Engine/timestamp/receipt requirements.
"""
import copy
import json
from pathlib import Path
import unittest

import project_native_copilot_cycle as cycle
import project_native_copilot_cycle_proof as proof
from test_project_native_copilot_cycle import encoded, stream

FIXTURE = Path(__file__).resolve().parents[1] / "repotests/testdata/copilot-cycle-inert-staging.jsonl"


def actual():
    raw = FIXTURE.read_bytes()
    if cycle.sha(raw) != "4948429783003def220fc4718df8d17ce257f862ef658e0d6f11a592cf0ae512":
        raise AssertionError("inert production staging export changed")
    return {row["stage"]: row["value"] for row in cycle.rows(raw)}


def prepared(key):
    rows = actual()
    fixture = rows["fixture"]
    value = rows["add-" + key]
    b, native = value["binding"], value["native"]
    active, data = native["active"], value["data"]["path"]
    source = rows["source-" + key + "-" + key + "-one"]
    # The inert Go helper predates the hosted events.jsonl convention. Adapt
    # only that authored path and its affected exact generated hashes here.
    if key != "user":
        config = cycle.load(cycle.stream(native["config"]))
        entry = config["mcpServers"]["uapcopilot" + key]
        entry["env"]["UAP_TEST_EVENTS"] = fixture["layout"]["root"] + "/events.jsonl"
        native["config"] = stream(encoded(config))
        generated = copy.deepcopy(entry)
        del generated["tools"]
        generated = (json.dumps({"mcpServers": {"uapcopilot" + key: generated}}, sort_keys=True, indent=2) + "\n").encode()
        native["managed_files"]["mcp.json"] = "file:644:" + cycle.sha(generated)
        owned = next(item for item in b["native_objects"] if item["kind"] == "copilot_project_mcp_server")
        owned["managed_digest"] = "sha256:" + cycle.sha(("agentplugins-native-mcp-v1\0copilot-mcpServers\0uapcopilot" + key + "\0").encode() + cycle.compact(entry))
    native["objects"] = copy.deepcopy(b["native_objects"])
    # Stager helper has no CLI Engine receipt. This test-only receipt lets the
    # native snapshot function check its join; complete capture admission still
    # requires actual Engine/public/state evidence and is never invoked here.
    b["receipts"] = [{"after_digest": b["native_objects"][0]["managed_digest"]}]
    return rows, value, source, active, data


class CopilotCycleProofContracts(unittest.TestCase):
    def check_native(self, rows, value, source, key, active, data):
        f = rows["fixture"]
        proof.native_snapshot(value["native"], value["binding"], source, key, key + "-one", False,
                              active, data, value["binding"]["physical_artifact_id"], f["layout"], f["nonce"], f["immutable"])

    def test_actual_user_source_and_installed_text_digests(self):
        rows = actual()
        f = rows["fixture"]
        authored = proof.recipe(f["layout"], f["nonce"], "user", "user-one")
        source = rows["source-user-user-one"]
        self.assertEqual(proof.tree(authored), source["files"])
        self.assertEqual(proof.text_digest(authored, source=True), source["digest"])
        args = prepared("user")
        self.check_native(*args[:3], "user", *args[3:])

    def test_actual_project_generated_native_trees_and_data_ownership(self):
        for key in ("a", "b"):
            rows, value, source, active, data = prepared(key)
            self.check_native(rows, value, source, key, active, data)
            ins = cycle.load(cycle.stream(value["state"]))["installations"][0]
            proof.data_snapshot(value["data"], ins, value["binding"], key, False, False, data,
                                value["binding"]["physical_artifact_id"], rows["fixture"]["nonce"])
            # Foreign B remains in the entire selected native skill tree.
            self.assertEqual("uap-foreign-b" in value["native"]["skill_files"], key == "b")

    def test_native_text_membership_binary_and_owned_object_mutations_refuse(self):
        for key in ("user", "a", "b"):
            for fault in ("skill", "extra-managed", "mode", "binary", "object", "foreign", "metadata"):
                rows, value, source, active, data = prepared(key)
                native = value["native"]
                if fault == "skill": native["skill"] = stream(b"foreign")
                elif fault == "extra-managed": native["managed_files"]["foreign"] = "file:644:" + "0" * 64
                elif fault == "mode": native["managed_files"]["."] = "directory:20000000700"
                elif fault == "binary":
                    if key == "user": native["managed_files"]["plugin.json"] = "file:644:" + "0" * 64
                    else: rows["fixture"]["immutable"]["probe"]["sha256"] = "0" * 64
                elif fault == "object": value["binding"]["native_objects"][0]["path"] = "/foreign"
                elif fault == "foreign":
                    if key == "user": native["config_path"] = "/foreign"
                    else: native["skill_files"]["uap-foreign-a"] = "directory:20000000700"
                elif fault == "metadata":
                    if key == "user": native["launcher_metadata"] = stream(b"{}")
                    else: native["launcher_metadata"] = stream(encoded({"protocol": 1, "sha256": "0" * 64, "cliVersion": "inert"}))
                with self.subTest(key=key, fault=fault), self.assertRaises(ValueError):
                    self.check_native(rows, value, source, key, active, data)

    def test_project_data_cannot_adopt_caller_or_stale_owner_or_marker(self):
        for fault in ("path", "receipt", "owner", "extra", "marker", "unallocated", "data-id"):
            rows, value, _, _, data = prepared("a")
            ins = cycle.load(cycle.stream(value["state"]))["installations"][0]
            snapshot = value["data"]
            if fault == "path": snapshot["path"] = data.rsplit("/", 1)[0] + "/caller-owned"
            elif fault == "receipt": snapshot["receipt"]["ownership_digest"] = "sha256:" + "0" * 64
            elif fault == "owner": snapshot["owner"] = stream(b"{}")
            elif fault == "extra": snapshot["files"]["caller"] = "file:600:" + "0" * 64
            elif fault == "marker": snapshot["marker"] = stream(b"unproved native data")
            elif fault == "unallocated": snapshot["allocated"] = False
            else: value["binding"]["data_receipt_id"] = "data_" + "0" * 24
            with self.subTest(fault=fault), self.assertRaises(ValueError):
                proof.data_snapshot(snapshot, ins, value["binding"], "a", False, False, data,
                                    value["binding"]["physical_artifact_id"], rows["fixture"]["nonce"])

    def test_user_skill_only_data_never_inherits_project_runtime_claim(self):
        rows, value, _, _, data = prepared("user")
        ins = cycle.load(cycle.stream(value["state"]))["installations"][0]
        proof.data_snapshot(value["data"], ins, value["binding"], "user", False, False, data,
                            value["binding"]["physical_artifact_id"], rows["fixture"]["nonce"])
        for field, wrong in (("allocated", True), ("marker", stream(b"fake")), ("receipt", {}), ("files", {})):
            bad = copy.deepcopy(value["data"])
            bad[field] = wrong
            with self.assertRaises((ValueError, TypeError, KeyError)):
                proof.data_snapshot(bad, ins, value["binding"], "user", False, False, data,
                                    value["binding"]["physical_artifact_id"], rows["fixture"]["nonce"])

    def test_user_shared_surface_and_project_independence_are_exact(self):
        for key in ("user", "a", "b"):
            rows, value, source, _, _ = prepared(key)
            ins = cycle.load(cycle.stream(value["state"]))["installations"][0]
            b = value["binding"]
            b.update(affected_surfaces=["copilot", "vscode"] if key == "user" else ["copilot"], updated_at="2026-10-28T00:00:00Z")
            proof.binding(b, ins, key, key + "-one", source, rows["fixture"]["layout"])
            for wrong in (["copilot"] if key == "user" else ["copilot", "vscode"], ["vscode"], ["copilot", "foreign"]):
                bad = copy.deepcopy(b)
                bad["affected_surfaces"] = wrong
                with self.subTest(key=key, wrong=wrong), self.assertRaises(ValueError):
                    proof.binding(bad, ins, key, key + "-one", source, rows["fixture"]["layout"])

    def test_engine_state_requires_real_metadata_not_bare_stager_success(self):
        rows, value, source, _, _ = prepared("a")
        # The unmodified inert stager is deliberately not a complete Engine
        # state: empty timestamps/no op-group/affected surfaces are refused.
        with self.assertRaises(ValueError):
            proof.state(value, source, "a", "a-one", rows["fixture"]["layout"])
        state = cycle.load(cycle.stream(value["state"]))
        ins = state["installations"][0]
        b = value["binding"]
        stamp = "2026-10-28T00:00:00Z"
        ins.update(operation_group_id="op-" + "a" * 32, created_at=stamp, updated_at=stamp)
        b.update(affected_surfaces=["copilot"], updated_at=stamp,
                 receipts=[{"operation_id": "op-" + "a" * 32 + "-001", "sequence": 1, "mutation_type": "directory_swap", "client_binding_id": b["client_binding_id"], "active_path": b["target_locator"], "after_digest": b["native_objects"][0]["managed_digest"], "phase": "committed"}])
        ins["clients"] = {b["client_binding_id"]: b}
        value["state"] = stream(encoded(state))
        proof.state(value, source, "a", "a-one", rows["fixture"]["layout"])
        for fault in ("source", "package", "iid", "binding", "scope", "retained", "foreign-client"):
            bad = copy.deepcopy(value)
            state = cycle.load(cycle.stream(bad["state"]))
            ins = state["installations"][0]
            if fault == "source": ins["source"]["canonical_source"] = "/foreign"
            elif fault == "package": ins["package"]["inventory"]["mcp_servers"] = ["foreign"]
            elif fault == "iid": ins["installation_id"] = "01234567-89ab-4cde-8fab-0123456789ab"
            elif fault == "binding": bad["binding"]["physical_artifact_id"] = "native-proof-" + "0" * 12
            elif fault == "scope": bad["binding"]["scope"] = "user"
            elif fault == "retained": ins["data_retained"] = True
            else: ins["clients"]["foreign"] = copy.deepcopy(bad["binding"])
            bad["state"] = stream(encoded(state))
            with self.subTest(fault=fault), self.assertRaises(ValueError):
                proof.state(bad, source, "a", "a-one", rows["fixture"]["layout"])


if __name__ == "__main__":
    unittest.main()
