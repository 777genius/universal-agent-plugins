"""Success-only Gemini source/state/public chronology; no filesystem reads."""
import copy
import re

import project_native_gemini_cycle as cycle
import project_native_gemini_cycle_proof as original
from project_native_gemini_complete import closed, digest, load, natural, receipt, records, require, same, stream, window

NAME = "uap-gemini-cycle"
RELOAD = "in a running Gemini CLI session use `/mcp reload` and `/skills reload`, or restart Gemini CLI"
NEXT = RELOAD + "; verify this plugin's authentication requirements before using it"


def view_stage(stage):
    if stage.startswith("baseline-"):
        return ""
    if stage.startswith("installed-"):
        return "add-" + cycle.scope(stage)
    return {"restarted-a": "intact-repair-a", "updated-a": "update-a", "repaired-a": "missing-repair-a",
            "removed-a": "remove-a", "removed-b": "remove-b", "removed-user": "remove-user"}.get(stage, stage)


def visible(view, selected):
    return [key for key in dict.fromkeys(("user", selected)) if key in view and view[key].get("item") is not None
            and not view[key]["removed"]]


def clocks(item, binding, previous, start, end):
    created = window(item["created_at"], start, end)
    updated = window(item["updated_at"], created, end)
    window(binding["updated_at"], created, end)
    if previous:
        require(item["created_at"] == previous[0]["created_at"]
                and updated >= window(previous[0]["updated_at"], created, end), "stable installation clocks")
    for value in item["data_receipts"].values():
        for key in ("created_at", "updated_at"):
            if key in value:
                window(value[key], start, end)


def typed_state(state, value):
    # Reject explicit empty/zero carriers that the current typed Go state
    # serialization omits, rather than accepting dormant authority fields.
    require("environment_receipts" not in state, "unexpected environment authority")
    if "transaction_receipts" in state:
        require(type(state["transaction_receipts"]) is list and len(state["transaction_receipts"]) == 1, "typed transaction carrier")
    item = state["installations"][0]
    require(type(item.get("operation_group_id")) is str and bool(item["operation_group_id"]), "operation group authority")
    if value["removed"]:
        require(item.get("data_retained") is True, "retained data flag")
    else:
        require("data_retained" not in item, "omitted installed retention flag")
    inventory = item["package"]["inventory"]
    require(inventory["mcp_present"] is True and inventory["mcp_enabled"] is True, "typed package inventory")
    for row in value["binding"]["receipts"] + state.get("transaction_receipts", []):
        if "before_digest" in row:
            require(type(row["before_digest"]) is str and bool(row["before_digest"]), "omitted empty before digest")


def source_states(body, paths, start, end):
    rows = records(body, cycle.PROOFS)
    recipes, previous, current, views, raw_states = {}, {}, {}, {}, {}
    identities, probe_inventory = {}, None
    for index, row in enumerate(rows):
        stage, value = row["stage"], row["value"]
        if stage.startswith("source-"):
            r = original.source_recipe(row, paths)
            prior = recipes.get(r["scope"])
            require(all(other["nonce"] != r["nonce"] for scope, other in recipes.items() if scope != r["scope"]),
                    "distinct initial user/A/B source nonces")
            if prior:
                require(r["scope"] == "a" and prior["revision"] == "A-one" and r["revision"] == "A-two"
                        and prior["nonce"] == r["nonce"] and prior["source"] == r["source"]
                        and prior["digest"] != r["digest"], "single source revision transition")
            if probe_inventory is not None:
                require(same(probe_inventory, r["files"]["bin/probe"]), "stable source-built probe metadata")
            probe_inventory = r["files"]["bin/probe"]
            recipes[r["scope"]] = r
            current.setdefault(r["scope"], {"item": None, "removed": False})["recipe"] = r
        else:
            selected = "a" if stage in ("before-source-revision-a", "before-missing-a") else cycle.scope(stage)
            require(value["scope"] == selected and selected in recipes, "selected proof scope")
            require(value["removed"] is (stage.startswith("remove-") or stage == "repeat-remove-a"), "proof removal chronology")
            prior = previous.get(selected)
            item, binding = original.state_proof(row, paths, recipes[selected], prior)
            if value["removed"]:
                require(prior is not None and same(binding, prior[1]), "removed wrapper preserves whole last binding")
            state_body = stream(value["state"])
            state = load(state_body)
            typed_state(state, value)
            clocks(item, binding, prior, start, end)
            before = [] if prior is None else prior[1]["receipts"]
            after = binding["receipts"]
            require(same(after[:len(before)], before), "preserved receipt prefix")
            delta = 1 if stage in ("add-user", "add-a", "add-b", "intact-repair-a", "update-a", "missing-repair-a") else 0
            require(len(after) == len(before) + delta, "exact committed receipt delta")
            group = state["transaction_receipts"][0]["operation_group_id"] if value["removed"] else after[-1]["operation_group_id"]
            require(item["operation_group_id"] == group, "state operation group join")
            repeated = {"repeat-add-a": "add-a", "unchanged-update-a": "repeat-add-a",
                        "before-source-revision-a": "intact-repair-a", "before-missing-a": "update-a",
                        "repeat-remove-a": "remove-a"}
            if stage in repeated:
                require(state_body == raw_states[repeated[stage]], "unchanged whole state bytes")
            ids = (item["installation_id"], binding["client_binding_id"], binding["physical_artifact_id"], binding["data_receipt_id"])
            require(all(key == selected or all(a != b for a, b in zip(ids, others)) for key, others in identities.items()), "distinct scope identities")
            identities[selected] = ids
            previous[selected] = item, binding
            current[selected] = {"recipe": recipes[selected], "item": item, "binding": binding,
                                 "removed": value["removed"], "state": state}
            raw_states[stage] = state_body
        views[stage] = copy.deepcopy(current)
        if index == 2:
            views[""] = copy.deepcopy(current)
    require(len(identities) == 3 and all(v["removed"] for v in current.values()), "final retained-data removal closure")
    # Source rows precede installation. Derive these retained runtime paths
    # from the independently validated, stable binding identities, including
    # for earlier views where no owned component was installed yet.
    for view in views.values():
        for selected, value in view.items():
            binding = previous[selected][1]
            value["recipe"].update(active=binding["target_locator"], profile=binding["native_profile_root"],
                                   data=previous[selected][0]["data_receipts"][binding["data_receipt_id"]]["locator"],
                                   skill_path=binding["native_profile_root"] + "/skills/" + value["recipe"]["name"] + "/SKILL.md")
    return {"views": views, "raw_states": raw_states}


def plan_record(view):
    r, binding = view["recipe"], view["binding"]
    user = r["scope"] == "user"
    action = ("agentplugins will install and verify Gemini CLI skills and MCP servers automatically; reload them in a running session or restart Gemini CLI" if user else
              "project skills and MCP settings can be configured automatically; native discovery, workspace trust and system policy require observation in Gemini CLI")
    return {"client_id": "gemini", "scope": "user" if user else "project", "status": "ready", "package_mode": "native",
            "activation": "prepared", "authentication": "not_checked", "policy": "allowed", "verification": "package_validated",
            "physical_artifact_id": binding["physical_artifact_id"],
            "components": [{"kind": "skill", "name": r["name"], "support": "native"}, {"kind": "mcp_server", "name": r["name"], "support": "native"}],
            "user_actions": ["verify the plugin's authentication requirements before using it", action], "warnings": ["authentication_not_catalog_verified"]}


def operation_result(view, label):
    r, binding, item = view["recipe"], view["binding"], view["item"]
    result = {"installation_id": item["installation_id"], "requires_confirmation": False,
              "mutated": label not in ("repeat-add-a", "unchanged-update-a")}
    if label.startswith("remove-"):
        result.update(plugin=NAME, client_id="gemini", group_phase="external_completed", affected_surfaces=["gemini"],
                      deactivation={"activation": "not_required", "artifact_removal_allowed": True, "external_removal_complete": True})
        return result
    actions = [RELOAD]
    if r["scope"] != "user":
        actions.append("Gemini project configuration is installed; native discovery, workspace trust, system policy and MCP initialization remain unobserved; start Gemini CLI in the selected project and inspect skills list and mcp list")
    result.update(plan=plan_record(view), activation={"activation": "active" if r["scope"] == "user" else "prepared",
                  "authentication": "not_checked", "policy": "allowed", "verification": "installation_verified", "user_actions": actions})
    if label != "repeat-add-a":
        result["group_phase"] = "external_completed"
    if label in ("repeat-add-a", "unchanged-update-a"):
        result["no_change"] = True
    return result


def assessment(r, cached=False):
    return {"schema_version": 1, "subject": {"tree_digest": r["digest"], "manifest_digest": "sha256:" + r["manifest"]["sha256"]},
            "scanner": {"id": "lintai", "version": "0.1.3"},
            "policy": {"id": "agent-plugin-install", "version": 2, "digest": "sha256:9cf869e299d847d7078aeca01f5a182fcdb0144bbf513c83290459991c79e037"},
            "outcome": "no_blocking_findings", "counts": {"blocking": 0, "warnings": 0, "total": 0}, "scanned_files": 3,
            "report_digest": "sha256:90f5acb6ff3ac64f669be4f4ba6deea7ae5aa3a9954662441c0e5498b8d56598", "evidence_source": "cache" if cached else "local_scan"}


def public_data(view, label, operation, paths):
    r, b = view["recipe"], view["binding"]
    source = "local" if "repair-" in label or label.startswith("remove-") else "direct local source"
    output = {"plugin": NAME, "version": "1.0.0", "source": source, "tree_digest": r["digest"],
              "manifest_digest": "sha256:" + r["manifest"]["sha256"], "dry_run": False, "result": operation_result(view, label)}
    if not label.startswith("remove-"):
        output["next_action"] = NEXT
    if label == "repeat-add-a":
        output["security"] = assessment(r, True)
        return output
    output["operation_id"] = operation
    target = {"target": "gemini", "status": "external_completed", "output": output}
    if not label.startswith("remove-"):
        target["next_action"] = NEXT
    if label in ("update-a", "unchanged-update-a"):
        target["selected"] = True
    data = {"operation_id": operation, "batch": True, "status": "completed", "succeeded": 1, "failed": 0,
            "plugin": NAME, "dry_run": False, "targets": [target]}
    if label.startswith("add-") or label in ("update-a", "unchanged-update-a"):
        data.update(version="1.0.0", source=source, tree_digest=r["digest"])
    if label.startswith("add-"):
        data.update(action_required=1, manifest_digest=output["manifest_digest"], security=assessment(r),
                    environment={"reason": "no_save_requested", "saved": False})
    if label.startswith("remove-"):
        data.update(status="data_retained", plugin_data_preserved=True, data_retained=True,
                    retained_data=[{"data_receipt_id": b["data_receipt_id"], "physical_backend_id": b["physical_artifact_id"],
                                    "scope": b["scope"], "state": "owned"}])
        action = "agentplugins remove " + view["item"]["installation_id"] + " --purge-data"
        if r["scope"] != "user":
            action += " --scope project --project-root " + paths[r["scope"]]
        data["retained_data_action"] = "run `" + action + "` to delete retained plugin data"
    return data


def public(bodies, paths, context):
    operations = set()
    for label in cycle.PUBLIC:
        cwd, argv = cycle.public_command(label, paths)
        receipt(load(bodies[label + ".command.json"]), argv, cwd, 30000, 1 if label == "repeat-remove-a" else 0)
        stdout, stderr = bodies[label + ".stdout.log"], bodies[label + ".stderr.log"]
        if label.startswith("git-"):
            require(stdout == stderr == b"", "empty Git streams")
        elif label == "version":
            require(stdout == b"0.65.0\n" and stderr == b"", "pinned version streams")
        elif label == "repeat-remove-a":
            require(stdout == b"" and stderr == b'agentplugins: plugin is not installed for target "gemini" in project scope; no target was changed\n', "natural repeated removal")
        else:
            require(stderr == cycle.progress(label), "exact public progress")
            value = closed(load(stdout), ("schema_version", "command", "result", "data"))
            require(type(value["data"]) is dict, "public data type")
            operation = value["data"].get("operation_id", "")
            v = context["views"][label][cycle.scope(label)]
            if label != "repeat-add-a":
                require(type(operation) is str and re.fullmatch(r"op-[0-9a-f]{32}", operation)
                        and operation not in operations, "public operation identity")
                operations.add(operation)
            if label not in ("repeat-add-a", "unchanged-update-a"):
                group = (v["state"]["transaction_receipts"][0] if label.startswith("remove-") else v["binding"]["receipts"][-1])["operation_group_id"]
                require(operation == group, "public committed group join")
            verb = {"repeat-add-a": "add", "unchanged-update-a": "update", "intact-repair-a": "repair", "missing-repair-a": "repair"}.get(label, label.split("-")[0])
            require(same(value, {"schema_version": 1, "command": verb, "result": "success", "data": public_data(v, label, operation, paths)}), "full public source/target/security/result schema")


def verify(bodies, paths, start, end):
    context = source_states(bodies["cycle-proof.jsonl"], paths, start, end)
    public(bodies, paths, context)
    return context
