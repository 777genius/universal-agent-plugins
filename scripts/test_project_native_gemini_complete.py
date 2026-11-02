"""Coherent synthetic Gemini evidence, authored without executing any client.

The old failed fixture supplies only fixed source/state construction primitives.
All public, catalog, provider, native and probe records below are independently
authored protocol data, never captured logs or output from a success validator.
"""
import copy
import json
import unittest

import project_native_gemini_cycle as cycle
import project_native_gemini_complete as complete
import project_native_gemini_complete_protocol as protocol
import project_native_runtime as runtime
from test_project_native_gemini_cycle import raw, record, recipe, rows, sha, state_row, stream

RELOAD = "in a running Gemini CLI session use `/mcp reload` and `/skills reload`, or restart Gemini CLI"
NEXT = RELOAD + "; verify this plugin's authentication requirements before using it"
NAME = "uap-gemini-cycle"
BASE = "2026-10-27T00:00:"


def stamp(second):
    return BASE + f"{second:02d}Z"


def operation(stage):
    return "op-" + f"{cycle.PROOFS.index(stage) + 1:032x}"


def state(stage, r, paths, prior):
    value = state_row(stage, r, paths, prior)
    document = runtime.strict_json(complete.stream(value["state"]))
    binding, item = value["binding"], document["installations"][0]
    mutate = stage.startswith("add-") or stage in ("intact-repair-a", "update-a", "missing-repair-a")
    if mutate:
        row = binding["receipts"][-1]
        row["operation_group_id"] = operation(stage)
        row["operation_id"] = operation(stage) + "-001"
        row["backup_path"] = row["active_path"].rsplit("/", 1)[0] + "/.agentplugins-backup-" + sha(row["operation_id"].encode())[:16]
    if value["removed"]:
        row = document["transaction_receipts"][0]
        group = operation("remove-a" if stage == "repeat-remove-a" else stage)
        row.update(operation_group_id=group, operation_id=group + "-001")
        row["backup_path"] = row["active_path"].rsplit("/", 1)[0] + "/.agentplugins-backup-" + sha(row["operation_id"].encode())[:16]
        item["operation_group_id"] = group
    else:
        item["clients"] = {binding["client_binding_id"]: binding}
        item["operation_group_id"] = binding["receipts"][-1]["operation_group_id"]
    value["state"] = stream(raw(document))
    return value


def public_data(value, r, label, paths):
    """Independent public CLI envelope constructor, not a reader call."""
    b = value["binding"]
    item = runtime.strict_json(complete.stream(value["state"]))["installations"][0]
    user = r["scope"] == "user"
    local = "local" if "repair-" in label or label.startswith("remove-") else "direct local source"
    result = {"installation_id": item["installation_id"], "requires_confirmation": False,
              "mutated": label not in ("repeat-add-a", "unchanged-update-a")}
    output = {"plugin": NAME, "version": "1.0.0", "source": local, "tree_digest": r["digest"],
              "manifest_digest": "sha256:" + r["manifest"]["sha256"], "dry_run": False, "result": result}
    if label.startswith("remove-"):
        result.update(plugin=NAME, client_id="gemini", group_phase="external_completed", affected_surfaces=["gemini"],
                      deactivation={"activation": "not_required", "artifact_removal_allowed": True, "external_removal_complete": True})
    else:
        action = ("agentplugins will install and verify Gemini CLI skills and MCP servers automatically; reload them in a running session or restart Gemini CLI" if user else
                  "project skills and MCP settings can be configured automatically; native discovery, workspace trust and system policy require observation in Gemini CLI")
        plan = {"client_id": "gemini", "scope": "user" if user else "project", "status": "ready", "package_mode": "native",
                "activation": "prepared", "authentication": "not_checked", "policy": "allowed", "verification": "package_validated",
                "physical_artifact_id": b["physical_artifact_id"],
                "components": [{"kind": "skill", "name": r["name"], "support": "native"}, {"kind": "mcp_server", "name": r["name"], "support": "native"}],
                "user_actions": ["verify the plugin's authentication requirements before using it", action], "warnings": ["authentication_not_catalog_verified"]}
        actions = [RELOAD]
        if not user:
            actions.append("Gemini project configuration is installed; native discovery, workspace trust, system policy and MCP initialization remain unobserved; start Gemini CLI in the selected project and inspect skills list and mcp list")
        result.update(plan=plan, activation={"activation": "active" if user else "prepared", "authentication": "not_checked", "policy": "allowed", "verification": "installation_verified", "user_actions": actions})
        if label != "repeat-add-a":
            result["group_phase"] = "external_completed"
        if label in ("repeat-add-a", "unchanged-update-a"):
            result["no_change"] = True
        output["next_action"] = NEXT
    security = {"schema_version": 1, "subject": {"tree_digest": r["digest"], "manifest_digest": output["manifest_digest"]},
                "scanner": {"id": "lintai", "version": "0.1.3"},
                "policy": {"id": "agent-plugin-install", "version": 2, "digest": "sha256:9cf869e299d847d7078aeca01f5a182fcdb0144bbf513c83290459991c79e037"},
                "outcome": "no_blocking_findings", "counts": {"blocking": 0, "warnings": 0, "total": 0}, "scanned_files": 3,
                "report_digest": "sha256:90f5acb6ff3ac64f669be4f4ba6deea7ae5aa3a9954662441c0e5498b8d56598", "evidence_source": "cache" if label == "repeat-add-a" else "local_scan"}
    if label == "repeat-add-a":
        output["security"] = security
        return output
    op = "op-" + f"{100:032x}" if label == "unchanged-update-a" else operation(label)
    output["operation_id"] = op
    target = {"target": "gemini", "status": "external_completed", "output": output}
    if not label.startswith("remove-"):
        target["next_action"] = NEXT
    if label in ("unchanged-update-a", "update-a"):
        target["selected"] = True
    data = {"operation_id": op, "batch": True, "status": "completed", "succeeded": 1, "failed": 0, "plugin": NAME, "dry_run": False, "targets": [target]}
    if label.startswith("add-") or label in ("unchanged-update-a", "update-a"):
        data.update(version="1.0.0", source=local, tree_digest=r["digest"])
    if label.startswith("add-"):
        data.update(action_required=1, manifest_digest=output["manifest_digest"], security=security, environment={"reason": "no_save_requested", "saved": False})
    if label.startswith("remove-"):
        data.update(status="data_retained", plugin_data_preserved=True, data_retained=True,
                    retained_data=[{"data_receipt_id": b["data_receipt_id"], "physical_backend_id": b["physical_artifact_id"], "scope": b["scope"], "state": "owned"}])
        action = "agentplugins remove " + item["installation_id"] + " --purge-data"
        if not user:
            action += " --scope project --project-root " + paths[r["scope"]]
        data["retained_data_action"] = "run `" + action + "` to delete retained plugin data"
    return data


def fixture_phase(stage, views, recipes, bindings, paths):
    # Independently map the finite observed stage to the last committed proof.
    selected = cycle.scope(stage)
    point = "" if stage.startswith("baseline-") else "add-" + selected if stage.startswith("installed-") else {
        "restarted-a": "intact-repair-a", "updated-a": "update-a", "repaired-a": "missing-repair-a",
        "removed-a": "remove-a", "removed-b": "remove-b", "removed-user": "remove-user"}.get(stage, stage)
    view = views[point]
    r = recipes[selected]["A-two" if selected == "a" and stage in ("updated-a", "repaired-a", "removed-a") else {"a": "A-one", "b": "B-one", "user": "U-one"}[selected]]
    advertised = [key for key in dict.fromkeys(("user", selected)) if key in view and not view[key]["removed"]]
    operation_kind = "write" if stage in ("installed-user", "installed-a", "installed-b") else "read" if stage in ("restarted-a", "updated-a", "repaired-a") else "none"
    wire = "uap_cycle_" + sha((stage + r["nonce"]).encode())[:24] if operation_kind != "none" else ""
    tool = "mcp_" + r["name"] + "_inspect_runtime" if wire else ""
    b = bindings[selected]
    active = b["target_locator"]
    physical = b["physical_artifact_id"]
    state_root = paths["root"] + "/installer-state" if selected == "user" else paths[selected] + "/.uap"
    return {"scope": selected, "r": r, "advertised": advertised, "view": view, "operation": operation_kind,
            "tool": tool, "wire": wire, "id": tool + "__" + wire if wire else "",
            "tools": ["mcp_" + bindings[key]["native_objects"][1]["logical_name"] + "_inspect_runtime" for key in advertised],
            "prompt": "Disposable Gemini fixture stage " + stage + ". Call only the preselected installed inspect_runtime tool when requested.",
            "sentinel": "UAP_GEMINI_DONE_" + stage + "_" + r["nonce"],
            "args": {"nonce": r["nonce"], "operation": operation_kind, "marker": "retained-" + r["nonce"]} if wire else {},
            "active": active, "data": state_root + "/plugin-data/" + physical}


def declaration(name):
    properties = {key: {"type": "string"} for key in ("nonce", "operation", "marker")}
    properties["wait_for_previous"] = {"type": "boolean", "description": "Set to true to wait for all previously requested tools in this turn to complete before starting. Set to false (or omit) to run in parallel. Use true when this tool depends on the output of previous tools."}
    return {"name": name, "description": "Inspect only disposable fixture runtime facts.",
            "parametersJsonSchema": {"type": "object", "properties": properties, "required": ["nonce"], "additionalProperties": False}}


def fixture(nonces=None, omit_user_a=False):
    source = {"source_commit": "a" * 40, "source_tree": "b" * 40}
    root = "/inert/gemini-complete"
    paths = {"root": root, "home": root + "/home", "a": root + "/project", "b": root + "/project-B", "outside": root + "/outside",
             "installer": "/inert/bin/agentplugins", "client": "/inert/client/gemini.js", "node": "/inert/node/bin/node", "git": "/inert/bin/git"}
    recipes = {key: {revision: recipe(key, paths, revision) for revision in revisions} for key, revisions in
               (("user", ("U-one",)), ("a", ("A-one", "A-two")), ("b", ("B-one",)))}
    # Reauthor all nonce-bearing recipe bytes before constructing state and
    # protocol evidence. This models aliasing coherently, without editing a
    # retained record or expecting a superficial stream-digest refusal.
    for selected, nonce in (nonces or {}).items():
        for revision, value in recipes[selected].items():
            previous = value["nonce"]
            value["nonce"] = nonce
            skill = complete.stream(value["skill"]).replace(previous.encode(), nonce.encode())
            mcp = runtime.strict_json(complete.stream(value["mcp"]))
            mcp["mcpServers"][value["name"]]["env"]["UAP_TEST_NONCE"] = nonce
            for label, name, body in (("skill", "skills/" + value["name"] + "/SKILL.md", skill), ("mcp", "mcp.json", raw(mcp))):
                value[label] = stream(body)
                value["files"][name] = {"bytes": len(body), "sha256": sha(body), "executable": False}
            value["digest"] = "sha256:" + sha((selected + revision + nonce).encode())
    proof_rows, states, views, current, bindings, by_stage = [], {}, {"": {}}, {}, {}, {}
    for stage in cycle.PROOFS:
        if stage.startswith("source-"):
            _, selected, revision = stage.split("-", 2)
            value = recipes[selected][revision]
        else:
            selected = "a" if stage.startswith("before-") else cycle.scope(stage)
            revision = "A-two" if selected == "a" and cycle.PROOFS.index(stage) >= cycle.PROOFS.index("update-a") else {"a": "A-one", "b": "B-one", "user": "U-one"}[selected]
            value = state(stage, recipes[selected][revision], paths, states.get(selected))
            states[selected] = value
            bindings[selected] = value["binding"]
            current[selected] = value
            by_stage[stage] = (value, recipes[selected][revision])
        proof_rows.append(record(stage, value))
        views[stage] = copy.deepcopy(current)
    result = {"client": "gemini", "provider_kind": "scripted_loopback", "capture_mode": "gemini-complete-cycle", "cycle_capture_review_required": False,
              "cycle_observed_complete": True, "runtime_attempt_complete": True, "test_failed": False, "activation": "prepared",
              "qualification": dict.fromkeys(cycle.QUAL, True), "vendor_authentication_qualified": False, "real_model_qualified": False,
              "source_identity": {"installer_base_commit": source["source_commit"], "installer_tree": source["source_tree"], "installer_source_state": "committed", "installer_patch_sha256": []},
              "cycle_layout": paths, "cycle_public_stages": list(cycle.PUBLIC), "cycle_listing_stages": list(cycle.LISTINGS),
              "cycle_native_stages": list(cycle.NATIVE), "cycle_provider_completed": list(cycle.NATIVE), "attempt_stage": "complete-cycle", "elapsed_ms": 60000,
              "cycle_completion": {"schema_version": 1, "started_at": stamp(0), "work_deadline_at": "2026-10-27T00:02:30Z", "finished_at": "2026-10-27T00:01:00Z",
                                   "work_context_error": False, "original_checks_error": False, "open_authorities": 0, "open_events": False,
                                   "provider": {"shutdown_complete": True, "forced_close": False, "serve_returned": True, "error": False,
                                                "busy": 0, "current_stage": "", "requests": 16, "completed_stages": list(cycle.NATIVE)}},
              "client_source": dict(complete.CLIENT_SOURCE), "scanner": dict(complete.SCANNER), "gemini_detection_link": {"path": root + "/detection-bin/gemini", "target": paths["client"]}}
    archive = [{"name": "gemini.js", "kind": "file", "bytes": 100, "sha256": "41847af2158ced805c4a3e098a05b9a2c26fb1a49b12583009b2eaf6a4059a59"}]
    archive += [{"name": "builtin/" + name + "/SKILL.md", "kind": "file", "bytes": 100, "sha256": digest} for name, digest in complete.BUILTINS.items()]
    node = {"name": "node-v24.21.0-linux-x64/bin/node", "kind": "file", "bytes": 126595440, "sha256": "7fde7b8afa198da66257f42ee2001d874c7355631e6d1579a5fb5ef1f246df4c"}
    result["acquisition"] = {"client": "gemini", "acquisition": {"sha256": "bb5e7e879608bd9b3fe261930d3886fa56aacbf471da1655f2978eb35b705a73", "pin_verified": True, "members": archive},
                             "node": {"sha256": "fd8e59d5a511510f6a298afb548f18c7d2b1be404d8b4a27d94fbe49f56cb2d6", "pin_verified": True, "members": [node]},
                             "runtime_files": [{"path": "/inert/client/" + item["name"], "sha256": item["sha256"]} for item in archive] + [{"path": paths["node"], "sha256": node["sha256"]}]}
    bodies = {"runtime-attempt.json": raw(result)}
    for label in cycle.PUBLIC:
        cwd, argv = cycle.public_command(label, paths)
        stdout, stderr = b"", cycle.progress(label)
        if label == "version":
            stdout = b"0.65.0\n"
        elif label == "repeat-remove-a":
            stderr = b'agentplugins: plugin is not installed for target "gemini" in project scope; no target was changed\n'
        elif not label.startswith("git-"):
            verb = {"repeat-add-a": "add", "unchanged-update-a": "update", "intact-repair-a": "repair", "missing-repair-a": "repair"}.get(label, label.split("-")[0])
            stdout = raw({"schema_version": 1, "command": verb, "result": "success", "data": public_data(*by_stage[label], label, paths)})
        bodies[label + ".command.json"] = raw({"argv": argv, "cwd": cwd, "exit": 1 if label == "repeat-remove-a" else 0, "error": False, "context_error": False, "elapsed_ms": 1, "cleanup": cycle.CLEANUP})
        bodies[label + ".stdout.log"], bodies[label + ".stderr.log"] = stdout, stderr
    aggregates = {name: [] for name in cycle.AGGREGATES}
    aggregates["cycle-proof.jsonl"] = proof_rows
    pid = 100
    for index, observation in enumerate(cycle.OBSERVATIONS):
        kind, stage = observation.split(":", 1)
        q = fixture_phase(stage, views, recipes, bindings, paths)
        begin, finish = stamp(1 + index * 2), stamp(2 + index * 2)
        probes, selected_facts = [], None
        for key in q["advertised"]:
            if omit_user_a and q["scope"] == "a" and key == "user" and "a" in q["advertised"]:
                continue
            pid += 1
            session = "synthetic-probe-" + str(pid)
            selected = kind == "native" and q["operation"] != "none" and key == q["scope"]
            r = recipes[key]["A-two" if key == "a" and stage in ("updated-a", "repaired-a", "removed-a") else {"a": "A-one", "b": "B-one", "user": "U-one"}[key]]
            methods = ["initialize", "tools/list"] + (["tools/call"] if selected else [])
            for method in methods:
                value = None
                if method == "tools/call":
                    value = {"nonce": r["nonce"], "revision": r["revision"], "cwd": q["active"], "argv": ["argument with spaces", q["active"] + "/argument", "${UNKNOWN}"],
                             "plugin_root": q["active"], "plugin_data": q["data"], "literal": "literal  " + paths["home"], "once": q["active"] + "/", "marker": "retained-" + r["nonce"], "pid": pid, "session": session}
                    selected_facts = value
                probes.append({"method": method, "pid": pid, "session": session, "nonce": r["nonce"], "timestamp": begin, "facts": value})
        aggregates["cycle-probe-events.jsonl"].append(record(stage if kind == "native" else observation, {"scope": q["scope"], "stream": stream(rows(probes))}))
        if kind == "listing":
            catalog = "Discovered Agent Skills:\n"
            for name, description in protocol.DESCRIPTIONS.items():
                catalog += name + " [Enabled] [Built-in]\n  Description: " + description + "\n  Location:    /inert/client/builtin/" + name + "/SKILL.md\n"
            for key in dict.fromkeys(("user", q["scope"])):
                profile = paths["home"] + "/.gemini" if key == "user" else paths[key] + "/.gemini"
                catalog += "foreign-" + key + " [Disabled]\n  Description: Foreign inert marker\n  Location:    " + profile + "/skills/foreign-" + key + "/SKILL.md\n"
                if key in q["advertised"]:
                    r = recipes[key]["A-two" if key == "a" and stage in ("updated-a", "repaired-a", "removed-a") else {"a": "A-one", "b": "B-one", "user": "U-one"}[key]]
                    catalog += r["name"] + " [Enabled]\n  Description: " + r["revision"] + "\n  Location:    " + profile + "/skills/" + r["name"] + "/SKILL.md\n"
            stdout = catalog.encode()
            argv = [paths["node"], paths["client"], "skills", "list", "--all"]
            aggregates["cycle-discovery.jsonl"].append(record(stage, {"scope": q["scope"], "stdout": stream(stdout), "stderr": stream(b"")}))
        else:
            count = 2 if q["operation"] != "none" else 1
            initial = {"contents": [{"role": "user", "parts": [{"text": q["prompt"]}]}], "tools": [{"functionDeclarations": [declaration(name) for name in q["tools"]]}]}
            requests = [initial]
            if count == 2:
                second = copy.deepcopy(initial)
                second["contents"] += [{"role": "model", "parts": [{"functionCall": {"id": q["wire"], "name": q["tool"], "args": q["args"]}, "thoughtSignature": "skip_thought_signature_validator"}]},
                                       {"role": "user", "parts": [{"functionResponse": {"id": q["wire"], "name": q["tool"], "response": {"output": "<untrusted_context>\n" + raw(selected_facts).decode() + "\n</untrusted_context>"}}}]}]
                requests.append(second)
            for request_index, body in enumerate(requests):
                aggregates["cycle-provider-requests.jsonl"].append(record(stage, {"index": request_index, "stream": stream(raw(body))}))
                part = {"functionCall": {"id": q["id"], "name": q["tool"], "args": q["args"]}} if count == 2 and request_index == 0 else {"text": q["sentinel"]}
                response = {"candidates": [{"index": 0, "content": {"role": "model", "parts": [part]}, "finishReason": "STOP"}], "usageMetadata": {"promptTokenCount": 1, "candidatesTokenCount": 1, "totalTokenCount": 2}}
                aggregates["cycle-provider-responses.jsonl"].append(record(stage, {"index": request_index, "stream": stream(b"data: " + raw(response) + b"\n\n")}))
            aggregates["cycle-provider-receipts.jsonl"].append(record(stage, {"scope": q["scope"], "operation": q["operation"], "tools": q["tools"], "native_call_id": q["id"], "wire_call_id": q["wire"], "prompt": q["prompt"], "sentinel": q["sentinel"], "requests": count, "responses": count, "issued": count == 2, "returned": count == 2, "error": False}))
            native_session = "synthetic-native-" + stage
            events = [{"type": "init", "timestamp": begin, "session_id": native_session, "model": "gemini-2.5-flash"}, {"type": "message", "timestamp": begin, "role": "user", "content": q["prompt"]}]
            if count == 2:
                events += [{"type": "tool_use", "timestamp": begin, "tool_name": q["tool"], "tool_id": q["id"], "parameters": q["args"]}, {"type": "tool_result", "timestamp": begin, "tool_id": q["id"], "status": "success", "output": raw(selected_facts).decode()}]
            stat = {key: 1 for key in ("total_tokens", "input_tokens", "output_tokens", "cached", "input")}
            events += [{"type": "message", "timestamp": finish, "role": "assistant", "content": q["sentinel"]}, {"type": "result", "timestamp": finish, "status": "success", "stats": {**stat, "duration_ms": 1000, "tool_calls": count - 1, "models": {"gemini-2.5-flash": stat}}}]
            stdout = rows(events)
            aggregates["cycle-native-events.jsonl"].append(record(stage, {"scope": q["scope"], "session": native_session, "stream": stream(stdout)}))
            argv = [paths["node"], paths["client"], "--prompt", q["prompt"], "--approval-mode", "default", "--output-format", "stream-json", "--model", "gemini-2.5-flash"]
        command = {"kind": kind, "scope": q["scope"], "argv": argv, "cwd": paths["outside" if q["scope"] == "user" else q["scope"]], "exit": 0, "error": False, "context_error": False,
                   "started_at": begin, "finished_at": finish, "elapsed_ms": 1000, "cleanup": cycle.CLEANUP, "stdout": stream(stdout), "stderr": stream(b"")}
        aggregates["cycle-native-commands.jsonl"].append(record(observation, command))
    bodies.update({name: rows(value) for name, value in aggregates.items()})
    return bodies, source


def aggregate(bodies, name, change):
    values = [runtime.strict_json(line) for line in bodies[name].splitlines()]
    change(values)
    bodies[name] = rows(values)


def provider_edit(bodies, stage, index, change, response=False):
    name = "cycle-provider-responses.jsonl" if response else "cycle-provider-requests.jsonl"

    def edit(values):
        row = next(v["value"] for v in values if v["stage"] == stage and v["value"]["index"] == index)
        body = complete.stream(row["stream"])
        value = runtime.strict_json(body[6:-2] if response else body)
        change(value)
        body = b"data: " + raw(value) + b"\n\n" if response else raw(value)
        row["stream"] = stream(body)

    aggregate(bodies, name, edit)


def native_edit(bodies, stage, change):
    events = [runtime.strict_json(line) for line in complete.stream(next(v["value"]["stream"] for v in
              [runtime.strict_json(line) for line in bodies["cycle-native-events.jsonl"].splitlines()] if v["stage"] == stage)).splitlines()]
    change(events)
    raw_events = stream(rows(events))
    aggregate(bodies, "cycle-native-events.jsonl", lambda values: next(v["value"] for v in values if v["stage"] == stage).update(stream=raw_events))
    aggregate(bodies, "cycle-native-commands.jsonl", lambda values: next(v["value"] for v in values if v["stage"] == "native:" + stage).update(stdout=raw_events))


def catalog_edit(bodies, stage, change):
    values = [runtime.strict_json(line) for line in bodies["cycle-discovery.jsonl"].splitlines()]
    row = next(v["value"] for v in values if v["stage"] == stage)
    modified = stream(change(complete.stream(row["stdout"])))
    row["stdout"] = modified
    bodies["cycle-discovery.jsonl"] = rows(values)
    aggregate(bodies, "cycle-native-commands.jsonl", lambda commands: next(v["value"] for v in commands if v["stage"] == "listing:" + stage).update(stdout=modified))


def proof_edit(bodies, stage, change):
    def edit(values):
        value = next(v["value"] for v in values if v["stage"] == stage)
        document = runtime.strict_json(complete.stream(value["state"]))
        change(document, value)
        if document["installations"][0]["clients"]:
            document["installations"][0]["clients"] = {value["binding"]["client_binding_id"]: value["binding"]}
        value["state"] = stream(raw(document))

    aggregate(bodies, "cycle-proof.jsonl", edit)


class GeminiCompleteTests(unittest.TestCase):
    def admit(self, bodies, source):
        return runtime.attempt_members(bodies, source, "gemini")

    def refuses(self, change):
        bodies, source = fixture()
        change(bodies, source)
        with self.assertRaises(ValueError):
            self.admit(bodies, source)

    def test_full_coherent_independent57_member_fixture_and_read_only_result(self):
        bodies, source = fixture()
        before = copy.deepcopy(bodies), copy.deepcopy(source)
        result = self.admit(bodies, source)
        self.assertEqual(len(bodies), 57)
        self.assertTrue(result["runtime_attempt_complete"])
        self.assertTrue(all(result["qualification"].values()))
        self.assertFalse(result["vendor_authentication_qualified"])
        self.assertFalse(result["real_model_qualified"])
        self.assertEqual((bodies, source), before)

    def test_distinct_source_nonces_prevent_advertised_probe_alias_collapse(self):
        for nonces in ({"user": "2" * 32}, {"user": "3" * 32}, {"a": "3" * 32}):
            bodies, source = fixture(nonces=nonces)
            with self.subTest(alias=nonces), self.assertRaises(ValueError):
                self.admit(bodies, source)
        full, _source = fixture(nonces={"user": "2" * 32})
        missing, source = fixture(nonces={"user": "2" * 32}, omit_user_a=True)
        def event_count(bodies):
            return sum(len(complete.stream(runtime.strict_json(line)["value"]["stream"]).splitlines())
                       for line in bodies["cycle-probe-events.jsonl"].splitlines())
        self.assertEqual(len(missing), 57)
        self.assertEqual(event_count(full) - event_count(missing), 20)
        with self.assertRaises(ValueError):
            self.admit(missing, source)

    def test_closed_ordinary_final_receipt_and_original_clock(self):
        mutations = {
            "missing": lambda r: r.pop("cycle_completion"),
            "version-bool": lambda r: r["cycle_completion"].update(schema_version=True),
            "renewed-origin": lambda r: r["cycle_completion"].update(work_deadline_at="2026-10-27T00:02:31Z"),
            "expired": lambda r: r["cycle_completion"].update(finished_at="2026-10-27T00:02:31Z"),
            "reversed": lambda r: r["cycle_completion"].update(finished_at="2026-10-26T23:59:59Z"),
            "context": lambda r: r["cycle_completion"].update(work_context_error=True),
            "checks": lambda r: r["cycle_completion"].update(original_checks_error=True),
            "authority": lambda r: r["cycle_completion"].update(open_authorities=1),
            "authority-bool": lambda r: r["cycle_completion"].update(open_authorities=False),
            "events": lambda r: r["cycle_completion"].update(open_events=True),
            "foreign": lambda r: r["cycle_completion"].update(assumed=True),
            "total": lambda r: r.update(elapsed_ms=180001),
            "duration": lambda r: r.update(elapsed_ms=59999),
            "duration-float": lambda r: r.update(elapsed_ms=60000.0),
        }
        for key, value in (("shutdown_complete", False), ("forced_close", True), ("serve_returned", False), ("error", True),
                           ("busy", 1), ("busy", False), ("current_stage", "removed-user"), ("requests", 17), ("requests", 16.0),
                           ("completed_stages", list(cycle.NATIVE[:-1]))):
            mutations["provider-" + key + "-" + str(value)] = lambda r, k=key, v=value: r["cycle_completion"]["provider"].update({k: v})
        for name, change in mutations.items():
            def edit(bodies, _source):
                result = runtime.strict_json(bodies["runtime-attempt.json"])
                change(result)
                bodies["runtime-attempt.json"] = raw(result)
            with self.subTest(name=name):
                self.refuses(edit)

    def test_exact_member_source_pin_mode_and_success_tuple(self):
        mutations = {
            "mode": lambda r: r.update(capture_mode="unknown"),
            "failed": lambda r: r.update(test_failed=True),
            "incomplete": lambda r: r.update(runtime_attempt_complete=False),
            "review": lambda r: r.update(cycle_capture_review_required=True),
            "observed": lambda r: r.update(cycle_observed_complete=False),
            "auth": lambda r: r.update(vendor_authentication_qualified=True),
            "model": lambda r: r.update(real_model_qualified=True),
            "integer-flag": lambda r: r["qualification"].update({"project-lifecycle": 1}),
            "partial-flag": lambda r: r["qualification"].update({"mcp-tools-call": False}),
            "source": lambda r: r["source_identity"].update(installer_tree="c" * 40),
            "layout": lambda r: r["cycle_layout"].update(a="/inert/other"),
            "backslash": lambda r: r["cycle_layout"].update(installer="/inert/other\\exe"),
            "pin": lambda r: r["acquisition"]["acquisition"].update(sha256="0" * 64),
            "node": lambda r: r["acquisition"]["node"]["members"][0].update(bytes=1),
            "builtin-hash": lambda r: r["acquisition"]["runtime_files"][1].update(sha256="0" * 64),
            "source-blob": lambda r: r["client_source"].update(tools_blob="c" * 40),
            "scanner": lambda r: r["scanner"].update(binary_sha256="0" * 64),
            "detection": lambda r: r["gemini_detection_link"].update(target="/inert/other"),
            "partial-stage": lambda r: r["cycle_native_stages"].pop(),
        }
        for name, change in mutations.items():
            def edit(bodies, _source):
                value = runtime.strict_json(bodies["runtime-attempt.json"])
                change(value)
                bodies["runtime-attempt.json"] = raw(value)
            with self.subTest(name=name):
                self.refuses(edit)
        self.refuses(lambda b, _s: b.pop("remove-user.stderr.log"))
        self.refuses(lambda b, _s: b.update({"extra.log": b""}))
        for member in cycle.AGGREGATES:
            with self.subTest(member=member):
                self.refuses(lambda b, _s, name=member: aggregate(b, name, lambda values: values.pop()))
        self.refuses(lambda b, _s: b.update({"git-a.command.json": b'{"argv":[],"argv":[]}'}))
        self.refuses(lambda b, _s: b.update({"git-a.stderr.log": b'\xff'}))
        self.refuses(lambda b, _s: b.update({"git-a.stderr.log": raw({"nested": '{"secret":"\\u0075ap-gemini-private-test"}'})}))

    def test_public_and_consistently_rehashed_state_semantic_refusals(self):
        for key, value in (("batch", False), ("succeeded", True), ("operation_id", "op-" + "0" * 32), ("tree_digest", "sha256:" + "0" * 64)):
            def edit(bodies, _source, k=key, val=value):
                output = runtime.strict_json(bodies["add-a.stdout.log"])
                output["data"][k] = val
                bodies["add-a.stdout.log"] = raw(output)
            with self.subTest(public=key):
                self.refuses(edit)
        self.refuses(lambda b, _s: b.update({"repeat-remove-a.stdout.log": b"{}"}))
        mutations = {
            "group": lambda d, v: d["installations"][0].update(operation_group_id="foreign"),
            "created": lambda d, v: d["installations"][0].update(created_at="2026-10-26T23:59:59Z"),
            "updated": lambda d, v: d["installations"][0].update(updated_at="2026-10-27T00:01:01Z"),
            "source": lambda d, v: d["installations"][0]["source"].update(canonical_source="/inert/foreign"),
            "data": lambda d, v: next(iter(d["installations"][0]["data_receipts"].values())).update(locator="/inert/foreign"),
            "dormant-env": lambda d, v: d.update(environment_receipts=[]),
            "dormant-retention": lambda d, v: d["installations"][0].update(data_retained=False),
            "inventory-type": lambda d, v: d["installations"][0]["package"]["inventory"].update(mcp_enabled=1),
            "binding": lambda d, v: v["binding"].update(activation="active"),
            "object": lambda d, v: v["binding"]["native_objects"][0].update(managed_digest="sha256:" + "0" * 64),
            "receipt-type": lambda d, v: v["binding"]["receipts"][0].update(sequence=True),
            "receipt-empty": lambda d, v: v["binding"]["receipts"][0].update(before_digest=""),
            "receipt-phase": lambda d, v: v["binding"]["receipts"][0].update(phase="prepared"),
            "receipt-path": lambda d, v: v["binding"]["receipts"][0].update(staging_path="/inert/foreign"),
            "receipt-extra": lambda d, v: v["binding"]["receipts"].append(copy.deepcopy(v["binding"]["receipts"][0])),
        }
        for name, change in mutations.items():
            with self.subTest(state=name):
                self.refuses(lambda b, _s, fn=change: proof_edit(b, "add-a", fn))
        # No-change requires the full original bytes, including clocks.
        self.refuses(lambda b, _s: proof_edit(b, "repeat-add-a", lambda d, v: d["installations"][0].update(updated_at=stamp(1))))
        self.refuses(lambda b, _s: proof_edit(b, "remove-a", lambda d, v: v["binding"].update(updated_at=stamp(1))))

    def test_every_catalog_rejects_rehashed_foreign_builtin_and_owned_drift(self):
        for stage in cycle.LISTINGS:
            with self.subTest(stage=stage):
                self.refuses(lambda b, _s, key=stage: catalog_edit(b, key, lambda body: body.replace(b"foreign-user [Disabled]", b"foreign-user [Enabled]")))
        self.refuses(lambda b, _s: catalog_edit(b, "updated-a", lambda body: body.replace(b"Description: A-two", b"Description: A-one")))
        self.refuses(lambda b, _s: catalog_edit(b, "installed-a", lambda body: body.replace(b"uapgeminia [Enabled]", b"uapgeminia [Disabled]")))
        self.refuses(lambda b, _s: catalog_edit(b, "removed-a", lambda body: body + b"uapgeminia [Enabled]\n  Description: A-two\n  Location:    /inert/foreign\n"))
        self.refuses(lambda b, _s: catalog_edit(b, "baseline-a", lambda body: body.replace(b"[Built-in]", b"", 1)))

    def test_catalog_whitespace_matches_preserved_go_unicode_white_space(self):
        bodies, source = fixture()
        legitimate = "\u0085\u00a0\u2003\u3000".encode()
        catalog_edit(bodies, "baseline-a", lambda body: legitimate + body.replace(b"Skills:\n", b"Skills:\n" + "\u2003".encode() + b"\n") + legitimate)
        self.assertTrue(self.admit(bodies, source)["runtime_attempt_complete"])
        for control in (b"\x1c", b"\x1d", b"\x1e", b"\x1f"):
            with self.subTest(control=control):
                self.refuses(lambda b, _s, prefix=control: catalog_edit(b, "baseline-a", lambda body: prefix + body))

    def test_provider_rejects_closed_nested_schema_history_and_exact_sse_drift(self):
        changes = {
            "top": lambda r: r.update(unknown=True),
            "role": lambda r: r["contents"][0].update(role="system"),
            "part": lambda r: r["contents"][0]["parts"][0].update(unknown=True),
            "prompt": lambda r: r["contents"][0]["parts"][0].update(text="foreign prompt"),
            "declaration": lambda r: r["tools"][0]["functionDeclarations"][0].update(description="foreign"),
            "schema": lambda r: r["tools"][0]["functionDeclarations"][0]["parametersJsonSchema"]["properties"]["wait_for_previous"].update(description="foreign"),
            "generation-bool": lambda r: r.update(generationConfig={"candidateCount": True}),
            "generation-float": lambda r: r.update(generationConfig={"candidateCount": 1.0}),
            "generation-overflow": lambda r: r.update(generationConfig={"temperature": 10 ** 400}),
            "thinking": lambda r: r.update(generationConfig={"thinkingConfig": {"includeThoughts": 0}}),
            "tool-config": lambda r: r.update(toolConfig={"functionCallingConfig": {"mode": "ANY", "allowedFunctionNames": ["foreign"]}}),
            "safety": lambda r: r.update(safetySettings=[{"category": "unknown", "threshold": "OFF"}]),
            "system": lambda r: r.update(systemInstruction={"parts": [{"inlineData": {}}]}),
            "cached": lambda r: r.update(cachedContent="x\0y"),
        }
        for name, change in changes.items():
            with self.subTest(request=name):
                self.refuses(lambda b, _s, fn=change: provider_edit(b, "installed-a", 0, fn))
        later = {
            "signature": lambda r: r["contents"][-2]["parts"][0].update(thoughtSignature="foreign"),
            "wire-id": lambda r: r["contents"][-2]["parts"][0]["functionCall"].update(id="foreign"),
            "response-role": lambda r: r["contents"][-1].update(role="model"),
            "wrapper": lambda r: r["contents"][-1]["parts"][0]["functionResponse"]["response"].update(output="{}"),
            "envelope": lambda r: r.update(generationConfig={"temperature": 1}),
            "extra-turn": lambda r: r["contents"].append({"role": "user", "parts": [{"text": "foreign"}]}),
        }
        for name, change in later.items():
            with self.subTest(history=name):
                self.refuses(lambda b, _s, fn=change: provider_edit(b, "installed-a", 1, fn))
        for key, value in (("finishReason", "MAX_TOKENS"), ("index", True)):
            with self.subTest(response=key):
                self.refuses(lambda b, _s, k=key, v=value: provider_edit(b, "installed-a", 0, lambda r: r["candidates"][0].update({k: v}), True))
        self.refuses(lambda b, _s: provider_edit(b, "installed-a", 0, lambda r: r["candidates"].append(copy.deepcopy(r["candidates"][0])), True))

    def test_provider_envelope_retains_numeric_lexemes_without_banning_floats(self):
        def pair(first, second):
            bodies, source = fixture()
            for index in (0, 1):
                provider_edit(bodies, "installed-a", index, lambda value: value.update(generationConfig={"temperature": 0.5}))
            def change(values):
                for row in values:
                    if row["stage"] != "installed-a":
                        continue
                    value = row["value"]
                    body = complete.stream(value["stream"])
                    self.assertEqual(body.count(b'"temperature":0.5'), 1)
                    token = first if value["index"] == 0 else second
                    value["stream"] = stream(body.replace(b'"temperature":0.5', b'"temperature":' + token))
            aggregate(bodies, "cycle-provider-requests.jsonl", change)
            return bodies, source
        for token in (b"0.5", b"5e-1", b"0.50", b"-0.0"):
            bodies, source = pair(token, token)
            with self.subTest(equal_float=token):
                self.assertTrue(self.admit(bodies, source)["runtime_attempt_complete"])
        for first, second in ((b"0.5", b"5e-1"), (b"0.5", b"0.50"), (b"-0.0", b"0.0")):
            bodies, source = pair(first, second)
            with self.subTest(lexical_drift=(first, second)), self.assertRaises(ValueError):
                self.admit(bodies, source)

    def test_native_and_probe_rehashed_semantics_require_the_same_facts_and_fresh_processes(self):
        changes = {
            "prompt": lambda e: e[1].update(content="foreign"),
            "model": lambda e: e[0].update(model="foreign"),
            "native-id": lambda e: e[2].update(tool_id="foreign"),
            "args": lambda e: e[2]["parameters"].update(marker="foreign"),
            "result": lambda e: e[3].update(status="error"),
            "facts": lambda e: e[3].update(output="{}"),
            "sentinel": lambda e: e[-2].update(content="foreign"),
            "terminal": lambda e: e[-1].update(status="error"),
            "stats": lambda e: e[-1]["stats"].update(tool_calls=True),
            "stats-model": lambda e: e[-1]["stats"].update(models={"foreign": {}}),
            "missing-end": lambda e: e.pop(),
            "duplicate-call": lambda e: e.insert(3, copy.deepcopy(e[2])),
            "timestamp": lambda e: e[2].update(timestamp="2026-10-27T00:01:01Z"),
        }
        for name, change in changes.items():
            with self.subTest(native=name):
                self.refuses(lambda b, _s, fn=change: native_edit(b, "installed-a", fn))
        for key, value in (("pid", True), ("session", ""), ("method", "tools/call"), ("nonce", "foreign")):
            def edit(bodies, _source, k=key, val=value):
                def change(values):
                    row = next(v["value"] for v in values if v["stage"] == "installed-a")
                    events = [runtime.strict_json(line) for line in complete.stream(row["stream"]).splitlines()]
                    events[0][k] = val
                    row["stream"] = stream(rows(events))
                aggregate(bodies, "cycle-probe-events.jsonl", change)
            with self.subTest(probe=key):
                self.refuses(edit)
        for key in ("pid", "session"):
            def collision(bodies, _source, field=key):
                def change(values):
                    earlier = next(v["value"] for v in values if v["stage"] == "installed-user")
                    reused = runtime.strict_json(complete.stream(earlier["stream"]).splitlines()[0])[field]
                    later = next(v["value"] for v in values if v["stage"] == "installed-a")
                    events = [runtime.strict_json(line) for line in complete.stream(later["stream"]).splitlines()]
                    for event in events:
                        if event["nonce"] == "1" * 32:
                            event[field] = reused
                    later["stream"] = stream(rows(events))
                aggregate(bodies, "cycle-probe-events.jsonl", change)
            with self.subTest(collision=key):
                self.refuses(collision)
        def provider_facts(bodies, _source):
            def change(value):
                output = value["contents"][-1]["parts"][0]["functionResponse"]["response"]
                facts_value = runtime.strict_json(output["output"][len("<untrusted_context>\n"):-len("\n</untrusted_context>")].encode())
                facts_value["marker"] = "foreign"
                output["output"] = "<untrusted_context>\n" + raw(facts_value).decode() + "\n</untrusted_context>"
            provider_edit(bodies, "updated-a", 1, change)
        self.refuses(provider_facts)

    def test_nanosecond_floor_is_exact_and_failed_fixture_cannot_be_promoted(self):
        bodies, source = fixture()
        def boundary(values):
            values[0]["value"].update(started_at="2026-10-27T00:00:01.000000999Z", finished_at="2026-10-27T00:00:01.001000000Z", elapsed_ms=0)
        aggregate(bodies, "cycle-native-commands.jsonl", boundary)
        self.admit(bodies, source)
        aggregate(bodies, "cycle-native-commands.jsonl", lambda values: values[0]["value"].update(elapsed_ms=1))
        with self.assertRaises(ValueError):
            self.admit(bodies, source)
        from test_project_native_gemini_cycle import fixture as failed_fixture
        failed, failed_source = failed_fixture()
        failed["runtime-attempt.json"] = fixture()[0]["runtime-attempt.json"]
        with self.assertRaises(ValueError):
            self.admit(failed, failed_source)

    def test_exact_ascii_rfc3339_lexical_clock_bounds(self):
        base = complete.instant("2026-10-27T00:00:00Z")
        for value in ("2026-10-27T00:00:00.0Z", "2026-10-27T00:00:00.000000000Z",
                      "2026-10-27T00:00:00+00:00", "2026-10-27T23:59:00+23:59", "2026-10-26T00:01:00-23:59"):
            with self.subTest(valid=value):
                self.assertEqual(complete.instant(value), base)
        self.assertEqual(complete.instant("2026-10-27T00:00:00.000000001Z"), base + 1)
        complete.instant("0001-01-01T00:00:00Z")
        complete.instant("9999-12-31T23:59:59.999999999Z")
        for value in ("0000-01-01T00:00:00Z", "2026-10-27T00:00:00,0Z", "2026-10-27T00:00:00.0000000000Z",
                      "2026-10-27T00:00:00.٠Z", "٢٠٢٦-10-27T00:00:00Z", "2026-10-27T00:00:00+24:00",
                      "2026-10-27T00:00:00-24:00", "2026-10-27T00:00:00+00:60", "2026-10-27T00:00:00+00:99",
                      "2026-10-27T00:00:00-00:99", "2026-10-27T24:00:00Z", "2026-02-30T00:00:00Z"):
            with self.subTest(invalid=value), self.assertRaises(ValueError):
                complete.instant(value)

    def test_coherent_timestamp_aliases_refuse_at_each_retained_clock_layer(self):
        for value in ("2026-10-27T00:00:00.٠Z", "2026-10-27T00:00:00,0Z", "2026-10-27T00:00:00.0000000000Z",
                      "2026-10-27T01:00:00+00:60", "2026-10-27T01:39:00+00:99", "2026-10-28T00:00:00+24:00"):
            def closure(bodies, _source, invalid=value):
                result = runtime.strict_json(bodies["runtime-attempt.json"])
                result["cycle_completion"]["started_at"] = invalid
                bodies["runtime-attempt.json"] = raw(result)
            with self.subTest(closure=value):
                self.refuses(closure)
        bodies, source = fixture()
        value = runtime.strict_json(bodies["runtime-attempt.json"])
        value["cycle_completion"]["started_at"] = "2026-10-27T23:59:00+23:59"
        bodies["runtime-attempt.json"] = raw(value)
        self.assertTrue(self.admit(bodies, source)["runtime_attempt_complete"])
        self.refuses(lambda b, _s: aggregate(b, "cycle-native-commands.jsonl", lambda values: values[0]["value"].update(started_at="2026-10-27T00:00:01.٠Z")))
        self.refuses(lambda b, _s: native_edit(b, "installed-a", lambda values: values[0].update(timestamp=values[0]["timestamp"].removesuffix("Z") + ".٠Z")))
        def state_clocks(bodies, _source):
            def change(values):
                for row in values:
                    value = row["value"]
                    if "state" not in value:
                        continue
                    document = runtime.strict_json(complete.stream(value["state"]))
                    item = document["installations"][0]
                    item["created_at"] = "2026-10-27T01:00:00+00:60"
                    value["state"] = stream(raw(document))
            aggregate(bodies, "cycle-proof.jsonl", change)
        self.refuses(state_clocks)
        def probe_clock(bodies, _source):
            def change(values):
                value = next(row["value"] for row in values if row["stage"] == "installed-user")
                events = [runtime.strict_json(line) for line in complete.stream(value["stream"]).splitlines()]
                events[0]["timestamp"] = events[0]["timestamp"].removesuffix("Z") + ".٠Z"
                value["stream"] = stream(rows(events))
            aggregate(bodies, "cycle-probe-events.jsonl", change)
        self.refuses(probe_clock)

    def test_success_streams_use_one_canonical_base64_and_typed_byte_bound(self):
        self.assertEqual(complete.stream(stream(b"f")), b"f")
        for body in ("Zh==", "Zg==\n", "Zg=", "Zg==="):
            value = stream(b"f")
            value["body"] = body
            with self.subTest(body=body), self.assertRaises(ValueError):
                complete.stream(value)
        for value in (True, 1.0, -1, (1 << 20) + 1):
            row = stream(b"f")
            row["bytes"] = value
            with self.subTest(bytes=value), self.assertRaises(ValueError):
                complete.stream(row)


if __name__ == "__main__":
    unittest.main()
