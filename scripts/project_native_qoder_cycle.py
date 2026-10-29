"""Closed Qoder skills-cycle capture; no filesystem access or child processes."""
import base64
from datetime import datetime
import hashlib
import json
from pathlib import PurePosixPath
import re

MODE = "qoder-complete-skills-cycle"
NAME = "uap-qoder-cycle"
DESCRIPTION = "Disposable Qoder project cycle"
PUBLIC = tuple("git-a git-b git-outside version add-a add-b repeat-add-a unchanged-update-a intact-repair-a update-a missing-repair-a remove-a repeat-remove-a remove-b".split())
CATALOG = tuple("empty-outside empty-a empty-b installed-a installed-b installed-outside stable-a updated-a updated-b updated-outside damaged-a repaired-a removed-a remaining-b removed-outside repeat-removed-a removed-b final-a final-b final-outside".split())
PROOF = tuple("fixture source-a-a-one caller-a source-b-b-one caller-b original-user original-a original-b original-registry original-profile add-a add-b repeat-add-a unchanged-update-a intact-repair-a before-source-revision-a source-a-a-two update-a before-missing-a missing-a missing-repair-a remove-a repeat-remove-a remove-b log-ledger".split())
ORDER = tuple(("public" if stage[0] == "P" else "catalog", stage[1:]) for stage in "Pgit-a Pgit-b Pgit-outside Pversion Cempty-outside Cempty-a Cempty-b Padd-a Cinstalled-a Padd-b Cinstalled-b Cinstalled-outside Prepeat-add-a Punchanged-update-a Pintact-repair-a Cstable-a Pupdate-a Cupdated-a Cupdated-b Cupdated-outside Cdamaged-a Pmissing-repair-a Crepaired-a Premove-a Cremoved-a Cremaining-b Cremoved-outside Prepeat-remove-a Crepeat-removed-a Premove-b Cremoved-b Cfinal-a Cfinal-b Cfinal-outside".split())
MEMBERS = ("qoder-cycle-public.jsonl", "qoder-cycle-catalog.jsonl", "qoder-cycle-proof.jsonl")
FLAGS = tuple("project-lifecycle native-restart-removal mcp-initialize mcp-tools-list mcp-tools-call".split())
CLEANUP = "full descendant supervision; forced cleanup fails"
HEX = re.compile(r"[0-9a-f]{64}\Z")
SCHEMA = "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json"


def require(condition):
    if not condition:
        raise ValueError("Qoder complete skills-cycle evidence join differs")


def obj(value, required, optional=""):
    required, optional = set(required.split()), set(optional.split())
    require(type(value) is dict and required <= set(value) <= required | optional)
    return value


def same(left, right):
    if type(left) is not type(right):
        return False
    if type(left) is dict:
        return set(left) == set(right) and all(same(left[key], value) for key, value in right.items())
    if type(left) is list:
        return len(left) == len(right) and all(same(a, b) for a, b in zip(left, right))
    return left == right


def natural(value, maximum):
    require(type(value) is int and 0 <= value <= maximum)
    return value


def digest(value):
    require(type(value) is str and HEX.fullmatch(value))
    return value


def load(raw):
    from project_native_runtime import strict_json
    return strict_json(raw)


def private(raw):
    from project_native_runtime import cline_cycle_private
    return cline_cycle_private(raw, marker=b"uap-qoder-private-")


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def compact(value):
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode()


def canonical(value):
    require(type(value) is str and value.startswith("/") and not value.startswith("//") and value != "/"
            and str(PurePosixPath(value)) == value and ".." not in PurePosixPath(value).parts and "\0" not in value)
    return value


def instant(value):
    require(type(value) is str)
    match = re.fullmatch(r"(\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d)(?:\.(\d{1,9}))?(Z|[+-]\d\d:\d\d)", value)
    require(match is not None)
    base, fraction, offset = match.groups()
    moment = datetime.fromisoformat(base + ("+00:00" if offset == "Z" else offset))
    return int(moment.timestamp()) * 10**9 + int((fraction or "").ljust(9, "0"))


def stream(value):
    obj(value, "bytes sha256 body")
    natural(value["bytes"], 1 << 20)
    digest(value["sha256"])
    require(type(value["body"]) is str and len(value["body"]) <= ((1 << 20) + 2) // 3 * 4)
    raw = base64.b64decode(value["body"], validate=True)
    require(len(raw) == value["bytes"] and sha(raw) == value["sha256"]
            and base64.b64encode(raw).decode() == value["body"] and not private(raw))
    return raw


def embedded(value):
    if type(value) is dict:
        if {"body", "sha256"} <= set(value):
            stream(value)
        else:
            for item in value.values():
                embedded(item)
    elif type(value) is list:
        for item in value:
            embedded(item)


def records(raw, order):
    require(type(raw) is bytes and len(raw) <= 1 << 20 and (not raw or raw.endswith(b"\n")) and not private(raw))
    result = [obj(load(line), "stage value") for line in raw.splitlines()]
    require(len(result) <= len(order))
    for index, row in enumerate(result):
        require(row["stage"] == order[index] and type(row["value"]) is dict)
        embedded(row["value"])
    return result


def timed(value, complete=False):
    obj(value, "command started_at ended_at stdout stderr log")
    command = obj(value["command"], "argv cwd exit error deadline elapsed_ms cleanup")
    require(type(command["argv"]) is list and 1 <= len(command["argv"]) <= 64
            and all(type(arg) is str and len(arg) <= 4096 and "\0" not in arg for arg in command["argv"]))
    canonical(command["cwd"])
    require(type(command["exit"]) is int and -1 <= command["exit"] <= 255
            and type(command["error"]) is bool and type(command["deadline"]) is bool and command["cleanup"] == CLEANUP)
    start, end = instant(value["started_at"]), instant(value["ended_at"])
    require(value["started_at"].endswith("Z") and value["ended_at"].endswith("Z"))
    maximum = 35000 if complete else 65000
    natural(command["elapsed_ms"], maximum)
    require(0 <= end - start <= maximum * 10**6 and command["elapsed_ms"] == (end - start) // 10**6)
    if complete:
        require(command["error"] is False and command["deadline"] is False)
    return command, stream(value["stdout"]), stream(value["stderr"]), start, end


def role(stage):
    return "a" if stage.endswith("-a") else "b" if stage.endswith("-b") else "outside"


def public_args(stage, layout, proofs):
    key = role(stage)
    cwd = layout[key]
    if stage.startswith("git-"):
        return cwd, [layout["git"], "-c", "core.hooksPath=/dev/null", "init", "--quiet"]
    if stage == "version":
        return cwd, [layout["client"], "--version"]
    verb = "repair" if "repair" in stage else stage.removeprefix("repeat-").removeprefix("unchanged-").split("-")[0]
    selector = layout["root"] + "/cycle-source-" + key if verb == "add" or stage == "update-a" else proofs[stage]["installation"]["installation_id"] if verb == "repair" else NAME
    return cwd, [layout["installer"], verb, selector, "--scope", "project", "--project-root", cwd, "--target", "qoder", "--format", "json"] + (["--no-save"] if verb == "add" else [])


def progress(stage):
    if stage in ("add-a", "add-b"):
        return f"Resolving and validating one Agent Plugin package for every selected target...\nInstalling {NAME} for 1 selected clients...\n".encode()
    if stage == "repeat-add-a":
        return b"Resolving and validating one Agent Plugin package for every selected target...\nApplying transactional client package...\n"
    if stage in ("unchanged-update-a", "update-a"):
        return b"Resolving and validating one updated Agent Plugin package for every selected target...\n"
    if "repair" in stage:
        return b"Resolving and validating each unique exact installed package revision once...\n"
    return b""


def public_result(raw, stage, verb, proof):
    row = obj(load(raw), "schema_version command result data")
    require(type(row["schema_version"]) is int and row["schema_version"] == 1 and row["command"] == verb and row["result"] == "success")
    single = stage == "repeat-add-a"
    data = row["data"]
    if single:
        output = obj(data, "plugin version source tree_digest manifest_digest dry_run result", "security next_action")
    else:
        obj(data, "batch status succeeded failed plugin dry_run targets", "operation_id action_required version source revision tree_digest manifest_digest security environment plugin_data_preserved data_retained retained_data retained_data_action")
        expected = dict(batch=True, status="data_retained" if verb == "remove" else "completed", succeeded=1, failed=0, plugin=NAME, dry_run=False)
        require(all(same(data[field], value) for field, value in expected.items()) and type(data["targets"]) is list and len(data["targets"]) == 1)
        target = obj(data["targets"][0], "target status output" + (" selected" if verb == "update" else ""), "next_action")
        require(target["target"] == "qoder" and target["status"] == "external_completed" and (verb != "update" or target["selected"] is True))
        output = obj(target["output"], "plugin version source tree_digest manifest_digest dry_run result", "operation_id revision security next_action")
    ins, bound = proof["installation"], proof["binding"]
    expected = dict(plugin=NAME, version="1.0.0", source="local" if verb in ("repair", "remove") else "direct local source", tree_digest=proof["source_digest"], manifest_digest=ins["package"]["manifest_digest"], dry_run=False)
    require(all(same(output[field], value) for field, value in expected.items()))
    changed = stage not in ("repeat-add-a", "unchanged-update-a")
    result = output["result"]
    require(type(result) is dict and result.get("installation_id") == ins["installation_id"] and result.get("requires_confirmation") is False and result.get("mutated") is changed)
    operation = None if single else data.get("operation_id")
    if single:
        require(result.get("no_change") is True)
    else:
        require(type(operation) is str and re.fullmatch(r"op-[0-9a-f]{32}", operation) and output.get("operation_id") == operation)
        if changed:
            require(operation == ins["operation_group_id"])
    checks = dict(tree_digest=proof["source_digest"], manifest_digest=ins["package"]["manifest_digest"], installation_id=ins["installation_id"], client_binding_id=bound["client_binding_id"],
                  physical_artifact_id=bound["physical_artifact_id"], target_locator=bound["target_locator"], client_id="qoder", target="qoder", scope="project")
    def visit(value):
        if type(value) is dict:
            for field, item in value.items():
                if field in checks:
                    require(same(item, checks[field]))
                if field == "mutated":
                    require(item is changed)
                if field == "no_change":
                    require(type(item) is bool and (not changed or item is False))
                visit(item)
        elif type(value) is list:
            for item in value:
                visit(item)
    visit(data)
    if verb == "remove":
        require(data.get("plugin_data_preserved") is True and data.get("data_retained") is True)
        retained = proof["data"]["receipt"]
        require(same(data.get("retained_data"), [{key: retained[key] for key in ("data_receipt_id", "physical_backend_id", "scope", "state")}]))
    return operation


def has_flag(stage):
    return stage not in ("empty-outside", "empty-a", "empty-b", "installed-outside", "updated-outside", "removed-outside", "final-outside")


def catalog(raw, stage, layout, base, proofs):
    from project_native_qoder_cycle_proof import control, nonce
    key = role(stage)
    cwd, argv = layout[key], [layout["client"], "plugins", "list", "--json"]
    expected = []
    if has_flag(stage):
        view = proofs["add-" + key]["view"]
        argv += ["--plugin-dir", view]
        if stage in ("installed-a", "installed-b", "stable-a", "updated-a", "updated-b", "repaired-a", "remaining-b"):
            revision = "a-two" if stage in ("updated-a", "repaired-a") else key + "-one"
            expected.append(dict(id=NAME + "@flag", name=NAME, source=NAME + "@inline", version="1.0.0", scope="flag", enabled=True, canDisable=False,
                                 installPath=view, description=DESCRIPTION, resources=resources(NAME, f"Disposable Qoder {key} {revision} {nonce(base, key)}")))
    for who in ("a", "b", "user"):
        name, path, description, _, _, entry = control(layout, base, who)
        expected.append(dict(id=name + "@local", name=name, source=name + "@local", enabled=False, canDisable=True, description=description, resources=resources(name, description), **entry))
    require(same(load(raw), expected))
    return cwd, argv


def resources(name, description):
    return dict(skills=[dict(name=name, description=description)], agents=[], mcpServers=[], commands=[], hooks=[])


def pins(summary, layout):
    require(same(summary["acquisition"], {"client": "qoder", "acquisition": {"sha256": "5e3213d2d4256c86c5ad2c8bf73fd392f92ba80d0d38f822806542805a48a8ea", "pin_verified": True,
        "members": [{"Name": "qodercli", "Kind": "file", "Bytes": 182982112, "sha256": "0da0976e13fe00bc28f725d6fe7c2f0fe9fa9a8c37eeb2ee4e80ad4c0fc58953"}]}, "node": None,
        "runtime_files": [{"path": layout["client"], "sha256": "0da0976e13fe00bc28f725d6fe7c2f0fe9fa9a8c37eeb2ee4e80ad4c0fc58953"}]}))
    require(same(summary["scanner"], {"archive_sha256": "2b3d176db752433b904a4b42375543ff398f4841d22e48f7d4f23ded925b72da", "binary_sha256": "b6723451f155eb49e84acb34d1f06de929bddfcff6238cd4e38cdc6519396fba", "platform": "linux-amd64", "route": "prepopulated supported scanner cache; ordinary CLI security assessment", "source_tag": "v0.1.3", "version": "0.1.3"}))


def proof_framing(stage, value):
    if stage == "fixture":
        obj(value, "nonce layout owner_uid owner_gid")
    elif stage.startswith("source-"):
        obj(value, "source digest files")
    elif stage.startswith("caller-"):
        obj(value, "path body files")
    elif stage in ("original-user", "original-a", "original-b"):
        obj(value, "key path manifest skill files entry")
    elif stage == "original-registry":
        obj(value, "path registry settings_path settings")
    elif stage == "original-profile":
        obj(value, "logs_absent inventory")
    elif stage == "missing-a":
        obj(value, "path source_digest object before absent")
    elif stage == "log-ledger":
        obj(value, "original_sha256 expected_sha256 runs")
    else:
        obj(value, "scope revision removed source_digest state binding delivery data")
        obj(value["delivery"], "active view files manifest skill")
        obj(value["data"], "allocated path receipt owner files")


def complete(captured, summary):
    from project_native_qoder_cycle_proof import validate
    from project_native_qoder_cycle_logs import transition
    layout, base, uid, gid, original, proofs, ledger = validate(captured[2], summary)
    require(summary["fixture_authored_updater_config"] == layout["home"] + "/.qoder/settings.json")
    pins(summary, layout)
    public, catalogs = ({row["stage"]: row["value"] for row in rows} for rows in captured[:2])
    expected, runs, groups, last, elapsed = dict(original), [], set(), None, 0
    for kind, stage in ORDER:
        value = (public if kind == "public" else catalogs)[stage]
        command, out, err, start, end = timed(value, True)
        require(last is None or last <= start)
        last, elapsed = end, elapsed + command["elapsed_ms"]
        if kind == "public":
            cwd, argv = public_args(stage, layout, proofs)
            require(command["exit"] == (1 if stage == "repeat-remove-a" else 0))
            if stage.startswith("git-"):
                require(out == err == b"")
            elif stage == "version":
                require(out == b"1.1.78\n" and err == b"")
            elif stage == "repeat-remove-a":
                require(out == b"" and err == b'agentplugins: plugin is not installed for target "qoder" in project scope; no target was changed\n')
            else:
                group = public_result(out, stage, argv[1], proofs[stage])
                require(err == progress(stage))
                if group is not None:
                    require(group not in groups)
                    groups.add(group)
        else:
            require(command["exit"] == 0 and err == b"")
            cwd, argv = catalog(out, stage, layout, base, proofs)
        require(command["argv"] == argv and command["cwd"] == cwd)
        expected = transition(value["log"], expected, runs, uid, gid, start, end, kind == "catalog" or stage == "version")
    obj(ledger, "original_sha256 expected_sha256 runs")
    require(len(runs) == summary["cycle_log_runs"] == 21 and same(ledger, dict(original_sha256=sha(compact(original)), expected_sha256=sha(compact(expected)), runs=runs)))
    require(elapsed <= summary["elapsed_ms"])


def members(bodies, source):
    try:
        return admit(bodies, source)
    except (KeyError, TypeError, AttributeError, IndexError, UnicodeError, RecursionError, OverflowError) as error:
        raise ValueError("Qoder complete skills-cycle structure refused") from error


def admit(bodies, source):
    require("runtime-attempt.json" in bodies and set(bodies) <= {"runtime-attempt.json", *MEMBERS}
            and all(type(raw) is bytes and len(raw) <= 1 << 20 and not private(raw) for raw in bodies.values()) and sum(map(len, bodies.values())) <= 16 << 20)
    summary = obj(load(bodies["runtime-attempt.json"]), "client version acquisition scanner source_identity activation vendor_authentication_qualified real_model_qualified capture_mode nonce layout fixture_authored_updater_config cycle_stage cycle_public_stages cycle_catalog_stages cycle_log_runs cycle_observed_complete runtime_attempt_complete test_failed elapsed_ms qualification")
    require(summary["client"] == "qoder" and summary["version"] == "1.1.78" and summary["capture_mode"] == MODE and summary["activation"] == "prepared"
            and summary["vendor_authentication_qualified"] is False and summary["real_model_qualified"] is False)
    require(all(type(summary[field]) is bool for field in ("runtime_attempt_complete", "test_failed", "cycle_observed_complete")))
    capture_limit = 195000 if summary["test_failed"] and not summary["runtime_attempt_complete"] else 180000
    natural(summary["elapsed_ms"], capture_limit)
    natural(summary["cycle_log_runs"], 21)
    expected_source = dict(installer_base_commit=source["source_commit"], installer_tree=source["source_tree"], installer_source_state="committed", installer_patch_sha256=[])
    require(same(summary["source_identity"], expected_source))
    flag = summary["runtime_attempt_complete"]
    require(same(summary["qualification"], {key: flag and key in FLAGS[:2] for key in FLAGS}))
    captured = [records(bodies.get(name, b""), order) for name, order in zip(MEMBERS, (PUBLIC, CATALOG, PROOF))]
    require(same(summary["cycle_public_stages"], [row["stage"] for row in captured[0]]) and same(summary["cycle_catalog_stages"], [row["stage"] for row in captured[1]]))
    require(summary["cycle_stage"] in {"prepare", "complete-cycle"} | {kind + ":" + stage for kind, stage in ORDER})
    from project_native_qoder_cycle_logs import framing
    for rows in captured[:2]:
        for row in rows:
            timed(row["value"])
            framing(row["value"]["log"])
    for row in captured[2]:
        proof_framing(row["stage"], row["value"])
    if not flag:
        require(summary["test_failed"] is True)
        return summary
    require(set(bodies) == {"runtime-attempt.json", *MEMBERS} and summary["test_failed"] is False and summary["cycle_observed_complete"] is True and summary["cycle_stage"] == "complete-cycle"
            and summary["elapsed_ms"] < 150000 and all(len(rows) == len(order) for rows, order in zip(captured, (PUBLIC, CATALOG, PROOF))))
    complete(captured, summary)
    return summary
