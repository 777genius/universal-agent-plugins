"""Source-guided, always-unqualified Kimi single installed tool capture.

The unchanged service owns startup bearer quarantine, bounded REST capture and
ordinary shutdown. No probe connection, injected MCP server, WebSocket framework
or vendor credential is introduced by this observer.
"""
import hashlib
import json
import math
from pathlib import Path
import re
import sys
import time
from urllib.parse import urlencode

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))
import project_native_research as shared
from project_native_collectors import kimi

TOOL = "mcp__uap-kimi-native-a__inspect_runtime"
MODEL = "uap-kimi-native-model"
SOURCE_COMMIT = "b8dd3a134060c6e80ba4a1659335cd63b68483f2"


class ResponseSchemaRefusal(ValueError):
    """Only a finite response shape can enter the graceful failed capture."""

    def __init__(self, stage):
        if stage not in ("session", "prompt", "approval", "snapshot", "history"):
            raise ValueError("Kimi unregistered response-schema stage")
        self.stage = stage
        super().__init__("Kimi finite response schema refused")


def response_object(value, required, optional=(), *, stage):
    # Keep identity, permission and value checks outside this narrow wrapper.
    try:
        return kimi._object(value, required, optional)
    except ValueError:
        raise ResponseSchemaRefusal(stage) from None


def strict_json(body):
    value = kimi._json(body)
    nodes = 0

    def visit(item, depth=0):
        nonlocal nodes
        nodes += 1
        if nodes > 16384 or depth > 32 or (isinstance(item, float) and not math.isfinite(item)):
            raise ValueError("Kimi finite bounded JSON structure differs")
        if isinstance(item, dict):
            for child in item.values():
                visit(child, depth + 1)
        elif isinstance(item, list):
            for child in item:
                visit(child, depth + 1)

    visit(value)
    return value


def _integer(value, low=0):
    if type(value) is not int or value < low:
        raise ValueError("Kimi exact integer identity differs")
    return value


def session(value, workspace, cwd):
    if isinstance(value, dict) and (
            not isinstance(value.get("id"), str) or not re.fullmatch(r"session_[A-Za-z0-9_-]{1,128}", value["id"])
            or value.get("workspace_id") != workspace or not isinstance(value.get("metadata"), dict)
            or value["metadata"].get("cwd") != str(cwd) or type(value.get("busy")) is not bool
            or value["busy"] or value.get("permission_rules") != []):
        raise ValueError("Kimi new session authority differs")
    response_object(value, ("id", "workspace_id", "title", "created_at", "updated_at", "busy", "metadata",
                         "agent_config", "usage", "permission_rules", "message_count", "last_seq"),
                 ("main_turn_active", "pending_interaction", "last_turn_reason", "archived", "archived_at",
                  "current_prompt_id", "last_prompt"), stage="session")
    kimi._string(value["title"], 4096, empty=True)
    kimi._instant(value["created_at"])
    kimi._instant(value["updated_at"])
    _integer(value["message_count"])
    _integer(value["last_seq"])
    return value["id"]


def approval(value, session_id, call_id, args):
    # Exact source fallback: toolApprovalService blob96f36f15ec9fcaf00f4ec373e935004ed14857e2.
    action = "Approve " + TOOL
    if isinstance(value, dict):
        kimi._string(value.get("approval_id"), 256)
        _integer(value.get("turn_id"), 1)
        if (value.get("session_id") != session_id or value.get("agent_id") != "main"
                or value.get("tool_call_id") != call_id or value.get("tool_name") != TOOL
                or value.get("action") != action or value.get("tool_input_display") != {
                    "kind": "generic", "summary": action, "detail": args}
                or not re.fullmatch(r"approval_[A-Za-z0-9_-]{1,128}", value["approval_id"])):
            raise ValueError("Kimi approval does not join the single provider-owned call")
    response_object(value, ("approval_id", "session_id", "agent_id", "tool_call_id", "tool_name", "action",
                         "tool_input_display", "created_at", "expires_at"), ("turn_id",), stage="approval")
    kimi._instant(value["created_at"])
    kimi._instant(value["expires_at"])
    return value["approval_id"], value["turn_id"]


def no_call(ctx, event_path, marker_path):
    body = kimi._read(ctx.root, event_path, 1 << 20, missing=True)
    for line in body.splitlines():
        value = kimi._object(strict_json(line), ("method", "pid", "session", "nonce", "timestamp", "facts"))
        if value["method"] not in ("initialize", "tools/list") or value["facts"] is not None:
            raise ValueError("Kimi probe called before native manual approval")
    if marker_path.exists() or marker_path.is_symlink():
        raise ValueError("Kimi marker exists before native manual approval")


def observe(ctx, request, diagnostic=None):
    diagnostic = diagnostic or kimi._KimiFailureDiagnostic("single-tool")
    diagnostic.ctx = ctx
    diagnostic.step("request-validate")
    kimi._object(request, ("server", "skill", "call_id", "provider_key", "nonce", "marker", "data"))
    server, skill = request["server"], request["skill"]
    kimi._object(skill, ("name", "description", "path", "source", "sha256"))
    kimi._object(server, ("name", "origin", "config", "sha256", "event", "nonce", "cwd"))
    diagnostic.step("authority")
    if server["name"] != "uap-kimi-native-a" or server["nonce"] != request["nonce"]:
        raise ValueError("Kimi selected installed A identity differs")
    event_path, marker_path = Path(server["event"]), Path(request["data"]) / "native-marker.txt"
    config_path = Path(server["origin"])
    config_body = kimi._read(ctx.root, config_path, 1 << 20)
    if (hashlib.sha256(config_body).hexdigest() != server["sha256"]
            or strict_json(config_body).get("mcpServers", {}).get(server["name"]) != server["config"]):
        raise ValueError("Kimi receipt-selected A config differs")
    skill_body = kimi._read(ctx.root, Path(skill["path"]), 1 << 20)
    if hashlib.sha256(skill_body).hexdigest() != skill["sha256"]:
        raise ValueError("Kimi receipt-selected A skill bytes differ")
    diagnostic.step("pre-call")
    no_call(ctx, event_path, marker_path)
    args = {"nonce": request["nonce"], "operation": "write", "marker": request["marker"]}

    def run(service):
        diagnostic.step("ready")
        token, count = kimi._ready(ctx, service), 0

        def api(label, method, path, body=None, *, auth=True, status=200, code=0):
            nonlocal count
            diagnostic.api(label, "enter")
            count += 1
            if count > 60 or (label != "shutdown" and count > 59):
                raise ValueError("Kimi single original HTTP inventory exhausted")
            diagnostic.api(label, "request")
            result_status, result_body = service.http(label, method, path, token=token if auth else None,
                                  json_body=body, timeout=min(5, service.remaining()))
            diagnostic.api(label, "envelope")
            strict_json(result_body)
            result = kimi._envelope(result_status, result_body, expected_status=status, code=code)
            diagnostic.api(label, "validate")
            return result

        if kimi._object(api("health", "GET", "/api/v1/healthz", auth=False), ("ok",))["ok"] is not True:
            raise ValueError("Kimi native health differs")
        api("unauthorized", "GET", "/api/v1/meta", auth=False, status=401, code=40101)
        meta = api("meta", "GET", "/api/v1/meta")
        kimi._meta(meta)
        workspaces = {}
        for scope in kimi.WORKSPACES:
            fixture = {"root": ctx.projects[scope], "name": "uap-kimi-native-" + scope}
            row = api("workspace-" + scope, "POST", "/api/v1/workspaces",
                      {"root": str(fixture["root"]), "name": fixture["name"]})
            workspaces[scope] = kimi._workspace(row, fixture)
            kimi._trust(api("trust-before-" + scope, "GET", "/api/v1/workspaces/" + workspaces[scope] + "/trust"), False)
        diagnostic.step("distinct-workspaces")
        if len(set(workspaces.values())) != 3:
            raise ValueError("Kimi workspace IDs are not distinct")
        for scope in ("a", "b"):
            path = "/api/v1/workspaces/" + workspaces[scope] + "/trust"
            kimi._trust(api("trust-set-" + scope, "POST", path, {}), True)
            kimi._trust(api("trust-after-" + scope, "GET", path), True)
        kimi._trust(api("trust-untrusted-c", "GET", "/api/v1/workspaces/" + workspaces["c"] + "/trust"), False)
        for scope in (*kimi.WORKSPACES, "user"):
            expected = [server] if scope == "a" else []
            kimi._installed_catalog(api("catalog-" + scope, "GET", "/api/v2/mcp/servers?" +
                                    urlencode({"cwd": str(ctx.projects[scope])})), expected)
        kimi._installed_skills(api("skills-a", "GET", "/api/v1/workspaces/" + workspaces["a"] + "/skills"), [skill])
        validated = {"provider_call_id": request["call_id"]}

        def session_capture():
            native_session = session(api("create-session", "POST", "/api/v1/sessions",
                                        {"metadata": {"cwd": str(ctx.projects["a"])}}), workspaces["a"], ctx.projects["a"])
            validated["native_session_id"] = native_session
            base = "/api/v1/sessions/" + native_session
            text = "Call the one installed fixture tool once with the exact nonce and marker; then report completion. " + request["nonce"]
            prompt = api("prompt", "POST", base + "/prompts", {"content": [{"type": "text", "text": text}],
                         "model": MODEL, "permission_mode": "manual"})
            if isinstance(prompt, dict):
                if prompt.get("content") != [{"type": "text", "text": text}] or prompt.get("status") not in ("running", "queued", "blocked"):
                    raise ValueError("Kimi accepted prompt differs")
                kimi._string(prompt.get("prompt_id"), 256)
                kimi._string(prompt.get("user_message_id"), 256)
            response_object(prompt, ("prompt_id", "user_message_id", "status", "content", "created_at"), ("metadata",), stage="prompt")
            kimi._instant(prompt["created_at"])
            matched = None
            for index in range(20):
                diagnostic.step("pre-call")
                no_call(ctx, event_path, marker_path)
                pending = api("approvals-" + str(index), "GET", base + "/approvals?status=pending")
                candidate = None
                if isinstance(pending, dict) and isinstance(pending.get("items"), list):
                    if len(pending["items"]) > 1:
                        raise ValueError("Kimi has foreign or multiple pending approvals")
                    if pending["items"]:
                        # An extra container field must not mask a mismatched
                        # session/tool/permission in its already known entry.
                        candidate = approval(pending["items"][0], native_session, request["call_id"], args)
                response_object(pending, ("items",), stage="approval")
                if not isinstance(pending["items"], list):
                    raise ResponseSchemaRefusal("approval")
                if len(pending["items"]) > 1:
                    raise ValueError("Kimi has foreign or multiple pending approvals")
                if pending["items"]:
                    matched = candidate
                    break
                time.sleep(min(0.25, service.remaining()))
            if matched is None:
                raise ValueError("Kimi exact native manual approval was not observed")
            diagnostic.step("pre-call")
            no_call(ctx, event_path, marker_path)
            validated["native_turn_id"], validated["approval_id"] = matched[1], matched[0]
            # Deliberately omit scope: no remembered or blanket tool approval.
            resolved = api("approve-once", "POST", base + "/approvals/" + matched[0], {"decision": "approved"})
            kimi._object(resolved, ("resolved", "resolved_at"))
            if resolved["resolved"] is not True:
                raise ValueError("Kimi one-call approval did not resolve")
            kimi._instant(resolved["resolved_at"])
            # These raw source-guided history shapes remain capture-only until a
            # genuine run freezes their finite result grammar in a separate review.
            snapshots = 0
            for index in range(10):
                snapshot = api("snapshot-" + str(index), "GET", base + "/snapshot")
                if isinstance(snapshot, dict) and (snapshot.get("pending_approvals") or snapshot.get("pending_questions")):
                    raise ValueError("Kimi has an additional unapproved interaction")
                response_object(snapshot, ("as_of_seq", "epoch", "session", "messages", "in_flight_turn", "pending_approvals", "pending_questions"), ("subagents",), stage="snapshot")
                snapshots += 1
                if snapshot["in_flight_turn"] is None:
                    break
                time.sleep(min(0.25, service.remaining()))
            history = api("history", "GET", base + "/messages?page_size=20")
            if isinstance(history, dict) and (history.get("has_more") is True or (
                    isinstance(history.get("items"), list) and len(history["items"]) > 20)):
                raise ValueError("Kimi bounded first-turn history differs")
            response_object(history, ("items", "has_more"), stage="history")
            if not isinstance(history["items"], list) or type(history["has_more"]) is not bool:
                raise ResponseSchemaRefusal("history")
            if len(history["items"]) > 20 or history["has_more"] is not False:
                raise ValueError("Kimi bounded first-turn history differs")
            return {"native_session_id": native_session, "native_turn_id": matched[1], "provider_call_id": request["call_id"],
                    "approval_id": matched[0], "api_request_count": count, "snapshot_count": snapshots,
                    "native_capture_complete": False, "native_history_grammar_frozen": False,
                    "boundary": "Bounded initial capture; no native history or qualification inferred from provider/probe success"}

        try:
            result = session_capture()
        except ResponseSchemaRefusal as refusal:
            # No later prompt, approval or tool action is permitted. The same
            # service still owns normal shutdown/reaping before any raw HTTP
            # can leave quarantine; identity and permission errors are not caught.
            result = {**validated, "response_schema_refused": True,
                      "failed_stage": refusal.stage, "native_capture_complete": False,
                      "native_history_grammar_frozen": False}
        diagnostic.step("preserved-authority")
        if kimi._read(ctx.root, config_path, 1 << 20) != config_body:
            raise ValueError("Kimi installed config changed during native tool attempt")
        shutdown = api("shutdown", "POST", "/api/v1/shutdown", {})
        if kimi._object(shutdown, ("ok",))["ok"] is not True:
            raise ValueError("Kimi native shutdown acknowledgement differs")
        service.mark_shutdown()
        result["api_request_count"] = count
        return result

    diagnostic.step("service-entry")
    return ctx.service("kimi-single-tool", ["web", "--no-open", "--host", "127.0.0.1", "--port", "0"],
                       diagnostic.observer(run), project="a", timeout=60)


def main(request_path, diagnostic=None):
    diagnostic = diagnostic or kimi._KimiFailureDiagnostic("single-tool")
    diagnostic.step("host-admission")
    shared.native.require_hosted("linux-amd64")
    diagnostic.step("request-read")
    path = Path(request_path)
    value = kimi._object(strict_json(kimi._read(path.parent, path, 1 << 20, mode=0o600)),
                         ("schema_version", "root", "home", "projects", "client", "identities", "observation"))
    diagnostic.step("request-validate")
    if type(value["schema_version"]) is not int or value["schema_version"] != 1:
        raise ValueError("Kimi attempt request version differs")
    diagnostic.step("authority")
    root, home, client = (Path(value[key]) for key in ("root", "home", "client"))
    projects = {key: Path(item) for key, item in value["projects"].items()}
    if (set(projects) != set((*kimi.WORKSPACES, "user")) or root.resolve() != root or not client.is_absolute()
            or client.name != "kimi" or any(item.resolve() != item or not item.is_relative_to(root) or item == root
                                           for item in (home, *projects.values())) or len({home, *projects.values()}) != 5
            or any(left.is_relative_to(right) for left in (home, *projects.values())
                   for right in (home, *projects.values()) if left != right)):
        raise ValueError("Kimi original profile/project authority differs")
    diagnostic.step("immutable-closure")
    identities = {Path(key): digest for key, digest in value["identities"].items()}
    if (client not in identities or not 3 <= len(identities) <= 128
            or any(not isinstance(digest, str) or not re.fullmatch(r"[0-9a-f]{64}", digest) for digest in identities.values())):
        raise ValueError("Kimi exact immutable runtime closure differs")
    key = value["observation"]["provider_key"]
    if not isinstance(key, str) or not re.fullmatch(r"uap-kimi-key-[0-9a-f]{32}", key):
        raise ValueError("Kimi private fixture key differs")
    diagnostic.step("context")
    ctx = shared.Context.__new__(shared.Context)
    ctx.root, ctx.home, ctx.projects, ctx.identities = root, home, projects, identities
    ctx.argv, ctx.secrets, ctx.captures = [str(client)], [], {}
    ctx.register_secret(key)  # Before any native startup or capture is possible.
    ctx.summary = {"schema_version": 1, "client": "kimi", "phase": "installed-single-tool-attempt",
                   "commands": [], "qualification": {name: False for name in shared.QUALIFICATION_CASES}}
    diagnostic.ctx = ctx
    ctx.env = kimi._installed_environment(root, home,
        {key: os.environ.get(key) for key in kimi._INSTALLED_ENVIRONMENT_KEYS})
    with shared.Budget(65) as budget:
        ctx.budget = budget
        ctx.summary["observations"] = observe(ctx, value["observation"], diagnostic)
        diagnostic.step("serialization")
        return emit_capture(ctx, diagnostic)


def emit_capture(ctx, diagnostic=None):
    # Context.service has already enforced unchanged graceful cleanup before
    # admitting any HTTP receipt. This is the same closed, redacted envelope
    # for success or the dedicated failed response-shape observation.
    bodies = shared.evidence(ctx.summary, ctx.captures, ctx.secrets)
    output = json.dumps({"schema_version": 1, "files": {name: body.decode("utf-8", "strict") for name, body in bodies.items()}},
                        allow_nan=False, sort_keys=True).encode()
    if len(output) > 1 << 20 or any(secret in output for secret in ctx.secrets):
        raise ValueError("Kimi private closed API evidence differs")
    if diagnostic is not None:
        diagnostic.step("emission")
    sys.stdout.buffer.write(output + b"\n")
    return 1 if ctx.summary["observations"].get("response_schema_refused") is True else 0


if __name__ == "__main__":
    if len(sys.argv) != 3 or sys.argv[1] != "--attempt-request":
        raise SystemExit("Kimi runtime attempt requires its exact private request")
    diagnostic = kimi._KimiFailureDiagnostic("single-tool")
    try:
        status = main(sys.argv[2], diagnostic)
    except Exception as error:
        kimi._emit_failure_diagnostic(diagnostic, error, sys.stderr)
        raise SystemExit("Kimi single-tool observation failed; private native streams quarantined") from None
    raise SystemExit(status)
