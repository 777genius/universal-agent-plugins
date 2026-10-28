"""Closed Copilot User/A/B lifecycle capture reader; never executes a client."""
import base64
import hashlib
import json
from pathlib import PurePosixPath
import re

from project_native_copilot_events import instant, natural, obj, require, same, values
from project_native_copilot_readiness import canonical, compact

MODE = "copilot-project-complete-cycle"
PUBLIC = tuple("git-a git-b version add-user add-a add-b repeat-add-a unchanged-update-a intact-repair-a update-a missing-repair-a remove-a repeat-remove-a remove-b repeat-remove-b remove-user repeat-remove-user".split())
REGISTRY = ("empty-user",) + PUBLIC[3:]
NATIVE = tuple("installed-user empty-a empty-b installed-a installed-b stable-a updated-a repaired-a removed-a remaining-b removed-b remaining-user final-a final-b".split())
AGGREGATES = tuple("cycle-" + name + ".jsonl" for name in ("native-events", "native-commands", "registry-listings", "provider-requests", "provider-responses", "provider-receipts", "probe-events", "proof"))
PROOF = tuple("fixture source-user-user-one caller-user source-a-a-one caller-a source-b-b-one caller-b foreign-user foreign-b add-user add-a after-native-installed-a add-b after-native-installed-b repeat-add-a unchanged-update-a intact-repair-a after-native-stable-a before-source-revision-a source-a-a-two update-a after-native-updated-a before-missing-a missing-a missing-repair-a after-native-repaired-a remove-a after-native-remaining-b repeat-remove-a remove-b repeat-remove-b remove-user repeat-remove-user".split())
QUALIFICATIONS = tuple("project-lifecycle native-restart-removal mcp-initialize mcp-tools-list mcp-tools-call".split())
TRIPLES = {label + suffix for label in PUBLIC for suffix in (".command.json", ".stdout.log", ".stderr.log")}
EXACT = TRIPLES | set(AGGREGATES) | {"runtime-attempt.json"}
SUMMARY = set("client provider_kind runtime_attempt_complete activation vendor_authentication_qualified real_model_qualified qualification mode provider_closed source_identity acquisition scanner nonce layout cycle_stage cycle_observed_complete cycle_public_stages cycle_registry_stages cycle_native_stages cycle_native_sessions cycle_probe_sessions test_failed elapsed_ms original_config_change".split())
CLEANUP = "ordinary full-descendant supervision; forced cleanup is a failure"
HEX = re.compile(r"[0-9a-f]{64}\Z")
CALLS = {"installed-a": ("a", "write"), "installed-b": ("b", "write"), "stable-a": ("a", "read"), "updated-a": ("a", "read"), "repaired-a": ("a", "read"), "remaining-b": ("b", "read")}


def load(raw):
    from project_native_runtime import strict_json
    return strict_json(raw)


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def private(raw):
    from project_native_runtime import cline_cycle_private
    return cline_cycle_private(raw, marker=b"uap-copilot-private-")


def stream(value):
    obj(value, "bytes sha256 body")
    natural(value["bytes"], 1 << 20)
    require(type(value["sha256"]) is str and HEX.fullmatch(value["sha256"]) is not None
            and type(value["body"]) is str and len(value["body"]) <= (1 << 20) * 4 // 3 + 4)
    raw = base64.b64decode(value["body"], validate=True)
    require(base64.b64encode(raw).decode() == value["body"] and len(raw) == value["bytes"]
            and sha(raw) == value["sha256"] and not private(raw))
    return raw


def rows(raw):
    require(type(raw) is bytes and len(raw) <= 1 << 20 and (not raw or raw.endswith(b"\n")) and not private(raw))
    result = [obj(load(line), "stage value") for line in raw.splitlines()]
    require(len(result) <= 128 and all(type(row["stage"]) is str and type(row["value"]) is dict for row in result))
    return result


def prefix(actual, order):
    require(type(actual) is list and actual == list(order[:len(actual)]))


def sequence(records, order, repeat=False):
    index, count = -1, 0
    for row in records:
        stage = row["stage"]
        require(stage in order)
        current = order.index(stage)
        require(current == index + 1 or repeat and current == index)
        count = count + 1 if current == index else 1
        require(count <= (2 if stage in CALLS else 1))
        index = current


def command(value):
    obj(value, "argv cwd exit elapsed_ms error context_error cleanup")
    require(type(value["argv"]) is list and 1 <= len(value["argv"]) <= 64
            and all(type(arg) is str and len(arg) <= 4096 and "\0" not in arg for arg in value["argv"]))
    canonical(value["cwd"])
    require(type(value["exit"]) is int and -1 <= value["exit"] <= 255
            and type(value["error"]) is bool and type(value["context_error"]) is bool)
    natural(value["elapsed_ms"], 65000)
    values(value, {"cleanup": CLEANUP})
    return value


def timed(value, complete=False, maximum=65000):
    obj(value, "command started_at ended_at stdout stderr")
    row = command(value["command"])
    start, end = instant(value["started_at"]), instant(value["ended_at"])
    require(value["started_at"].endswith("Z") and value["ended_at"].endswith("Z")
            and 0 <= end - start <= maximum * 10**6 and row["elapsed_ms"] == (end - start) // 10**6)
    natural(row["elapsed_ms"], maximum)
    out, err = stream(value["stdout"]), stream(value["stderr"])
    if complete:
        values(row, {"exit": 0, "error": False, "context_error": False})
        require(err == b"")
    return row, out, start, end


def raw_fields(value):
    """Validate every embedded raw receipt even on a truthful failed prefix."""
    if type(value) is dict:
        if {"sha256", "body"} <= set(value):
            stream(value)
        else:
            for item in value.values():
                raw_fields(item)
    elif type(value) is list:
        for item in value:
            raw_fields(item)


def pins(result, layout):
    acquisition = obj(result["acquisition"], "client acquisition runtime_files")
    values(acquisition, {"client": "copilot", "acquisition": {"sha256": "9aeb4430e9ac9523546db07b227c7aba179a4710a55f9ac29ad143b0f9e1b325", "pin_verified": True,
        "members": [{"name": "copilot", "kind": "file", "bytes": 198445888, "sha256": "f19b3554a6b62605db3a26d5255ebe2a3ee39fcd6a357edb18301ea5037f6f0d"}]},
        "runtime_files": [{"path": layout["client"], "sha256": "f19b3554a6b62605db3a26d5255ebe2a3ee39fcd6a357edb18301ea5037f6f0d"}]})
    require(same(result["scanner"], {"archive_sha256": "2b3d176db752433b904a4b42375543ff398f4841d22e48f7d4f23ded925b72da", "binary_sha256": "b6723451f155eb49e84acb34d1f06de929bddfcff6238cd4e38cdc6519396fba", "platform": "linux-amd64", "route": "prepopulated supported scanner cache; ordinary CLI security assessment", "source_tag": "v0.1.3", "version": "0.1.3"}))


def owner(label):
    return "a" if label.endswith("-a") else "b" if label.endswith("-b") else "user"


def public_argv(label, layout):
    key = owner(label)
    cwd = layout["outside"] if key == "user" else layout[key]
    if label.startswith("git-"):
        return [layout["git"], "-c", "core.hooksPath=/dev/null", "init", "--quiet"], cwd
    if label == "version":
        return [layout["client"], "--version"], cwd
    verb = "repair" if label in ("intact-repair-a", "missing-repair-a") else label.removeprefix("repeat-").removeprefix("unchanged-").split("-")[0]
    selector = layout["root"] + "/cycle-source-" + key if verb == "add" or label == "update-a" else "native-proof"
    argv = [layout["installer"], verb, selector, "--scope", "user" if key == "user" else "project", "--target", "copilot", "--format", "json"]
    if key != "user":
        argv += ["--project-root", cwd]
    if verb == "add":
        argv += ["--no-save"]
    return argv, cwd


def progress(label):
    if label in ("add-user", "add-a", "add-b"):
        return b"Resolving and validating one Agent Plugin package for every selected target...\nInstalling native-proof for 1 selected clients...\n"
    if label == "repeat-add-a":
        return b"Resolving and validating one Agent Plugin package for every selected target...\nApplying transactional client package...\n"
    if label in ("unchanged-update-a", "update-a"):
        return b"Resolving and validating one updated Agent Plugin package for every selected target...\n"
    if label in ("intact-repair-a", "missing-repair-a"):
        return b"Resolving and validating each unique exact installed package revision once...\n"
    return b""


def public_json(raw, label, proof):
    row = obj(load(raw), "schema_version command result data")
    verb = public_argv(label, {"root": "", "a": "", "b": "", "outside": "", "installer": ""})[0][1]
    values(row, {"schema_version": 1, "command": verb, "result": "success"})
    changed = label not in ("repeat-add-a", "unchanged-update-a")
    data = row["data"]
    required = set("batch status succeeded failed plugin dry_run targets".split())
    allowed = required | set("operation_id action_required version source revision tree_digest manifest_digest security environment plugin_data_preserved data_retained retained_data retained_data_action".split())
    require(type(data) is dict and required <= set(data) <= allowed)
    values(data, {"batch": True, "succeeded": 1, "failed": 0, "plugin": "native-proof", "dry_run": False})
    require(data["status"] == ("data_retained" if verb == "remove" else "completed"))
    require(type(data["targets"]) is list and len(data["targets"]) == 1)
    target = data["targets"][0]
    required_target = {"target", "status", "output"} | ({"selected"} if verb == "update" else set())
    require(type(target) is dict and required_target <= set(target) <= required_target | {"next_action"})
    values(target, {"target": "copilot", "status": "external_completed"})
    if verb == "update":
        require(target["selected"] is True)
    output = target["output"]
    output_required = set("plugin version source tree_digest manifest_digest dry_run result".split())
    require(type(output) is dict and output_required <= set(output) <= output_required | {"operation_id", "revision", "security", "next_action"})
    values(output, {"plugin": "native-proof", "version": proof["version"], "source": "direct local source", "tree_digest": proof["source_digest"],
                    "manifest_digest": proof["installation"]["package"]["manifest_digest"], "dry_run": False})
    outcome = output["result"]
    require(type(outcome) is dict)
    values(outcome, {"installation_id": proof["installation"]["installation_id"], "requires_confirmation": False, "mutated": changed})
    if "group_phase" in outcome:
        require(outcome["group_phase"] == "external_completed")
    if verb == "remove":
        values(data, {"plugin_data_preserved": True, "data_retained": True})
        retained = proof["data"]["receipt"]
        require(same(data["retained_data"], [{key: retained[key] for key in ("data_receipt_id", "physical_backend_id", "scope", "state")}]))
    found = []
    def visit(value):
        if type(value) is dict:
            for key, item in value.items():
                if key == "mutated":
                    require(item is changed)
                    found.append(item)
                if key == "no_change":
                    require(type(item) is bool and (not changed or item is False))
                if key in ("tree_digest", "manifest_digest"):
                    require(item == (proof["source_digest"] if key == "tree_digest" else proof["installation"]["package"]["manifest_digest"]))
                if key in ("client_id", "target"):
                    require(item == "copilot")
                if key == "scope":
                    require(item == ("user" if proof["scope"] == "user" else "project"))
                if key in ("installation_id", "client_binding_id", "physical_artifact_id", "target_locator"):
                    expected = proof["installation"]["installation_id"] if key == "installation_id" else proof["binding"].get(key)
                    if expected is not None:
                        require(item == expected)
                visit(item)
        elif type(value) is list:
            for item in value:
                visit(item)
    visit(row["data"])
    require(found)


def complete_public(bodies, commands, layout, proofs):
    for label in PUBLIC:
        row = commands[label]
        natural(row["elapsed_ms"], 35000)
        argv, cwd = public_argv(label, layout)
        values(row, {"argv": argv, "cwd": cwd, "exit": 1 if label.startswith("repeat-remove-") else 0, "error": False, "context_error": False})
        out, err = bodies[label + ".stdout.log"], bodies[label + ".stderr.log"]
        if label.startswith("repeat-remove-"):
            scope = "user" if owner(label) == "user" else "project"
            require(out == b"" and err == ('agentplugins: plugin is not installed for target "copilot" in ' + scope + ' scope; no target was changed\n').encode())
        elif label.startswith("git-"):
            require(out == err == b"")
        elif label == "version":
            require(out == b"GitHub Copilot CLI 1.0.103.\nRun 'copilot update' to check for updates.\n" and err == b"")
        else:
            require(err == progress(label))
            public_json(out, label, proofs[label])


def framing(name, stage, value):
    if name == AGGREGATES[0]:
        stream(value)
    elif name in AGGREGATES[3:5]:
        obj(value, "index body")
        require(type(value["index"]) is int and 1 <= value["index"] <= (2 if stage in CALLS else 1))
        stream(value["body"])
    elif name == AGGREGATES[5]:
        obj(value, "requests responses error returned shutdown serve_closed connections_closed forced")
        for key in ("requests", "responses"):
            natural(value[key], 2 if stage in CALLS else 1)
        require(all(type(value[key]) is bool for key in ("error", "returned", "shutdown", "serve_closed", "connections_closed", "forced")))
    elif name == AGGREGATES[6]:
        obj(value, "before_bytes before_sha256 after_bytes after_sha256 delta")
        for key in ("before_bytes", "after_bytes"):
            natural(value[key], 1 << 20)
        for key in ("before_sha256", "after_sha256"):
            require(type(value[key]) is str and HEX.fullmatch(value[key]))
        require(value["after_bytes"] == value["before_bytes"] + len(stream(value["delta"])))
    elif name == AGGREGATES[7]:
        if stage == "fixture":
            obj(value, "nonce layout immutable")
        elif stage.startswith("source-"):
            obj(value, "source digest files")
        elif stage.startswith("caller-"):
            obj(value, "path body files")
        elif stage.startswith("foreign-"):
            obj(value, "name description source path body files")
        elif stage == "missing-a":
            obj(value, "path source_digest object before absent")
            require(value["absent"] is True)
        else:
            obj(value, "scope revision version removed state binding source_digest native data")
            require(value["scope"] in ("user", "a", "b") and type(value["removed"]) is bool)
            obj(value["native"], "active managed_files objects skill_path skill config_path config skill_files launcher_metadata")
            obj(value["data"], "allocated path receipt owner marker files")
            require(type(value["data"]["allocated"]) is bool)


def members(bodies, source):
    try:
        return _members(bodies, source)
    except (KeyError, TypeError, AttributeError, IndexError, RecursionError, OverflowError) as error:
        raise ValueError("Copilot cycle closed capture refused") from error


def _members(bodies, source):
    require("runtime-attempt.json" in bodies and not set(bodies) - EXACT
            and sum(map(len, bodies.values())) <= 16 << 20 and all(len(raw) <= 1 << 20 and not private(raw) for raw in bodies.values()))
    result = load(bodies["runtime-attempt.json"])
    require(type(result) is dict and not set(result) - SUMMARY)
    values(result, {"mode": MODE, "client": "copilot", "provider_kind": "scripted_loopback", "activation": "prepared", "vendor_authentication_qualified": False, "real_model_qualified": False})
    for key in ("test_failed", "runtime_attempt_complete", "provider_closed", "cycle_observed_complete"):
        require(type(result.get(key)) is bool)
    natural(result["elapsed_ms"], 180000)
    if "original_config_change" in result:
        from project_native_copilot_user import config_facts
        config_facts(result["original_config_change"])
    complete = result["runtime_attempt_complete"]
    require(same(result["qualification"], {name: complete for name in QUALIFICATIONS}) and (not complete or not result["test_failed"]))
    expected_source = {"installer_base_commit": source["source_commit"], "installer_tree": source["source_tree"], "installer_source_state": "committed", "installer_patch_sha256": []}
    if "source_identity" in result:
        require(same(result["source_identity"], expected_source))
    commands = {}
    for label in PUBLIC:
        names = {label + suffix for suffix in (".command.json", ".stdout.log", ".stderr.log")}
        require(not names & set(bodies) or names <= set(bodies))
        if names <= set(bodies):
            commands[label] = command(load(bodies[label + ".command.json"]))
    prefix(list(commands), PUBLIC)
    for key, order in (("cycle_public_stages", PUBLIC), ("cycle_registry_stages", REGISTRY), ("cycle_native_stages", NATIVE)):
        prefix(result.get(key), order)
    require(result["cycle_public_stages"] == list(commands))
    natural(result["cycle_native_sessions"], 14)
    natural(result["cycle_probe_sessions"], 6)
    if "cycle_stage" in result:
        require(result["cycle_stage"] in {"complete-cycle"} | {"public:" + x for x in PUBLIC}
                | {"registry:" + x for x in REGISTRY} | {"native:" + x for x in NATIVE})
    data = {name: rows(bodies.get(name, b"")) for name in AGGREGATES}
    for name, order in ((AGGREGATES[0], NATIVE), (AGGREGATES[1], NATIVE), (AGGREGATES[2], REGISTRY), (AGGREGATES[5], NATIVE), (AGGREGATES[6], NATIVE), (AGGREGATES[7], PROOF)):
        sequence(data[name], order)
    for name in AGGREGATES[3:5]:
        sequence(data[name], NATIVE, repeat=True)
    require([r["stage"] for r in data[AGGREGATES[2]]] == result["cycle_registry_stages"])
    for name, records in data.items():
        for row in records:
            framing(name, row["stage"], row["value"])
            raw_fields(row["value"])
    for name in (AGGREGATES[1], AGGREGATES[2]):
        for row in data[name]:
            timed(row["value"])
    if not complete:
        require(result["test_failed"] is True and all(flag is False for flag in result["qualification"].values()))
        return result
    require(set(bodies) == EXACT and result["provider_closed"] and result["cycle_observed_complete"]
            and result["cycle_stage"] == "complete-cycle" and "original_config_change" not in result
            and result["source_identity"] == expected_source and result["elapsed_ms"] < 180000
            and result["cycle_public_stages"] == list(PUBLIC) and result["cycle_registry_stages"] == list(REGISTRY)
            and result["cycle_native_stages"] == list(NATIVE) and result["cycle_native_sessions"] == 14 and result["cycle_probe_sessions"] == 6)
    for name, count in zip(AGGREGATES, (14, 14, 15, 20, 20, 14, 14, 33)):
        require(len(data[name]) == count)
    from project_native_copilot_cycle_proof import validate
    layout, proofs, recipes = validate(data[AGGREGATES[7]], result)
    pins(result, layout)
    complete_public(bodies, commands, layout, proofs)
    from project_native_copilot_cycle_exchange import validate as exchange
    elapsed = exchange(data, layout, recipes)
    require(result["elapsed_ms"] >= elapsed + sum(row["elapsed_ms"] for row in commands.values()))
    return result
