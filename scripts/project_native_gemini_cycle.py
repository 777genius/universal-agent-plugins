"""Source-bound failed-only first Gemini complete-cycle capture admission."""
import posixpath
import re
from datetime import datetime

from project_native_gemini_cycle_bytes import private, rows, stream, strict_json

MODE = "gemini-complete-cycle-first-capture"
PUBLIC = ("git-a", "git-b", "git-user", "version", "add-user", "add-a", "add-b", "repeat-add-a",
          "unchanged-update-a", "intact-repair-a", "update-a", "missing-repair-a", "remove-a",
          "repeat-remove-a", "remove-b", "remove-user")
LISTINGS = ("baseline-a", "baseline-b", "baseline-user", "installed-user", "installed-a", "installed-b",
            "repeat-add-a", "unchanged-update-a", "intact-repair-a", "updated-a", "repaired-a",
            "removed-a", "removed-b", "removed-user")
NATIVE = ("baseline-a", "installed-user", "installed-a", "installed-b", "restarted-a", "updated-a",
          "repaired-a", "removed-a", "removed-b", "removed-user")
OBSERVATIONS = ("listing:baseline-a", "listing:baseline-b", "listing:baseline-user", "native:baseline-a",
                "listing:installed-user", "native:installed-user", "listing:installed-a", "native:installed-a",
                "listing:installed-b", "native:installed-b", "listing:repeat-add-a", "listing:unchanged-update-a",
                "listing:intact-repair-a", "native:restarted-a", "listing:updated-a", "native:updated-a",
                "listing:repaired-a", "native:repaired-a", "listing:removed-a", "native:removed-a",
                "listing:removed-b", "native:removed-b", "listing:removed-user", "native:removed-user")
PROOFS = ("source-user-U-one", "source-a-A-one", "source-b-B-one", "add-user", "add-a", "add-b",
          "repeat-add-a", "unchanged-update-a", "intact-repair-a", "before-source-revision-a", "source-a-A-two",
          "update-a", "before-missing-a", "missing-repair-a", "remove-a", "repeat-remove-a", "remove-b", "remove-user")
AGGREGATES = ("cycle-native-commands.jsonl", "cycle-native-events.jsonl", "cycle-discovery.jsonl",
              "cycle-provider-requests.jsonl", "cycle-provider-responses.jsonl", "cycle-provider-receipts.jsonl",
              "cycle-probe-events.jsonl", "cycle-proof.jsonl")
QUAL = dict.fromkeys(("project-lifecycle", "native-restart-removal", "mcp-initialize", "mcp-tools-list", "mcp-tools-call"), False)
CLEANUP = "ordinary full-descendant supervision; forced cleanup is a failure"


def closed(value, required, optional=()):
    if type(value) is not dict or set(required) - value.keys() or value.keys() - set(required) - set(optional):
        raise ValueError("Gemini cycle closed object differs")
    return value


def prefix(actual, expected):
    return type(actual) is list and len(actual) <= len(expected) and actual == list(expected[:len(actual)])


def scope(stage):
    return "user" if stage == "version" or stage.endswith("-user") else "a" if stage.endswith("-a") else "b" if stage.endswith("-b") else ""


def layout(value):
    closed(value, ("root", "home", "a", "b", "outside", "installer", "client", "node", "git"))
    if (len(set(value.values())) != len(value) or any(type(v) is not str or not v.startswith("/")
            or v.startswith("//") or posixpath.normpath(v) != v or "\0" in v for v in value.values())):
        raise ValueError("Gemini cycle canonical distinct layout differs")
    for key, rel in (("home", "home"), ("a", "project"), ("b", "project-B"), ("outside", "outside")):
        if value[key] != value["root"] + "/" + rel:
            raise ValueError("Gemini cycle original home/project layout differs")
    return value


def public_command(label, paths):
    selected = scope(label)
    cwd = paths["outside" if selected == "user" else selected]
    if label.startswith("git-"):
        return cwd, [paths["git"], "-c", "core.hooksPath=/dev/null", "init", "--quiet"]
    if label == "version":
        return cwd, [paths["node"], paths["client"], "--version"]
    verb = {"repeat-add-a": "add", "unchanged-update-a": "update", "intact-repair-a": "repair",
            "missing-repair-a": "repair", "repeat-remove-a": "remove"}.get(label, label.split("-")[0])
    selector = paths["root"] + "/cycle-source-" + selected if verb == "add" or label == "update-a" else "uap-gemini-cycle"
    args = [paths["installer"], verb, selector, "--scope", "user" if selected == "user" else "project", "--target", "gemini", "--format", "json"]
    if selected != "user": args += ["--project-root", cwd]
    if verb == "add": args += ["--no-save"]
    return cwd, args


def receipt(value, argv, cwd):
    closed(value, ("argv", "cwd", "exit", "error", "context_error", "elapsed_ms", "cleanup"))
    if (value["argv"] != argv or value["cwd"] != cwd or type(value["exit"]) is not int
            or type(value["error"]) is not bool or type(value["context_error"]) is not bool
            or type(value["elapsed_ms"]) is not int or value["elapsed_ms"] < 0 or value["cleanup"] != CLEANUP):
        raise ValueError("Gemini cycle exact typed command receipt differs")


def records(body, order, repeat=False):
    values = rows(body)
    previous, seen = -1, set()
    for row in values:
        stage = row["stage"]
        if stage not in order: raise ValueError("Gemini cycle unregistered aggregate stage")
        index = order.index(stage)
        if index < previous or (not repeat and stage in seen): raise ValueError("Gemini cycle reordered/repeated aggregate stage")
        if index > previous + 1: raise ValueError("Gemini cycle aggregate stage gap")
        previous = index; seen.add(stage)
    return values


def _members(bodies, source, result):
    if (result.get("client") != "gemini" or result.get("provider_kind") != "scripted_loopback"
            or result.get("capture_mode") != MODE or result.get("cycle_capture_review_required") is not True
            or result.get("runtime_attempt_complete") is not False or result.get("test_failed") is not True
            or type(result.get("cycle_observed_complete")) is not bool or result.get("activation") != "prepared"
            or result.get("vendor_authentication_qualified") is not False or result.get("real_model_qualified") is not False
            or result.get("qualification") != QUAL or any(type(v) is not bool for v in result.get("qualification", {}).values())
            or result.get("source_identity") != {"installer_base_commit": source["source_commit"], "installer_tree": source["source_tree"], "installer_source_state": "committed", "installer_patch_sha256": []}):
        raise ValueError("Gemini cycle first capture must remain failed and wholly unqualified")
    paths = layout(result.get("cycle_layout"))
    if result.get("attempt_stage") not in {"prepare","source-revision-a","missing-owned-skill-a","complete-cycle-capture-review",*PUBLIC,*OBSERVATIONS}:
        raise ValueError("Gemini cycle actual finite failure stage differs")
    for key, order in (("cycle_public_stages", PUBLIC), ("cycle_listing_stages", LISTINGS), ("cycle_native_stages", NATIVE), ("cycle_provider_completed", NATIVE)):
        if not prefix(result.get(key, []), order): raise ValueError("Gemini cycle exact completed prefix differs")
    if result.get("cycle_provider_completed",[])!=result["cycle_native_stages"]:
        raise ValueError("Gemini cycle provider/native closed prefixes differ")
    public = result["cycle_public_stages"]
    allowed = {"runtime-attempt.json", *AGGREGATES}
    for label in public:
        triple = {label + suffix for suffix in (".command.json", ".stdout.log", ".stderr.log")}
        allowed |= triple
        if not triple <= bodies.keys(): raise ValueError("Gemini cycle incomplete public triple")
        cwd, args = public_command(label, paths)
        row = strict_json(bodies[label + ".command.json"]);receipt(row, args, cwd)
        later = public.index(label) < len(public) - 1 or result["cycle_observed_complete"] or result.get("attempt_stage") != label
        if later:
            expected_exit = 1 if label == "repeat-remove-a" else 0
            if row["exit"] != expected_exit or row["error"] or row["context_error"]: raise ValueError("Gemini cycle continued past failed public process")
            if label == "repeat-remove-a" and (bodies[label + ".stdout.log"] or bodies[label + ".stderr.log"] != b'agentplugins: plugin is not installed for target "gemini" in project scope; no target was changed\n'):
                raise ValueError("Gemini cycle repeated removal refusal differs")
            if label == "version" and (bodies[label + ".stdout.log"] != b"0.65.0\n" or bodies[label + ".stderr.log"]): raise ValueError("Gemini cycle pinned version differs")
            if label.startswith("git-") and (bodies[label + ".stdout.log"] or bodies[label + ".stderr.log"]): raise ValueError("Gemini cycle Git output differs")
            if not label.startswith("git-") and label not in ("version", "repeat-remove-a"):
                _public_result(bodies[label + ".stdout.log"], label)
                if bodies[label+".stderr.log"]!=progress(label): raise ValueError("Gemini exact public progress differs")
    if (set(bodies) != allowed or len(bodies) > 57 or any(type(v) is not bytes or len(v) > 2 << 20 or private(v) for v in bodies.values()) or sum(map(len, bodies.values())) > 16 << 20):
        raise ValueError("Gemini cycle closed ordinary member/size/privacy boundary")
    aggregates = {name: rows(bodies[name]) for name in AGGREGATES}
    commands = records(bodies[AGGREGATES[0]], OBSERVATIONS)
    for row in commands:
        _observation(row, paths)
        value=row["value"]
        if row is not commands[-1] or result["cycle_observed_complete"] or result.get("attempt_stage") != row["stage"]:
            if value["exit"] != 0 or value["error"] or value["context_error"]:
                raise ValueError("Gemini cycle continued past failed native capture")
    for name, order in (("cycle-native-events.jsonl", NATIVE), ("cycle-discovery.jsonl", LISTINGS)):
        selected = records(bodies[name], order)
        expected = result["cycle_native_stages" if name == "cycle-native-events.jsonl" else "cycle_listing_stages"]
        actual=[v["stage"] for v in selected]
        pending=not result["cycle_observed_complete"] and name=="cycle-native-events.jsonl" and len(expected)<len(NATIVE) and result.get("attempt_stage")=="native:"+NATIVE[len(expected)]
        if actual!=expected and not (pending and actual==expected+[NATIVE[len(expected)]]):
            raise ValueError("Gemini cycle raw/validated observation prefix differs")
        for row in selected:
            value = row["value"]
            if name == "cycle-native-events.jsonl": closed(value, ("scope", "session", "stream"));stream(value["stream"])
            else: closed(value, ("scope", "stdout", "stderr"));stream(value["stdout"]);stream(value["stderr"])
            if value["scope"] != scope(row["stage"]): raise ValueError("Gemini cycle observation scope differs")
            command_stage=("native:" if name == "cycle-native-events.jsonl" else "listing:") + row["stage"]
            matches=[v["value"] for v in commands if v["stage"]==command_stage]
            if len(matches)!=1: raise ValueError("Gemini cycle validated observation lacks captured command")
            if name == "cycle-native-events.jsonl":
                if stream(value["stream"]) != stream(matches[0]["stdout"]): raise ValueError("Gemini cycle raw native stream join differs")
            elif any(stream(value[k]) != stream(matches[0][k]) for k in ("stdout","stderr")):
                raise ValueError("Gemini cycle raw discovery stream join differs")
    for name in ("cycle-provider-requests.jsonl", "cycle-provider-responses.jsonl"):
        for row in records(bodies[name], NATIVE, repeat=True):
            closed(row["value"], ("index", "stream"))
            if type(row["value"]["index"]) is not int or row["value"]["index"] not in (0, 1): raise ValueError("Gemini cycle provider stream index")
            stream(row["value"]["stream"])
    provider = records(bodies["cycle-provider-receipts.jsonl"], NATIVE)
    for row in provider:
        _provider(row)
        for name,key in (("cycle-provider-requests.jsonl","requests"),("cycle-provider-responses.jsonl","responses")):
            selected=[v["value"] for v in aggregates[name] if v["stage"]==row["stage"]]
            if len(selected)!=row["value"][key] or [v["index"] for v in selected]!=list(range(len(selected))):
                raise ValueError("Gemini cycle provider raw counts/indices differ")
        if row["stage"] in result.get("cycle_provider_completed",[]):
            no_call=row["value"]["operation"]=="none";want=1 if no_call else 2
            if any(row["value"][k]!=want for k in ("requests","responses")) or row["value"]["error"] or row["value"]["issued"] is no_call or row["value"]["returned"] is no_call:
                raise ValueError("Gemini cycle completed provider receipt contradicts raw closure")
    probe_order = tuple(v.removeprefix("native:") if v.startswith("native:") else v for v in OBSERVATIONS)
    for row in records(bodies["cycle-probe-events.jsonl"], probe_order):
        closed(row["value"], ("scope", "stream"));stream(row["value"]["stream"])
    from project_native_gemini_cycle_proof import proofs
    proof_rows = records(bodies["cycle-proof.jsonl"], PROOFS)
    recipes,_=proofs(proof_rows, paths)
    for row in provider: _provider_identity(row,recipes)
    if result["cycle_observed_complete"]:
        if (result.get("attempt_stage") != "complete-cycle-capture-review" or public != list(PUBLIC)
                or result["cycle_listing_stages"] != list(LISTINGS) or result["cycle_native_stages"] != list(NATIVE)
                or result.get("cycle_provider_completed") != list(NATIVE) or len(commands) != 24 or len(provider) != 10
                or len(proof_rows) != 18 or len(aggregates["cycle-discovery.jsonl"]) != 14
                or len(aggregates["cycle-native-events.jsonl"]) != 10 or len(aggregates["cycle-probe-events.jsonl"]) != 24
                or len(aggregates["cycle-provider-requests.jsonl"]) != 16 or len(aggregates["cycle-provider-responses.jsonl"]) != 16):
            raise ValueError("Gemini complete first capture lacks exact full matrices")
        if type(result.get("elapsed_ms")) is not int or not 0<=result["elapsed_ms"]<=180000:
            raise ValueError("Gemini complete first capture exceeded original total budget")
    return result


def _observation(row, paths):
    kind, label = row["stage"].split(":", 1);selected = scope(label)
    value = closed(row["value"], ("kind", "scope", "argv", "cwd", "exit", "error", "context_error", "started_at", "finished_at", "elapsed_ms", "cleanup", "stdout", "stderr"))
    if kind != value["kind"] or value["scope"] != selected or value["cwd"] != paths["outside" if selected == "user" else selected]: raise ValueError("Gemini native command scope differs")
    prefix_args = [paths["node"], paths["client"]]
    args = value["argv"]
    if kind == "listing":
        if args != prefix_args + ["skills", "list", "--all"]: raise ValueError("Gemini exact listing argv differs")
    elif (type(args) is not list or len(args) != 10 or args[:3] != prefix_args + ["--prompt"] or type(args[3]) is not str
          or args[4:] != ["--approval-mode", "default", "--output-format", "stream-json", "--model", "gemini-2.5-flash"]):
        raise ValueError("Gemini exact native argv differs")
    receipt({k: value[k] for k in ("argv", "cwd", "exit", "error", "context_error", "elapsed_ms", "cleanup")}, args, value["cwd"])
    start,end=(datetime.fromisoformat(value[k].replace("Z","+00:00")) for k in ("started_at","finished_at"))
    if start.tzinfo is None or end.tzinfo is None or end<start or value["elapsed_ms"]>180000:
        raise ValueError("Gemini original native interval differs")
    stream(value["stdout"]);stream(value["stderr"])


def _public_result(raw,label):
    value=closed(strict_json(raw),("schema_version","command","result","data"))
    verb={"repeat-add-a":"add","unchanged-update-a":"update","intact-repair-a":"repair","missing-repair-a":"repair"}.get(label,label.split("-")[0])
    if type(value["schema_version"]) is not int or value["schema_version"]!=1 or value["command"]!=verb or value["result"]!="success" or type(value["data"]) is not dict:
        raise ValueError("Gemini strict current public result differs")
    expected=label not in ("repeat-add-a","unchanged-update-a");found=[]
    def visit(node):
        if type(node) is dict:
            for key,item in node.items():
                if key=="mutated":
                    if type(item) is not bool or item is not expected: raise ValueError("Gemini public mutation truth differs")
                    found.append(item)
                visit(item)
        elif type(node) is list:
            for item in node: visit(item)
    visit(value)
    if not found: raise ValueError("Gemini public mutation receipt missing")


def progress(label):
    if label in ("add-user","add-a","add-b"): return b"Resolving and validating one Agent Plugin package for every selected target...\nInstalling uap-gemini-cycle for 1 selected clients...\n"
    if label=="repeat-add-a": return b"Resolving and validating one Agent Plugin package for every selected target...\nApplying transactional client package...\n"
    if label in ("unchanged-update-a","update-a"): return b"Resolving and validating one updated Agent Plugin package for every selected target...\n"
    if label in ("intact-repair-a","missing-repair-a"): return b"Resolving and validating each unique exact installed package revision once...\n"
    return b""


def _provider(row):
    value = closed(row["value"], ("scope", "operation", "tools", "native_call_id", "wire_call_id", "prompt", "sentinel", "requests", "responses", "issued", "returned", "error"))
    if value["scope"] != scope(row["stage"]) or value["operation"] not in ("none", "write", "read") or type(value["tools"]) is not list or len(value["tools"]) > 2 or len(set(value["tools"])) != len(value["tools"]):
        raise ValueError("Gemini provider finite operation/tools differ")
    if any(type(value[k]) is not bool for k in ("issued", "returned", "error")) or any(type(value[k]) is not int or not 0 <= value[k] <= 2 for k in ("requests", "responses")):
        raise ValueError("Gemini provider finite receipt type differs")


def _provider_identity(row,recipes):
    value=row["value"];stage=row["stage"];selected=scope(stage)
    operation="write" if stage in ("installed-user","installed-a","installed-b") else "read" if stage in ("restarted-a","updated-a","repaired-a") else "none"
    names={"user":"uapgeminiu","a":"uapgeminia","b":"uapgeminib"}
    expected=[] if stage in ("baseline-a","removed-user") else ["mcp_uapgeminiu_inspect_runtime"]
    if stage in ("installed-a","installed-b","restarted-a","updated-a","repaired-a"): expected.append("mcp_"+names[selected]+"_inspect_runtime")
    nonce=recipes[selected]["nonce"]
    from project_native_gemini_cycle_proof import digest
    wire="" if operation=="none" else "uap_cycle_"+digest((stage+nonce).encode())[:24]
    native="" if operation=="none" else "mcp_"+names[selected]+"_inspect_runtime__"+wire
    if (value["operation"]!=operation or value["tools"]!=expected or value["wire_call_id"]!=wire or value["native_call_id"]!=native
            or value["prompt"]!="Disposable Gemini fixture stage "+stage+". Call only the preselected installed inspect_runtime tool when requested."
            or value["sentinel"]!="UAP_GEMINI_DONE_"+stage+"_"+nonce):
        raise ValueError("Gemini provider held stage/tool/nonce/native-wire identity differs")


def members(bodies, source, result):
    try:
        return _members(bodies, source, result)
    except (KeyError, TypeError, AttributeError, IndexError, OverflowError, UnicodeError, RecursionError) as error:
        raise ValueError("Gemini cycle structural refusal") from error
