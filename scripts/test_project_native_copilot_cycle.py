"""Inert semantic controls; generated containers are never native qualification."""
import base64
import copy
import json
from pathlib import Path
import unittest

import project_native_copilot_cycle as cycle
import project_native_copilot_cycle_exchange as exchange
import project_native_copilot_cycle_proof as proof
import project_native_copilot_events as events
import project_native_runtime as runtime
from test_project_native_copilot_readiness import actual

SOURCE = {"source_commit": "a" * 40, "source_tree": "b" * 40}


def encoded(value):
    return json.dumps(value, separators=(",", ":")).encode()


def stream(raw):
    return {"bytes": len(raw), "sha256": cycle.sha(raw), "body": base64.b64encode(raw).decode()}


def prefix():
    return {"mode": cycle.MODE, "client": "copilot", "provider_kind": "scripted_loopback", "activation": "prepared", "vendor_authentication_qualified": False, "real_model_qualified": False,
            "runtime_attempt_complete": False, "test_failed": True, "provider_closed": False, "cycle_observed_complete": False,
            "cycle_public_stages": [], "cycle_registry_stages": [], "cycle_native_stages": [], "cycle_native_sessions": 0, "cycle_probe_sessions": 0,
            "qualification": {key: False for key in cycle.QUALIFICATIONS}, "elapsed_ms": 1}


def measured():
    raw = actual()["native-attempt.stdout.log"]
    rows = [runtime.strict_json(line) for line in raw.splitlines()]
    return rows, events.instant("2026-10-27T18:31:21.7Z"), events.instant("2026-10-27T18:31:21.9Z")


def no_call():
    old, start, end = measured()
    rows = [copy.deepcopy(old[index]) for index in exchange.NO_CALL]
    for item in rows[:-1]:
        if "turnId" in item["data"]:
            item["data"]["turnId"] = "0"
    for index in (7, 8):
        rows[index]["parentId"] = rows[5]["id"]
    return rows, start, end


class CopilotCycleContracts(unittest.TestCase):
    def refused(self, bodies):
        with self.assertRaises(ValueError):
            cycle.members(bodies, SOURCE)

    def test_truthful_failed_prefix_and_unforgeable_flags(self):
        bodies = {"runtime-attempt.json": encoded(prefix())}
        self.assertEqual(runtime.attempt_members(bodies, SOURCE), prefix())
        for name in ("runtime_attempt_complete", "test_failed", "provider_closed", "cycle_observed_complete"):
            for wrong in (None, 0, "false"):
                bad = prefix()
                bad[name] = wrong
                self.refused({"runtime-attempt.json": encoded(bad)})
        for key in cycle.QUALIFICATIONS:
            bad = prefix()
            bad["qualification"][key] = True
            self.refused({"runtime-attempt.json": encoded(bad)})
        for key in ("vendor_authentication_qualified", "real_model_qualified"):
            bad = prefix()
            bad[key] = True
            self.refused({"runtime-attempt.json": encoded(bad)})
        bad = prefix()
        bad.update(test_failed=False, runtime_attempt_complete=True, provider_closed=True, cycle_observed_complete=True,
                   qualification={key: True for key in cycle.QUALIFICATIONS}, cycle_stage="complete-cycle")
        self.refused({"runtime-attempt.json": encoded(bad)})

    def test_closed_members_stages_source_and_triples(self):
        for extra in ({"foreign": b""}, {"git-a.command.json": b"{}"}, {"cycle-proof.jsonl": b"{\"stage\":\"foreign\",\"value\":{}}\n"}):
            self.refused({"runtime-attempt.json": encoded(prefix()), **extra})
        for key, wrong in (("cycle_stage", "foreign"), ("cycle_public_stages", ["version"]), ("cycle_native_stages", ["empty-a"]), ("cycle_native_sessions", True), ("cycle_probe_sessions", 7), ("source_identity", {})):
            bad = prefix()
            bad[key] = wrong
            self.refused({"runtime-attempt.json": encoded(bad)})
        with self.assertRaises(ValueError):
            cycle.rows(b'{"stage":"fixture","stage":"fixture","value":{}}\n')
        with self.assertRaises(ValueError):
            cycle.rows(b'{"stage":"fixture","value":{}}')

    def test_private_raw_and_decoded_stream_not_only_outer_base64(self):
        for raw in (b"uap-copilot-private-canary", b'{"value":"uap-copilot-\\u0070rivate-canary"}', b'"{\\"value\\":\\"uap-copilot-private-canary\\"}"', b"\xff", b"x" * ((1 << 20) + 1)):
            self.assertTrue(cycle.private(raw))
            with self.assertRaises(ValueError):
                cycle.stream(stream(raw))
        for raw in (b"", b"ordinary fixture\n", b'{"safe":"fixture"}'):
            self.assertEqual(cycle.stream(stream(raw)), raw)
        for key, value in (("bytes", True), ("sha256", "0" * 64), ("body", None), ("body", "Zh=="), ("foreign", "value")):
            bad = stream(b"f")
            bad[key] = value
            with self.assertRaises((ValueError, TypeError)):
                cycle.stream(bad)
        # Cline default marker is unchanged; Copilot passes its own marker.
        self.assertTrue(runtime.cline_cycle_private(b"uap-cline-private-control"))
        self.assertFalse(runtime.cline_cycle_private(b"uap-copilot-private-control"))
        bad = prefix()
        raw = encoded({"stage": "installed-user", "value": {"index": 1, "body": stream(b"uap-copilot-private-canary")}}) + b"\n"
        self.refused({"runtime-attempt.json": encoded(bad), "cycle-provider-requests.jsonl": raw})

    def test_finite_provider_closure_and_probe_receipts_on_failure(self):
        value = {"requests": 1, "responses": 1, "error": False, "returned": True, "shutdown": True, "serve_closed": True, "connections_closed": True, "forced": False}
        cycle.framing(cycle.AGGREGATES[5], "installed-user", value)
        for key, wrong in (("forced", 0), ("requests", True), ("responses", 2), ("extra", False)):
            bad = {**value, key: wrong}
            with self.assertRaises(ValueError):
                cycle.framing(cycle.AGGREGATES[5], "installed-user", bad)
        probe = {"before_bytes": 0, "before_sha256": cycle.sha(b""), "after_bytes": 0, "after_sha256": cycle.sha(b""), "delta": stream(b"")}
        self.assertEqual(exchange.probe(probe, b"", None, 0, 1, set(), set(), "id"), b"")
        for key, wrong in (("after_bytes", 1), ("before_sha256", "0" * 64), ("delta", stream(b"unexpected\n"))):
            with self.assertRaises(ValueError):
                exchange.probe({**probe, key: wrong}, b"", None, 0, 1, set(), set(), "id")

    def test_measured_call_startup_partial_order_preserves_legacy_decoder(self):
        rows, start, end = measured()
        original = b"\n".join(map(encoded, rows)) + b"\n"
        events.decode(original, start, end)
        # Native may load tools before its connected notification. Preserve raw
        # order/times/parent graph and canonicalize only semantic startup data.
        rows[1], rows[2] = rows[2], rows[1]
        rows[1]["timestamp"], rows[2]["timestamp"] = rows[2]["timestamp"], rows[1]["timestamp"]
        raw = b"\n".join(map(encoded, rows)) + b"\n"
        _, data, _ = events.decode(raw, start, end, startup=True)
        self.assertEqual(data[1]["status"], "connected")
        with self.assertRaises(ValueError):
            events.decode(raw, start, end)
        for wrong in ("duplicate", "summary-early", "pending-late", "foreign"):
            bad = copy.deepcopy(rows)
            if wrong == "duplicate": bad[2] = copy.deepcopy(bad[1])
            elif wrong == "summary-early": bad[0], bad[5] = bad[5], bad[0]
            elif wrong == "pending-late": bad[0], bad[1] = bad[1], bad[0]
            else: bad[1]["type"] = "unknown.startup"
            with self.assertRaises(ValueError):
                events.decode(b"\n".join(map(encoded, bad)) + b"\n", start, end, startup=True)

    def test_no_call_graph_refuses_tools_extra_frames_and_parent_aliases(self):
        rows, start, end = no_call()
        raw = b"\n".join(map(encoded, rows)) + b"\n"
        exchange.no_call_decode(raw, start, end)
        for wrong in ("extra", "tool", "parent", "duplicate", "time", "usage"):
            bad = copy.deepcopy(rows)
            if wrong == "extra": bad.append(bad[-1])
            elif wrong == "tool": bad[6]["type"] = "tool.execution_start"
            elif wrong == "parent": bad[8]["parentId"] = bad[0]["id"]
            elif wrong == "duplicate": bad[8]["id"] = bad[7]["id"]
            elif wrong == "time": bad[6]["timestamp"] = "2026-01-01T00:00:00Z"
            else: bad[-1]["usage"]["codeChanges"]["filesModified"] = ["foreign"]
            with self.assertRaises(ValueError):
                exchange.no_call_decode(b"\n".join(map(encoded, bad)) + b"\n", start, end)

    def test_no_call_provider_requires_no_declarations_and_exact_history(self):
        request = runtime.strict_json(actual()["provider-request-1.json"])
        transformed = request["messages"][1]["content"]
        del request["tools"]
        exchange.request(encoded(request), "", transformed, None, "call")
        exchange.request(encoded({**request, "tools": []}), "", transformed, None, "call")
        for key, wrong in (("tools", None), ("tools", [{"type": "function"}]), ("tool_choice", "required"), ("stream", False), ("temperature", True), ("messages", request["messages"] + [{"role": "tool", "content": "foreign"}])):
            with self.assertRaises(ValueError):
                exchange.request(encoded({**request, key: wrong}), "", transformed, None, "call")
        delta = {"role": "assistant", "content": "fixture"}
        def chunk(d, f):
            return b"data: " + cycle.compact({"id": "chatcmpl-n", "object": "chat.completion.chunk", "created": 1, "model": "uap-fixture-model", "choices": [{"index": 0, "delta": d, "finish_reason": f}]}) + b"\n\n"
        raw = chunk(delta, None) + chunk({}, "stop") + b"data: [DONE]\n\n"
        exchange.response(raw, "n", delta, "stop")
        for wrong in (raw + b"\n", raw.replace(b"[DONE]", b"[OTHER]"), raw.replace(b"fixture", b"foreign")):
            with self.assertRaises(ValueError): exchange.response(wrong, "n", delta, "stop")

    def test_measured_public_output_joins_scope_source_and_mutation(self):
        bodies = actual()
        summary = runtime.strict_json(bodies["runtime-attempt.json"])
        public = runtime.strict_json(bodies["add-A.stdout.log"])
        fixture_proof = {"scope": "a", "version": "1.0.0", "source_digest": summary["installation_source"]["tree_digest"],
                         "binding": summary["binding"], "installation": {"installation_id": public["data"]["targets"][0]["output"]["result"]["installation_id"],
                         "package": {"manifest_digest": summary["binding"]["package_revision"]["manifest_digest"]}}}
        cycle.public_json(bodies["add-A.stdout.log"], "add-a", fixture_proof)
        for fault in ("scope", "digest", "mutated", "confirmation", "target", "dry-run"):
            bad = copy.deepcopy(public)
            result = bad["data"]["targets"][0]["output"]["result"]
            if fault == "scope": result["plan"]["scope"] = "user"
            elif fault == "digest": bad["data"]["targets"][0]["output"]["tree_digest"] = "sha256:" + "0" * 64
            elif fault == "mutated": result["mutated"] = False
            elif fault == "confirmation": result["requires_confirmation"] = True
            elif fault == "target": bad["data"]["targets"][0]["target"] = "foreign"
            else: bad["data"]["dry_run"] = True
            with self.subTest(fault=fault), self.assertRaises(ValueError):
                cycle.public_json(encoded(bad), "add-a", fixture_proof)

    def test_complete_catalog_preserves_foreign_skills_and_proves_native_absence(self):
        rows, _, _ = measured()
        data = [copy.deepcopy(row["data"]) for row in rows[:-1]]
        root = "/fixture"
        layout = {"root": root, "home": root + "/home", "a": root + "/project", "b": root + "/project-B", "outside": root + "/outside"}
        base = "0" * 32
        n = proof.nonce(base, "a")
        def skill(name, description, source, path, invocable=True):
            return {"name": name, "commandName": name, "description": description, "source": source, "path": path, "userInvocable": invocable, "enabled": True}
        name, description, path, _ = proof.foreign(layout, base, "user")
        original = skill(name, description, "personal-copilot", path)
        user_path = root + "/installer-state/managed/clients/copilot/native-proof-000000000000/skills/uapcopilotuser/SKILL.md"
        user = skill("uapcopilotuser", "Disposable Copilot user user-one " + proof.nonce(base, "user"), "plugin", user_path)
        user["commandName"] = "native-proof:uapcopilotuser"
        managed_path = root + "/project/.github/skills/uapcopilota/SKILL.md"
        managed = skill("uapcopilota", "Disposable Copilot a a-one " + n, "project", managed_path)
        builtins = copy.deepcopy(data[3]["skills"][1:])
        for item in builtins:
            item["path"] = root + "/xdg-cache/copilot/pkg/linux-x64/1.0.103/builtin/" + item["name"] + "/SKILL.md"
        recipes = {"base_nonce": base, "proofs": {"add-user": {"skill_path": user_path}, "after-native-installed-a": {"revision": "a-one", "nonce": n, "skill_path": managed_path}}}
        data[3]["skills"] = [original, user, managed] + builtins
        self.assertEqual(exchange.catalog(data, layout, recipes, "installed-a"), layout["a"])
        for index, field, value in ((1, "commandName", "uapcopilotuser"),
                                    (1, "commandName", "other:uapcopilotuser"),
                                    (1, "commandName", "native-proof:native-proof:uapcopilotuser"),
                                    (1, "name", "native-proof:uapcopilotuser"),
                                    (0, "commandName", "native-proof:uap-foreign-user"),
                                    (2, "commandName", "native-proof:uapcopilota"),
                                    (3, "commandName", "native-proof:" + builtins[0]["name"])):
            bad = copy.deepcopy(data)
            bad[3]["skills"][index][field] = value
            with self.subTest(index=index, field=field, value=value), self.assertRaises(ValueError):
                exchange.catalog(bad, layout, recipes, "installed-a")
        # Final absence retains original foreign User + measured builtins, with
        # zero fixture skills or server declarations in the whole catalog.
        empty = copy.deepcopy(data[3:7])
        empty[0]["skills"] = [original] + builtins
        empty[2]["servers"] = empty[2]["servers"][:-1]
        self.assertEqual(exchange.catalog(empty, layout, recipes, "final-a"), layout["a"])
        for fault in ("foreign-removed", "stale-managed", "foreign-source", "duplicate", "server", "disabled"):
            bad = copy.deepcopy(empty)
            if fault == "foreign-removed": bad[0]["skills"] = builtins
            elif fault == "stale-managed": bad[0]["skills"].append(managed)
            elif fault == "foreign-source": bad[0]["skills"][0]["source"] = "project"
            elif fault == "duplicate": bad[0]["skills"].append(copy.deepcopy(original))
            elif fault == "server": bad[2]["servers"].append({"name": "foreign", "status": "connected"})
            else: bad[1]["message"] = events.DISABLED.replace(", skill", "")
            with self.subTest(fault=fault), self.assertRaises(ValueError):
                exchange.catalog(bad, layout, recipes, "final-a")

    def test_completed_public_and_registry_keep_original_kind_budgets(self):
        layout = {key: "/fixture/" + key for key in ("root", "a", "b", "outside", "installer", "client", "git")}
        argv, cwd = cycle.public_argv("git-a", layout)
        record = {"argv": argv, "cwd": cwd, "exit": 0, "elapsed_ms": 35001, "error": False, "context_error": False, "cleanup": cycle.CLEANUP}
        # Failed framing still records a bounded late child truthfully; the
        # completed public path applies the actual 30s + 5s grace ceiling.
        cycle.command(record)
        with self.assertRaises(ValueError):
            cycle.complete_public({}, {"git-a": record}, layout, {})
        record["elapsed_ms"] = 35000
        receipt = {"command": record, "started_at": "2026-10-28T00:00:00Z", "ended_at": "2026-10-28T00:00:35Z", "stdout": stream(b""), "stderr": stream(b"")}
        cycle.timed(receipt, True, maximum=35000)
        receipt["ended_at"] = "2026-10-28T00:00:35.001Z"
        record["elapsed_ms"] = 35001
        cycle.timed(receipt, True)
        with self.assertRaises(ValueError):
            cycle.timed(receipt, True, maximum=35000)

    def test_exact_scope_commands_and_canonical_selector(self):
        layout = {key: "/fixture/" + key for key in ("root", "home", "a", "b", "outside", "installer", "client", "git", "probe")}
        add, cwd = cycle.public_argv("add-user", layout)
        self.assertEqual(cwd, "/fixture/outside")
        self.assertEqual(add, ["/fixture/installer", "add", "/fixture/root/cycle-source-user", "--scope", "user", "--target", "copilot", "--format", "json", "--no-save"])
        repair, cwd = cycle.public_argv("missing-repair-a", layout)
        self.assertEqual(cwd, "/fixture/a")
        self.assertEqual(repair, ["/fixture/installer", "repair", "native-proof", "--scope", "project", "--target", "copilot", "--format", "json", "--project-root", "/fixture/a"])
        self.assertEqual(runtime.CASES["copilot"]["test"], "TestAgentpluginsCopilotNativeProjectCompleteCycle")
        self.assertEqual(len(cycle.EXACT), 60)
        root = Path(__file__).resolve().parents[1]
        self.assertIn("func " + runtime.CASES["copilot"]["test"] + "(t *testing.T)", (root / runtime.CASES["copilot"]["file"]).read_text())


if __name__ == "__main__":
    unittest.main()
