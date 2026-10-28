"""Run a closed current-source native test on an actual disposable hosted runner.

Acquisition and bounded process capture reuse the existing research runner. A
valid artifact never changes a failed test into a passed qualification.
"""
import argparse
import base64
import hashlib
import io
import json
import math
import os
from pathlib import Path
import re
import secrets
import shutil
import stat
import sys
import time
import zipfile

import project_native_research as shared
import project_native_cline_cycle as cline_cycle_success

SOURCE = shared.SOURCE
CASES = {
    "cline": {"test": "TestAgentpluginsClineNativeProjectCompleteCycle",
              "file": "repotests/agentplugins_cline_cycle_lifecycle_test.go",
              "optin": "AGENTPLUGINS_CLINE_PROJECT_RUNTIME_ATTEMPT", "seconds": 180,
              "phase": "runtime-attempt"},
    "copilot": {"test": "TestAgentpluginsCopilotNativeProjectRuntimeAttempt",
                "file": "repotests/agentplugins_copilot_project_native_attempt_test.go",
                "optin": "AGENTPLUGINS_COPILOT_PROJECT_RUNTIME_ATTEMPT", "seconds": 180,
                "phase": "runtime-attempt"},
    "gemini": {"test": "TestAgentpluginsGeminiNativeProjectRuntimeAttempt",
               "file": "repotests/agentplugins_gemini_project_native_attempt_test.go",
               "optin": "AGENTPLUGINS_GEMINI_PROJECT_RUNTIME_ATTEMPT", "seconds": 180,
               "phase": "runtime-attempt"},
    "kimi": {"test": "TestAgentpluginsKimiNativeProjectComponentsLifecycle",
             "file": "repotests/agentplugins_kimi_project_native_lifecycle_test.go",
             "optin": "AGENTPLUGINS_KIMI_PROJECT_NATIVE_E2E", "seconds": 900,
             "phase": "installed-components-lifecycle"},
    "kimi-call": {"test": "TestAgentpluginsKimiNativeProjectRuntimeAttempt",
                  "file": "repotests/agentplugins_kimi_project_native_attempt_test.go",
                  "optin": "AGENTPLUGINS_KIMI_PROJECT_RUNTIME_ATTEMPT", "seconds": 180,
                  "phase": "installed-single-tool-attempt"},
    "qoder": {"test": "TestAgentpluginsQoderNativeProjectRuntimeAttempt",
              "file": "repotests/agentplugins_qoder_project_native_attempt_test.go",
              "optin": "AGENTPLUGINS_QODER_PROJECT_RUNTIME_ATTEMPT", "seconds": 180, "phase": "runtime-attempt"},
    "grok": {"test": "TestAgentpluginsGrokNativeProjectRuntimeAttempt",
             "file": "repotests/agentplugins_grok_project_native_attempt_test.go",
             "optin": "AGENTPLUGINS_GROK_PROJECT_RUNTIME_ATTEMPT", "seconds": 180, "phase": "runtime-attempt"},
    "kiro": {"test": "TestAgentpluginsKiroNativeProjectRuntimeAttempt",
             "file": "repotests/agentplugins_kiro_project_native_attempt_test.go",
             "optin": "AGENTPLUGINS_KIRO_PROJECT_RUNTIME_ATTEMPT", "seconds": 180, "phase": "runtime-attempt"},
    "windsurf": {"test": "TestAgentpluginsDevinNativeProjectRuntimeAttempt",
                 "file": "repotests/agentplugins_windsurf_project_native_attempt_test.go",
                 "optin": "AGENTPLUGINS_WINDSURF_PROJECT_RUNTIME_ATTEMPT", "seconds": 180, "phase": "runtime-attempt"},
}
CLI_ATTEMPTS = {"qoder": "1.1.78", "grok": "1.0.70", "kiro": "2.34.0", "windsurf": "3000.11.205"}
MAX_ZIP = 32 << 20
MAX_MEMBER = 2 << 20
COMMANDS = ("git-A", "git-B", "version", "add-A", "native-attempt")
KIMI_COMMANDS = ("git-a", "git-b", "git-c", "version", "add-user", "add-a", "add-b", "add-c",
                 "observe-empty", "observe-installed", "observe-stable", "observe-updated", "observe-repaired",
                 "observe-restart", "observe-removed", "repeat-add-a", "unchanged-update-a", "intact-repair-a",
                 "update-a", "missing-repair-a", "remove-a", "repeat-remove-a")
CLI_COMMANDS = ("git-a", "git-b", "version", "add-a", "native-attempt")
CLINE_COMMANDS = ("git-a", "git-b", "version", "add-a")
KIMI_CALL_COMMANDS = ("git-a", "git-b", "git-c", "version", "add-a", "native-attempt")
CLINE_ENV = frozenset("HOME USERPROFILE XDG_CONFIG_HOME XDG_CACHE_HOME XDG_STATE_HOME XDG_DATA_HOME "
    "NPM_CONFIG_CACHE NPM_CONFIG_USERCONFIG NPM_CONFIG_GLOBALCONFIG GIT_CONFIG_GLOBAL GIT_CONFIG_NOSYSTEM "
    "GIT_TERMINAL_PROMPT PATH TMPDIR LANG NO_COLOR TERM GITHUB_ACTIONS RUNNER_ENVIRONMENT "
    "AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED AGENTPLUGINS_NATIVE_DISPOSABLE_LINUX "
    "AGENTPLUGINS_INSTALLER_BIN AGENTPLUGINS_INSTALLER_COMMIT AGENTPLUGINS_INSTALLER_TREE "
    "AGENTPLUGINS_NATIVE_PROBE_BIN AGENTPLUGINS_LINTAI_BIN AGENTPLUGINS_LINTAI_SHA256 "
    "AGENTPLUGINS_LINTAI_ARCHIVE_SHA256 AGENTPLUGINS_CLINE_BIN AGENTPLUGINS_PROJECT_NATIVE_IDENTITY "
    "AGENTPLUGINS_PROJECT_NATIVE_OUTPUT AGENTPLUGINS_CLINE_PROJECT_RUNTIME_ATTEMPT PLUGIN_KIT_AI_REPO_ROOT".split())
CLINE_UNIT = re.compile(r"uap-cline-project-[0-9a-f]{32}\.service\Z")


def strict_json(body):
    def pairs(rows):
        value = {}
        for key, item in rows:
            if key in value:
                raise ValueError("duplicate JSON member")
            value[key] = item
        return value
    def finite(value):
        result = float(value)
        if not math.isfinite(result):
            raise ValueError("nonfinite JSON")
        return result
    return json.loads(body.decode("utf-8", errors="strict"), object_pairs_hook=pairs, parse_float=finite,
                      parse_constant=lambda _: (_ for _ in ()).throw(ValueError("nonfinite JSON")))


def ordinary_bytes(path, limit, *, owners=None):
    path = Path(path)
    before = path.lstat()
    if (not path.is_absolute() or path.resolve() != path or not stat.S_ISREG(before.st_mode)
            or before.st_nlink != 1 or before.st_uid not in (owners if owners is not None else (os.getuid(),))
            or not 0 <= before.st_size <= limit):
        raise ValueError("bounded ordinary owned file required")
    descriptor = shared.artifacts.open_member(path)
    try:
        held = os.fstat(descriptor)
        if not shared.artifacts.same_opened(before, held):
            raise ValueError("opened file identity differs")
        with os.fdopen(os.dup(descriptor), "rb") as stream:
            body = shared.bounded_read(stream, limit)
        if shared.artifacts.identity(os.fstat(descriptor)) != shared.artifacts.identity(held):
            raise ValueError("held file changed")
    finally:
        os.close(descriptor)
    if len(body) != before.st_size or shared.artifacts.identity(path.lstat()) != shared.artifacts.identity(before):
        raise ValueError("file changed while reading")
    return body


def write_new(path, body, mode=0o600):
    with path.open("xb") as stream:
        stream.write(body)
    path.chmod(mode)


def attempt_members(bodies, source, client="copilot"):
    required = {"runtime-attempt.json"}
    installed = {"copilot": {"mcp.json.installed", "uapcopilota-SKILL.md.installed"},
                 "gemini": {".gemini-settings.json.installed", "uapgeminia-SKILL.md.installed"}}
    if client not in installed:
        raise ValueError("unknown runtime attempt schema")
    allowed = required | {"probe-events.jsonl"} | installed[client]
    allowed |= {"provider-request-" + str(i) + ".json" for i in range(1, 9)}
    for label in COMMANDS:
        triple = {label + suffix for suffix in (".command.json", ".stdout.log", ".stderr.log")}
        allowed |= triple
        if set(bodies) & triple and not triple <= set(bodies):
            raise ValueError("incomplete command evidence triple")
        if triple <= set(bodies):
            row = strict_json(bodies[label + ".command.json"])
            if (not isinstance(row, dict) or set(row) != {"argv", "cwd", "exit", "elapsed_ms", "error", "context_error", "cleanup"}
                    or not isinstance(row["argv"], list) or not row["argv"]
                    or any(not isinstance(arg, str) for arg in row["argv"])
                    or not isinstance(row["cwd"], str) or type(row["exit"]) is not int
                    or type(row["elapsed_ms"]) is not int or type(row["error"]) is not bool
                    or type(row["context_error"]) is not bool):
                raise ValueError("invalid command receipt")
    if not required <= set(bodies) or set(bodies) - allowed:
        raise ValueError("unrecorded native artifact member")
    result = strict_json(bodies["runtime-attempt.json"])
    if (not isinstance(result, dict) or result.get("client") != client
            or result.get("provider_kind") != "scripted_loopback" or result.get("activation") != "prepared"
            or type(result.get("runtime_attempt_complete")) is not bool
            or (client == "copilot" and result["runtime_attempt_complete"] is not False)
            or result.get("vendor_authentication_qualified") is not False
            or result.get("real_model_qualified") is not False
            or result.get("qualification") != {key: False for key in shared.QUALIFICATION_CASES}
            or type(result.get("test_failed")) is not bool):
        raise ValueError("attempt must retain explicit unqualified state")
    identity = result.get("source_identity")
    if identity is not None and identity != {"installer_base_commit": source["source_commit"],
                                            "installer_tree": source["source_tree"],
                                            "installer_source_state": "committed", "installer_patch_sha256": []}:
        raise ValueError("native evidence source differs")
    numbers = sorted(int(name.removeprefix("provider-request-").removesuffix(".json"))
                     for name in bodies if name.startswith("provider-request-"))
    if numbers != list(range(1, len(numbers) + 1)):
        raise ValueError("provider evidence sequence has gaps")
    if client == "gemini" and not result["test_failed"]:
        successful = required | installed[client] | {"probe-events.jsonl"}
        successful |= {label + suffix for label in COMMANDS for suffix in (".command.json", ".stdout.log", ".stderr.log")}
        if (result["runtime_attempt_complete"] is not True or not successful <= set(bodies) or len(numbers) < 2
                or identity is None or result.get("provider_returned") is not True
                or not isinstance(result.get("native_session_id"), str) or not result["native_session_id"]
                or not isinstance(result.get("provider_call_id"), str) or not result["provider_call_id"]
                or not isinstance(result.get("probe_facts"), dict) or not result["probe_facts"]):
            raise ValueError("successful Gemini attempt is missing required runtime observations")
        for label in COMMANDS:
            row = strict_json(bodies[label + ".command.json"])
            if row["exit"] != 0 or row["error"] or row["context_error"]:
                raise ValueError("successful Gemini attempt contains a failed command")
    return result


def kimi_members(bodies, source):
    allowed = {"lifecycle.json"}
    for label in KIMI_COMMANDS:
        triple = {label + suffix for suffix in (".command.json", ".stdout.log", ".stderr.log")}
        allowed |= triple
        if set(bodies) & triple and not triple <= set(bodies):
            raise ValueError("incomplete Kimi command evidence triple")
        if triple <= set(bodies):
            row = strict_json(bodies[label + ".command.json"])
            if (not isinstance(row, dict) or set(row) != {"argv", "cwd", "exit", "error", "deadline", "cleanup"}
                    or not isinstance(row["argv"], list) or not row["argv"]
                    or any(not isinstance(arg, str) for arg in row["argv"])
                    or not isinstance(row["cwd"], str) or type(row["exit"]) is not int
                    or type(row["error"]) is not bool or type(row["deadline"]) is not bool):
                raise ValueError("invalid Kimi command receipt")
    if "lifecycle.json" not in bodies or set(bodies) - allowed:
        raise ValueError("unrecorded Kimi artifact member")
    result = strict_json(bodies["lifecycle.json"])
    if (not isinstance(result, dict) or result.get("client") != "kimi" or result.get("activation") != "prepared"
            or result.get("vendor_authentication_qualified") is not False or result.get("real_model_qualified") is not False
            or type(result.get("test_failed")) is not bool):
        raise ValueError("Kimi lifecycle evidence state differs")
    qualification = result.get("qualification")
    if (not isinstance(qualification, dict) or set(qualification) != set(shared.QUALIFICATION_CASES)
            or any(type(value) is not bool for value in qualification.values())
            or qualification["mcp-tools-call"] is not False):
        raise ValueError("Kimi cannot infer a tools-call qualification")
    if result["test_failed"] and any(qualification.values()):
        raise ValueError("failed Kimi lifecycle cannot qualify stages")
    if not result["test_failed"]:
        if set(bodies) != allowed or any(not qualification[key] for key in shared.QUALIFICATION_CASES if key != "mcp-tools-call"):
            raise ValueError("successful Kimi lifecycle must contain all selected stages")
    identity = result.get("source_identity")
    if identity is not None and identity != {"installer_base_commit": source["source_commit"], "installer_tree": source["source_tree"],
                                            "installer_source_state": "committed", "installer_patch_sha256": []}:
        raise ValueError("Kimi evidence source differs")
    return result


def grok_private_credential(body):
    if len(body) > 1 << 20 or b"uap-grok-private-" in body:
        return True
    budget = [4 * len(body), 16384]

    def numeric_refusal(raw, error):
        if str(error) != "nonfinite JSON" or len(raw) > budget[0]:
            return True
        budget[0] -= len(raw)
        try:
            json.loads(raw.decode("utf-8", errors="strict"), parse_int=lambda _: None, parse_float=lambda _: None,
                       parse_constant=lambda _: (_ for _ in ()).throw(json.JSONDecodeError("Non-JSON constant", "", 0)))
        except json.JSONDecodeError:
            return False
        except (UnicodeError, RecursionError, ValueError):
            return True
        return True

    def encoded(raw, depth):
        if depth > 64 or len(raw) > budget[0]:
            return True
        budget[0] -= len(raw)
        try:
            value = strict_json(raw)
        except (UnicodeError, RecursionError):
            return True
        except ValueError as error:
            if not isinstance(error, json.JSONDecodeError) and numeric_refusal(raw, error):
                return True
            if b"\n" not in raw:
                return False
            for line in io.BytesIO(raw):
                line = line.removesuffix(b"\n")
                if len(line) > budget[0]:
                    return True
                budget[0] -= len(line)
                try:
                    value = strict_json(line)
                except (UnicodeError, RecursionError):
                    return True
                except ValueError as line_error:
                    if not isinstance(line_error, json.JSONDecodeError) and numeric_refusal(line, line_error):
                        return True
                    continue
                if visit(value, depth):
                    return True
            return False
        return visit(value, depth)

    def visit(value, depth, structure=0):
        budget[1] -= 1
        if depth > 64 or structure > 32 or budget[1] < 0:
            return True
        if type(value) is str:
            try:
                raw = value.encode("utf-8", errors="strict")
            except UnicodeError:
                return True
            return b"uap-grok-private-" in raw or encoded(raw, depth + 1)
        if type(value) is dict:
            for key, item in value.items():
                try:
                    if len(key.encode("utf-8", errors="strict")) > 4096:
                        return True
                except UnicodeError:
                    return True
                if visit(key, depth + 1, structure + 1) or visit(item, depth + 1, structure + 1):
                    return True
        elif type(value) is list:
            return any(visit(item, depth + 1, structure + 1) for item in value)
        return False

    return encoded(body, 0)


def grok_provider_route_observation(row):
    if "route_observation" not in row:
        return
    observation = row["route_observation"]
    flags = {"peer_loopback", "host_matches", "relative_url", "empty_url_host", "empty_raw_path", "content_type_exact"}
    fields = flags | {"method", "uri", "content_type_count", "content_type_kind"}
    uris = {"/", "/v1/chat/completions", "/chat/completions", "/v1/models", "/models", "/v1/responses", "/responses", "other"}
    if (row["failure"] != "route" or not row["started"] or type(observation) is not dict or set(observation) != fields
            or any(type(observation[key]) is not bool for key in flags)
            or type(observation["method"]) is not str or observation["method"] not in {"POST", "GET", "other"}
            or type(observation["uri"]) is not str or observation["uri"] not in uris
            or type(observation["content_type_count"]) is not str
            or observation["content_type_count"] not in {"none", "one", "multiple"}
            or type(observation["content_type_kind"]) is not str
            or observation["content_type_kind"] not in {"missing", "json-bare", "json-utf8", "other"}):
        raise ValueError("finite Grok route observation differs")
    count, kind = observation["content_type_count"], observation["content_type_kind"]
    if ((count == "none") != (kind == "missing") or (count == "multiple" and kind != "other")
            or (kind in {"json-bare", "json-utf8"} and count != "one")
            or (observation["content_type_exact"] and (count != "one" or kind != "json-bare"))
            or (observation["method"] == "POST" and observation["uri"] == "/v1/chat/completions"
                and all(observation[key] for key in flags))):
        raise ValueError("contradictory Grok route observation")


def grok_provider_members(bodies, client):
    if client == "grok" and any(grok_private_credential(body) for body in bodies.values()):
        raise ValueError("unredacted Grok fixture credential refused")
    members = {"grok-provider.json", "grok-provider.request.json", "grok-provider.response.sse"}
    present = set(bodies) & members
    if not present:
        return set()
    if client != "grok" or "grok-provider.json" not in present:
        raise ValueError("Grok-only provider receipt required")
    if len(bodies["grok-provider.json"]) > 2048:
        raise ValueError("Grok provider receipt bound exceeded")
    row = strict_json(bodies["grok-provider.json"])
    fields = {"transport", "model", "started", "requests", "responses", "credential_matched",
              "failure", "shutdown_complete", "forced_close"}
    failures = {"none", "listen", "serve", "deadline", "route", "credential", "duplicate",
                "body", "request-shape", "response-write", "cleanup"}
    if (type(row) is not dict or set(row) not in (fields, fields | {"route_observation"})
            or row["transport"] != "scripted loopback OpenAI chat-completions SSE; no real model"
            or row["model"] != "uap-grok-fixture"
            or type(row["failure"]) is not str or row["failure"] not in failures
            or any(type(row[key]) is not bool for key in ("started", "credential_matched", "shutdown_complete", "forced_close"))
            or any(type(row[key]) is not int or not 0 <= row[key] <= 1 for key in ("requests", "responses"))
            or row["responses"] > row["requests"]):
        raise ValueError("finite Grok provider receipt differs")
    if ((not row["started"] and (row["failure"] != "listen" or row["requests"] or row["responses"]
                                 or row["credential_matched"] or row["shutdown_complete"] or row["forced_close"]))
            or (row["requests"] and (not row["started"] or not row["credential_matched"]))
            or (row["failure"] == "none" and (row["requests"] != row["responses"]
                                               or row["forced_close"] or not row["shutdown_complete"]))):
        raise ValueError("Grok partial capture or cleanup state differs")
    grok_provider_route_observation(row)
    required = {"grok-provider.json"}
    if row["requests"]:
        required.add("grok-provider.request.json")
    if row["responses"]:
        required.add("grok-provider.response.sse")
    if present != required or any(not bodies[name].strip() for name in present):
        raise ValueError("Grok provider member counts differ")
    if row["requests"]:
        request = strict_json(bodies["grok-provider.request.json"])
        if type(request) is not dict:
            raise ValueError("Grok opaque captured request must be a JSON object")
        if "uap-grok-private-" in json.dumps(request):
            raise ValueError("encoded Grok fixture credential refused")
    return members


GROK_TITLE_SYSTEM = (
    "You are tasked with generating the session title. The user is asking almost always software engineering related questions on their codebase.\n"
    "We describe the session title below\n# Session Title\n"
    "A short and distinctive 5-10 word descriptive title for the session. Super info dense, no filler.\n\n"
    "You will be given the user query below encapsulated in <user_query></user_query>.\n\n"
    "Just generate the session_title and nothing else")
EXPECTED_TITLE_REQUEST = {
    "model": "uap-grok-title-fixture", "stream": True, "stream_options": {"include_usage": True},
    "temperature": 1.0, "max_tokens": 100,
    "messages": [{"role": "system", "content": GROK_TITLE_SYSTEM},
                 {"role": "user", "content": "<user_query>\nDescribe only the installed uap-grok-attempt skill marker. Do not run any tool or change any file.\n</user_query>"}],
    "tools": [{"type": "function", "function": {
        "name": "session_title", "description": "Generate the session_title which we use for the user_message",
        "parameters": {"type": "object", "required": ["session_title"], "additionalProperties": False,
                       "properties": {"session_title": {"type": "string", "description": "Final session title, just 5-10 word descriptive title for the session. Super info dense, no filler."}}}}}],
    "tool_choice": {"type": "function", "function": {"name": "session_title"}}}


def grok_title_provider_members(bodies, client):
    names = {"grok-title-provider.json", "grok-title-provider.request.json", "grok-title-provider.response.sse"}
    present = set(bodies) & names
    if not present:
        return set()
    if client != "grok" or "grok-provider.json" not in bodies or "grok-title-provider.json" not in present:
        raise ValueError("separate Grok title/main receipts required")
    if len(bodies["grok-title-provider.json"]) > 2048:
        raise ValueError("Grok title receipt bound")
    row = strict_json(bodies["grok-title-provider.json"])
    fields = {"role", "transport", "model", "started", "requests", "responses", "credential_matched",
              "failure", "shutdown_complete", "forced_close"}
    failures = {"none", "listen", "serve", "deadline", "route", "credential", "duplicate",
                "body", "request-shape", "privacy", "response-write", "cleanup"}
    if (type(row) is not dict or set(row) not in (fields, fields | {"route_observation"})
            or row["role"] != "session-title"
            or row["transport"] != "scripted loopback OpenAI session-title SSE; no real model"
            or row["model"] != "uap-grok-title-fixture"
            or type(row["failure"]) is not str or row["failure"] not in failures
            or any(type(row[key]) is not bool for key in ("started", "credential_matched", "shutdown_complete", "forced_close"))
            or any(type(row[key]) is not int or not 0 <= row[key] <= 1 for key in ("requests", "responses"))
            or row["responses"] > row["requests"]):
        raise ValueError("finite Grok title receipt differs")
    if ((not row["started"] and (row["failure"] != "listen" or row["requests"] or row["responses"]
                                 or row["credential_matched"] or row["shutdown_complete"] or row["forced_close"]))
            or (row["requests"] and (not row["started"] or not row["credential_matched"]))
            or (row["failure"] == "none" and (row["requests"] != row["responses"]
                                               or row["forced_close"] or not row["shutdown_complete"]))
            or (row["started"] and row["failure"] == "listen")
            or (row["failure"] in {"body", "privacy"} and row["requests"])
            or (row["failure"] in {"request-shape", "privacy", "body", "response-write"} and row["responses"])):
        raise ValueError("Grok title partial capture or cleanup differs")
    grok_provider_route_observation(row)
    required = {"grok-title-provider.json"}
    if row["requests"]:
        required.add("grok-title-provider.request.json")
    if row["responses"]:
        required.add("grok-title-provider.response.sse")
    if present != required or any(not bodies[name].strip() for name in present):
        raise ValueError("Grok title member counts differ")
    request = None
    if row["requests"]:
        raw = bodies["grok-title-provider.request.json"]
        request = strict_json(raw)
        if len(raw) > 1 << 20 or type(request) is not dict or grok_private_credential(raw):
            raise ValueError("Grok title safe captured object required")
    if row["responses"]:
        expected = EXPECTED_TITLE_REQUEST
        response = bodies["grok-title-provider.response.sse"]
        if (request != expected or type(request["stream"]) is not bool
                or request["stream_options"]["include_usage"] is not True
                or request["tools"][0]["function"]["parameters"]["additionalProperties"] is not False
                or type(request["temperature"]) not in (int, float) or len(response) != 570
                or hashlib.sha256(response).hexdigest() != "eea6c4c389c7e27cf0e6d1cc9e705bc84a4d77ae8c5afddaa846ec8255e6ed68"):
            raise ValueError("Grok title fixed request/response differs")
    return names


def grok_duplicate_object(raw):
    """Finite refused JSON data only; never a provider/model request grammar."""
    if type(raw) is not bytes or not 0 < len(raw) <= 1 << 20:
        raise ValueError("Grok duplicate raw body bound")
    value = strict_json(raw)
    if type(value) is not dict:
        raise ValueError("Grok duplicate raw object required")
    nodes = 0
    def visit(item, depth):
        nonlocal nodes
        nodes += 1
        if depth > 32 or nodes > 16384:
            raise ValueError("Grok duplicate JSON structure bound")
        if type(item) is dict:
            for key, child in item.items():
                if len(key.encode("utf-8", errors="strict")) > 4096:
                    raise ValueError("Grok duplicate JSON key bound")
                visit(child, depth + 1)
        elif type(item) is list:
            for child in item: visit(child, depth + 1)
        elif type(item) in (int, float):
            try:
                finite = math.isfinite(float(item))
            except OverflowError:
                finite = False
            if not finite:
                raise ValueError("Grok duplicate nonfinite JSON number")
    visit(value, 0)
    decoded = json.dumps(value, ensure_ascii=False, separators=(",", ":")).encode("utf-8", errors="strict")
    if grok_private_credential(raw) or grok_private_credential(decoded):
        raise ValueError("Grok duplicate privacy refusal")
    return value


def grok_duplicate_members(bodies, client):
    names = {"grok-provider.duplicate.json", "grok-provider.duplicate-request.json"}
    present = set(bodies) & names
    if not present:
        return set()
    receipt = bodies.get("grok-provider.duplicate.json")
    if client != "grok" or type(receipt) is not bytes or not 0 < len(receipt) <= 2048:
        raise ValueError("Grok-only duplicate receipt required")
    row = strict_json(receipt)
    fields = {"schema_version", "role", "attempt_index", "status", "original_failure", "response_issued", "captured_bytes", "captured_sha256"}
    if (type(row) is not dict or set(row) != fields or type(row["schema_version"]) is not int or row["schema_version"] != 1
            or row["role"] != "main-duplicate-diagnostic" or type(row["attempt_index"]) is not int or row["attempt_index"] != 2
            or row["original_failure"] != "duplicate" or row["response_issued"] is not False
            or type(row["captured_bytes"]) is not int or type(row["captured_sha256"]) is not str
            or type(row["status"]) is not str or row["status"] not in {"captured", "body-refused", "json-refused", "privacy-refused", "context-refused"}):
        raise ValueError("Grok duplicate finite receipt differs")
    # Old main/title validators remain the canonical closed parent schemas.
    grok_provider_members(bodies, client)
    grok_title_provider_members(bodies, client)
    main = strict_json(bodies.get("grok-provider.json", b"null"))
    title = strict_json(bodies.get("grok-title-provider.json", b"null"))
    if (type(main) is not dict or main.get("requests") != 1 or main.get("responses") != 1 or main.get("failure") != "duplicate"
            or main.get("started") is not True or main.get("credential_matched") is not True
            or type(title) is not dict or title.get("role") != "session-title" or title.get("started") is not True):
        raise ValueError("Grok duplicate completed first main/title join absent")
    request = grok_duplicate_object(bodies["grok-provider.request.json"])
    if (request.get("model") != "uap-grok-fixture" or request.get("stream") is not True
            or type(request.get("messages")) is not list or not 1 <= len(request["messages"]) <= 128
            or hashlib.sha256(bodies["grok-provider.response.sse"]).hexdigest() != "f2c19ea9dbc0ddbf7cc2facaa5d2d01877897afa0f6fea33cc5651c0ecc9f7b9"):
        raise ValueError("Grok duplicate unchanged first exchange differs")
    result = strict_json(bodies.get("runtime-attempt.json", b"null"))
    if (type(result) is not dict or result.get("client") != "grok" or result.get("runtime_attempt_complete") is not False
            or result.get("test_failed") is not True or result.get("vendor_authentication_qualified") is not False
            or result.get("real_model_qualified") is not False or type(result.get("qualification")) is not dict
            or set(result["qualification"]) != set(shared.QUALIFICATION_CASES) or any(value is not False for value in result["qualification"].values())):
        raise ValueError("Grok duplicate remains failed and unqualified")
    if row["status"] == "captured":
        raw = bodies.get("grok-provider.duplicate-request.json")
        if (type(raw) is not bytes or not 0 < len(raw) <= 1 << 20 or row["captured_bytes"] != len(raw)
                or row["captured_sha256"] != hashlib.sha256(raw).hexdigest()):
            raise ValueError("Grok duplicate raw byte/hash join differs")
        grok_duplicate_object(raw)
    elif present != {"grok-provider.duplicate.json"} or row["captured_bytes"] != 0 or row["captured_sha256"] != "":
        raise ValueError("Grok duplicate refusal retained raw")
    return names


def grok_selection_identity_keys(row, fields):
    return type(row) is dict and all(key == expected or key.casefold() != expected.casefold()
                                   for key in row for expected in fields.split())


def grok_selected_skill(skills, name, path):
    if type(skills) is not list:
        raise ValueError("Grok selected skills must be an array")
    found = False
    for row in skills:
        if (not grok_selection_identity_keys(row, "name description source userInvocable disabled")
                or type(row.get("name")) is not str or not row["name"]
                or not grok_selection_identity_keys(row.get("source"), "type plugin_name path")):
            raise ValueError("Grok selected skill identity differs")
        origin = row["source"]
        plugin = origin.get("plugin_name", "")
        if row["name"].casefold() != name.casefold() and (type(plugin) is not str or plugin.casefold() != name.casefold()):
            continue
        if (found or row["name"] != name or row.get("description") != "Harmless installed startup marker"
                or row.get("userInvocable") is not True or row.get("disabled", False) is not False
                or origin != {"type": "plugin", "plugin_name": name, "path": path}):
            raise ValueError("Grok selected skill source is not the owned installation")
        found = True
    if not found:
        raise ValueError("Grok selected loaded skill is absent")


def grok_selected_inspection(bodies, result, client):
    if client != "grok":
        if "fixture_selection" in result:
            raise ValueError("Grok fixture selection cannot apply to another client")
        return
    marker, observation = result.get("fixture_selection"), result.get("native_inspection")
    if "fixture_selection" not in result:
        if type(observation) is dict and (observation.get("enabled") is True or observation.get("project_trusted") is True):
            raise ValueError("Grok selected observation requires its fixture marker")
        return
    if marker != "owned-project-trust-and-selected-plugin-name":
        raise ValueError("unregistered Grok fixture selection")
    # Authored intent is not a completed observation. Preserve failed/unknown
    # raw inspect attempts without inventing a successful selected skill.
    if "native_inspection" not in result:
        return
    binding = result.get("binding")
    if type(binding) is not dict:
        raise ValueError("Grok selected inspection requires its project binding")
    profile, target, physical = (binding.get(key) for key in ("native_profile_root", "target_locator", "physical_artifact_id"))
    if (any(type(value) is not str or not value or not os.path.isabs(value) or os.path.normpath(value) != value for value in (profile, target))
            or type(physical) is not str or re.fullmatch(r"[a-z0-9][a-z0-9.-]{0,63}", physical) is None
            or binding.get("client_id") != "grok" or binding.get("scope") != "project"
            or binding.get("activation") != "prepared" or binding.get("verification") != "package_validated"
            or Path(profile).name != ".grok" or target != str(Path(profile) / "plugins" / physical)):
        raise ValueError("Grok selected binding identity differs")
    project, name = str(Path(profile).parent), "uap-grok-attempt"
    skill = str(Path(target) / "skills" / name / "SKILL.md")
    expected = {"name": name, "scope": "project", "path": target, "enabled": True, "project_trusted": True,
                "installed_skill": skill, "interpretation": "exact selected plugin skill listed; no skill execution or lifecycle/runtime qualification"}
    if (type(observation) is not dict or observation != expected or observation.get("enabled") is not True
            or observation.get("project_trusted") is not True or result.get("installed_skill") != skill):
        raise ValueError("Grok completed selected observation differs")
    raw = bodies.get("native-inspect.stdout.log")
    command = strict_json(bodies.get("native-inspect.command.json", b"null"))
    version = strict_json(bodies.get("version.command.json", b"null"))
    if (type(command) is not dict or type(version) is not dict or type(version.get("argv")) is not list
            or len(version["argv"]) != 2 or version["argv"][1] != "--version"
            or command.get("argv") != [version["argv"][0], "inspect", "--json"] or command.get("cwd") != project
            or command.get("exit") != 0 or command.get("error") is not False or command.get("deadline") is not False
            or bodies.get("native-inspect.stderr.log") != b"" or type(raw) is not bytes):
        raise ValueError("Grok selected observation lacks its successful exact command")
    inspect = strict_json(raw)
    if (not grok_selection_identity_keys(inspect, "grokVersion cwd projectRoot projectTrusted skills plugins")
            or inspect.get("grokVersion") != "1.0.70" or inspect.get("cwd") != project
            or inspect.get("projectRoot") not in (project, project + "/") or inspect.get("projectTrusted") is not True
            or type(inspect.get("plugins")) is not list):
        raise ValueError("Grok selected raw project inspection differs")
    seen, found = set(), False
    for plugin in inspect["plugins"]:
        if (not grok_selection_identity_keys(plugin, "name scope path enabled") or type(plugin.get("name")) is not str
                or not plugin["name"].strip() or plugin["name"].casefold() in seen
                or plugin.get("scope") not in ("project", "user") or type(plugin.get("path")) is not str
                or not os.path.isabs(plugin["path"]) or os.path.normpath(plugin["path"]) != plugin["path"]
                or type(plugin.get("enabled")) is not bool):
            raise ValueError("Grok selected raw plugin identity differs")
        seen.add(plugin["name"].casefold())
        if plugin["name"].casefold() == name.casefold():
            if plugin["name"] != name or plugin["scope"] != "project" or plugin["path"] != target or plugin["enabled"] is not True:
                raise ValueError("Grok selected raw plugin differs")
            found = True
    if not found:
        raise ValueError("Grok selected raw plugin is absent")
    grok_selected_skill(inspect.get("skills"), name, skill)


def cli_attempt_members(bodies, source, client):
    if client not in CLI_ATTEMPTS:
        raise ValueError("unknown fixed native CLI attempt")
    allowed = {"runtime-attempt.json", "probe-events.jsonl"}
    allowed |= grok_provider_members(bodies, client)
    allowed |= grok_title_provider_members(bodies, client)
    allowed |= grok_duplicate_members(bodies, client)
    commands = CLI_COMMANDS
    if client == "qoder":
        commands += ("native-catalog",)
    elif client == "grok":
        commands += ("native-inspect",)
    for label in commands:
        triple = {label + suffix for suffix in (".command.json", ".stdout.log", ".stderr.log")}
        allowed |= triple
        if set(bodies) & triple and not triple <= set(bodies):
            raise ValueError("incomplete native CLI command triple")
        if triple <= set(bodies):
            row = strict_json(bodies[label + ".command.json"])
            if (not isinstance(row, dict) or set(row) != {"argv", "cwd", "exit", "error", "deadline", "elapsed_ms", "cleanup"}
                    or not isinstance(row["argv"], list) or not row["argv"]
                    or any(not isinstance(value, str) for value in row["argv"])
                    or not isinstance(row["cwd"], str) or type(row["exit"]) is not int
                    or type(row["error"]) is not bool or type(row["deadline"]) is not bool
                    or type(row["elapsed_ms"]) is not int or row["elapsed_ms"] < 0
                    or row["cleanup"] != "full descendant supervision; forced cleanup fails"):
                raise ValueError("native CLI command receipt differs")
    if "runtime-attempt.json" not in bodies or set(bodies) - allowed:
        raise ValueError("unrecorded native CLI attempt member")
    result = strict_json(bodies["runtime-attempt.json"])
    if (not isinstance(result, dict) or result.get("client") != client or result.get("version") != CLI_ATTEMPTS[client]
            or result.get("runtime_attempt_complete") is not False or result.get("test_failed") is not True
            or result.get("activation") != "prepared" or result.get("vendor_authentication_qualified") is not False
            or result.get("real_model_qualified") is not False):
        raise ValueError("initial native CLI attempt cannot qualify an unreviewed boundary")
    qualification = result.get("qualification")
    if (not isinstance(qualification, dict) or set(qualification) != set(shared.QUALIFICATION_CASES)
            or any(value is not False for value in qualification.values())):
        raise ValueError("native CLI attempt qualification must remain false")
    if result.get("source_identity") != {"installer_base_commit": source["source_commit"], "installer_tree": source["source_tree"],
                                        "installer_source_state": "committed", "installer_patch_sha256": []}:
        raise ValueError("native CLI evidence source differs")
    grok_selected_inspection(bodies, result, client)
    return result


CLINE_CYCLE_PUBLIC = ("git-a", "git-b", "version", "add-user", "add-a", "add-b", "repeat-add-a",
    "unchanged-update-a", "intact-repair-a", "update-a", "missing-repair-a", "remove-a", "repeat-remove-a", "remove-b", "remove-user")
CLINE_CYCLE_LISTINGS = ("empty-a", "empty-b", "installed-user", "installed-a", "installed-b", "stable-a",
    "updated-a", "repaired-a", "removed-a", "remaining-b", "removed-b", "removed-user", "final-a", "final-b")
CLINE_CYCLE_ACP = ("empty-a", "empty-b", "installed-a", "installed-b", "updated-a", "repaired-a",
    "removed-a", "remaining-b", "final-a", "final-b")
CLINE_CYCLE_EMPTY = frozenset(("empty-a", "empty-b", "removed-a", "final-a", "final-b"))
CLINE_CYCLE_PROOF = ("source-user-user-one", "source-a-a-one", "source-b-b-one", "add-user", "add-a", "add-b",
    "repeat-add-a", "unchanged-update-a", "intact-repair-a", "before-source-revision-a", "source-a-a-two",
    "update-a", "before-missing-a", "missing-repair-a", "remove-a", "repeat-remove-a", "remove-b", "remove-user")
CLINE_CYCLE_AGGREGATES = ("cycle-native-acp.jsonl", "cycle-native-commands.jsonl", "cycle-discovery.jsonl",
    "cycle-provider-requests.jsonl", "cycle-provider-responses.jsonl", "cycle-provider-receipts.jsonl",
    "cycle-probe-events.jsonl", "cycle-proof.jsonl")


def cline_cycle_complete_numeric_json(raw, budget):
    if len(raw) > budget[0]:
        return True
    budget[0] -= len(raw)
    def reject_constant(_):
        raise json.JSONDecodeError("nonstandard JSON constant", "", 0)
    try:
        json.loads(raw.decode("utf-8", errors="strict"), parse_int=lambda _: 0,
                   parse_float=lambda _: 0, parse_constant=reject_constant)
    except json.JSONDecodeError:
        return False
    except (UnicodeError, RecursionError, ValueError):
        return True
    return True


def cline_cycle_private(body):
    if len(body) > 1 << 20 or b"uap-cline-private-" in body:
        return True
    budget = [4 * len(body), 16384]
    def encoded(raw, depth):
        if depth > 64 or len(raw) > budget[0]:
            return True
        budget[0] -= len(raw)
        try:
            value = strict_json(raw)
        except UnicodeError:
            return True
        except RecursionError:
            return True
        except ValueError as error:
            if str(error) == "nonfinite JSON":
                if cline_cycle_complete_numeric_json(raw, budget):
                    return True
            elif not isinstance(error, json.JSONDecodeError):
                return True
            if b"\n" not in raw:
                return False
            for line in io.BytesIO(raw):
                if len(line) > budget[0]:
                    return True
                budget[0] -= len(line)
                try:
                    value = strict_json(line)
                except (UnicodeError, RecursionError):
                    return True
                except ValueError as line_error:
                    if str(line_error) == "nonfinite JSON":
                        if cline_cycle_complete_numeric_json(line, budget):
                            return True
                    elif not isinstance(line_error, json.JSONDecodeError):
                        return True
                    continue
                if visit(value, depth):
                    return True
            return False
        return visit(value, depth)
    def visit(value, depth, document_depth=0):
        budget[1] -= 1
        # Mirror the preserved Go strict decoder's per-document structure cap;
        # depth64 separately bounds traversal across nested JSON strings.
        if depth > 64 or document_depth > 32 or budget[1] < 0:
            return True
        if type(value) is str:
            try:
                raw = value.encode("utf-8", errors="strict")
            except UnicodeError:
                return True
            if b"uap-cline-private-" in raw:
                return True
            return encoded(raw, depth + 1)
        if type(value) is dict:
            try:
                invalid_key = any(len(key.encode("utf-8", errors="strict")) > 4096 for key in value)
            except UnicodeError:
                return True
            if invalid_key:
                return True
            return any(visit(key, depth + 1, document_depth + 1) or visit(item, depth + 1, document_depth + 1) for key, item in value.items())
        if type(value) is list:
            return any(visit(item, depth + 1, document_depth + 1) for item in value)
        return False
    return encoded(body, 0)


def cline_cycle_stream(row):
    if (type(row) is not dict or set(row) != {"bytes", "sha256", "body"}
            or type(row["bytes"]) is not int or not 0 <= row["bytes"] <= 1 << 20
            or not isinstance(row["body"], str) or len(row["body"]) > (1 << 20) * 4 // 3 + 4
            or not isinstance(row["sha256"], str) or re.fullmatch(r"[0-9a-f]{64}", row["sha256"]) is None):
        raise ValueError("closed Cline cycle raw stream receipt required")
    try:
        body = base64.b64decode(row["body"], validate=True)
    except ValueError as error:
        raise ValueError("Cline cycle raw base64 differs") from error
    if len(body) != row["bytes"] or hashlib.sha256(body).hexdigest() != row["sha256"] or cline_cycle_private(body):
        raise ValueError("Cline cycle raw bytes/hash/private canary differs")
    return body


def cline_cycle_rows(body):
    if len(body) > 1 << 20 or (body and not body.endswith(b"\n")) or cline_cycle_private(body):
        raise ValueError("bounded complete Cline cycle aggregate required")
    rows = []
    for line in io.BytesIO(body):
        row = strict_json(line)
        if (len(rows) >= 128 or type(row) is not dict or set(row) != {"stage", "value"}
                or not isinstance(row["stage"], str) or type(row["value"]) is not dict):
            raise ValueError("closed Cline cycle stage record required")
        rows.append(row)
    return rows


def cline_cycle_prefix(actual, expected):
    return type(actual) is list and actual == list(expected[:len(actual)])


def cline_cycle_records(rows, order, repeat=False):
    seen, previous = set(), -1
    for row in rows:
        if row["stage"] not in order:
            raise ValueError("unregistered Cline cycle stage")
        index = order.index(row["stage"])
        if index < previous or index > previous + 1 or (not repeat and index == previous):
            raise ValueError("Cline cycle skipped, duplicate or reordered stage")
        previous = index
        seen.add(row["stage"])
    return seen


def cline_cycle_members(bodies, source):
    required = {"runtime-attempt.json", *CLINE_CYCLE_AGGREGATES}
    allowed = required | {label + suffix for label in CLINE_CYCLE_PUBLIC
                          for suffix in (".command.json", ".stdout.log", ".stderr.log")}
    if (not required <= set(bodies) or set(bodies) - allowed or len(bodies) > 54
            or sum(map(len, bodies.values())) > 12 << 20
            or any(cline_cycle_private(body) for body in bodies.values())):
        raise ValueError("Cline cycle closed member/size/private boundary")
    result = strict_json(bodies["runtime-attempt.json"])
    if type(result) is not dict:
        raise ValueError("Cline cycle result object required")
    first_capture = result.get("capture_mode") == "cline-complete-cycle-first-capture"
    success = result.get("runtime_attempt_complete") is True and result.get("test_failed") is False
    failed = result.get("runtime_attempt_complete") is False and result.get("test_failed") is True
    if (type(result) is not dict or result.get("capture_mode") != "cline-complete-cycle-first-capture"
            or result.get("client") != "cline" or result.get("version") != "3.0.73"
            or result.get("runtime_attempt_complete") is not False or result.get("test_failed") is not True
            or result.get("cycle_capture_review_required") is not True or result.get("activation") != "prepared"
            or result.get("vendor_authentication_qualified") is not False or result.get("real_model_qualified") is not False
            or result.get("source_identity") != {"installer_base_commit": source["source_commit"], "installer_tree": source["source_tree"],
                "installer_source_state": "committed", "installer_patch_sha256": []}
            or type(result.get("qualification")) is not dict or set(result["qualification"]) != set(shared.QUALIFICATION_CASES)
            or any(value is not False for value in result["qualification"].values())):
        if (type(result) is not dict or result.get("capture_mode") != "cline-complete-cycle"
                or result.get("client") != "cline" or result.get("version") != "3.0.73"
                or not (success or failed) or result.get("cycle_capture_review_required") is not False
                or result.get("activation") != "prepared" or result.get("vendor_authentication_qualified") is not False
                or result.get("real_model_qualified") is not False
                or result.get("source_identity") != {"installer_base_commit": source["source_commit"], "installer_tree": source["source_tree"],
                    "installer_source_state": "committed", "installer_patch_sha256": []}
                or type(result.get("qualification")) is not dict or set(result["qualification"]) != set(shared.QUALIFICATION_CASES)
                or any(value is not success for value in result["qualification"].values())):
            raise ValueError("Cline cycle result/qualification pair differs")
    complete = result.get("cycle_observed_complete", False)
    if type(complete) is not bool:
        raise ValueError("Cline cycle completion type differs")
    if success and not complete:
        raise ValueError("Cline success requires observed complete cycle")
    public = []
    for label in CLINE_CYCLE_PUBLIC:
        triple = {label + suffix for suffix in (".command.json", ".stdout.log", ".stderr.log")}
        if set(bodies) & triple:
            if not triple <= set(bodies):
                raise ValueError("Cline cycle partial public triple")
            row = strict_json(bodies[label + ".command.json"])
            if (type(row) is not dict or set(row) != {"argv", "cwd", "exit", "error", "error_kind", "deadline"}
                    or type(row["argv"]) is not list or not row["argv"] or any(type(value) is not str for value in row["argv"])
                    or type(row["cwd"]) is not str or type(row["exit"]) is not int
                    or type(row["error"]) is not bool or type(row["deadline"]) is not bool
                    or row["error_kind"] not in ("none", "exit", "capture")):
                raise ValueError("Cline cycle closed public command receipt")
            if complete:
                absent = label == "repeat-remove-a"
                if (row["exit"] != int(absent) or row["error"] is not False or row["deadline"]
                        or row["error_kind"] != "none"
                        or (absent and (bodies[label + ".stdout.log"] or bodies[label + ".stderr.log"] !=
                            b'agentplugins: plugin is not installed for target "cline" in project scope; no target was changed\n'))):
                    raise ValueError("Cline cycle public success/exact repeat-removal differs")
            public.append(label)
    if (not cline_cycle_prefix(public, CLINE_CYCLE_PUBLIC) or result.get("cycle_public_stages") != public
            or not cline_cycle_prefix(result.get("cycle_listing_stages"), CLINE_CYCLE_LISTINGS)
            or not cline_cycle_prefix(result.get("cycle_acp_stages"), CLINE_CYCLE_ACP)):
        raise ValueError("Cline cycle summary stage matrix differs")
    rows = {name: cline_cycle_rows(bodies[name]) for name in CLINE_CYCLE_AGGREGATES}
    for name in ("cycle-native-acp.jsonl", "cycle-native-commands.jsonl", "cycle-provider-receipts.jsonl", "cycle-probe-events.jsonl"):
        cline_cycle_records(rows[name], CLINE_CYCLE_ACP)
    cline_cycle_records(rows["cycle-discovery.jsonl"], CLINE_CYCLE_LISTINGS, True)
    cline_cycle_records(rows["cycle-proof.jsonl"], CLINE_CYCLE_PROOF)
    for name in ("cycle-native-acp.jsonl", "cycle-probe-events.jsonl"):
        for row in rows[name]:
            raw = cline_cycle_stream(row["value"])
            if name == "cycle-native-acp.jsonl" and complete and not raw.strip():
                raise ValueError("Cline complete native transcript missing")
            if name == "cycle-probe-events.jsonl" and ((row["stage"] in CLINE_CYCLE_EMPTY and raw)
                    or (complete and row["stage"] not in CLINE_CYCLE_EMPTY and not raw.strip())):
                raise ValueError("Cline empty/calling probe capture differs")
    cline_cycle_structured(rows, complete)
    if complete:
        if (len(bodies) != 54 or public != list(CLINE_CYCLE_PUBLIC)
                or result["cycle_listing_stages"] != list(CLINE_CYCLE_LISTINGS)
                or result["cycle_acp_stages"] != list(CLINE_CYCLE_ACP)
                or result.get("attempt_stage") != ("complete-cycle-capture-review" if first_capture else "complete-cycle")):
            raise ValueError("complete Cline cycle matrix required")
        for name in ("cycle-native-acp.jsonl", "cycle-native-commands.jsonl", "cycle-provider-receipts.jsonl", "cycle-probe-events.jsonl"):
            if [row["stage"] for row in rows[name]] != list(CLINE_CYCLE_ACP):
                raise ValueError("complete Cline cycle ACP closure required")
        if [row["stage"] for row in rows["cycle-proof.jsonl"]] != list(CLINE_CYCLE_PROOF):
            raise ValueError("complete Cline cycle source/state proof closure required")
        cleanup = result.get("cycle_provider_cleanup")
        if cleanup != {"shutdown_complete": True, "forced_close": False, "error": False, "completed_stages": list(CLINE_CYCLE_ACP)}:
            raise ValueError("complete Cline cycle provider cleanup required")
    if success:
        cline_cycle_success.validate_success(rows, strict_json, cline_cycle_stream)
    return result


def cline_cycle_structured(rows, complete):
    identifiers = {key: set() for key in ("call_id", "prompt", "sentinel")}
    for name in ("cycle-provider-requests.jsonl", "cycle-provider-responses.jsonl"):
        cline_cycle_records(rows[name], CLINE_CYCLE_ACP, True)
        counts = {}
        for row in rows[name]:
            value, stage = row["value"], row["stage"]
            if set(value) != {"index", "stream"} or type(value["index"]) is not int or value["index"] != counts.get(stage, 0):
                raise ValueError("Cline cycle provider sequence differs")
            raw = cline_cycle_stream(value["stream"])
            counts[stage] = counts.get(stage, 0) + 1
            if counts[stage] > (1 if stage in CLINE_CYCLE_EMPTY else 2) or not raw.strip():
                raise ValueError("Cline cycle provider bound/empty stream")
        for row in rows["cycle-provider-receipts.jsonl"]:
            key = "requests" if "requests" in name else "responses"
            if row["value"].get(key) != counts.get(row["stage"], 0):
                raise ValueError("Cline cycle provider receipt/member count differs")
    for row in rows["cycle-provider-receipts.jsonl"]:
        value, empty = row["value"], row["stage"] in CLINE_CYCLE_EMPTY
        operation = "empty" if empty else ("write" if row["stage"] in ("installed-a", "installed-b") else "read")
        fields = {"requests", "responses", "empty", "exact_call_issued", "single_call_approved", "error", "call_id", "prompt", "sentinel", "operation"}
        if (set(value) != fields or value["empty"] is not empty
                or any(type(value[key]) is not bool for key in ("empty", "exact_call_issued", "single_call_approved", "error"))
                or any(type(value[key]) is not int or not 0 <= value[key] <= (1 if empty else 2) for key in ("requests", "responses"))
                or value["responses"] > value["requests"] or value["operation"] != operation
                or any(type(value[key]) is not str or not value[key] or len(value[key]) > 256 for key in ("call_id", "prompt", "sentinel", "operation"))):
            raise ValueError("Cline cycle finite provider stage receipt")
        for key in identifiers:
            if value[key] in identifiers[key]:
                raise ValueError("Cline cycle provider phase identity reused")
            identifiers[key].add(value[key])
        if complete and (value["requests"] != (1 if empty else 2) or value["responses"] != value["requests"] or value["error"]
                or value["exact_call_issued"] is not (not empty) or value["single_call_approved"] is not (not empty)):
            raise ValueError("Cline cycle provider complete stage differs")
    for row in rows["cycle-discovery.jsonl"]:
        value = row["value"]
        if (set(value) != {"kind", "argv", "cwd", "mcp_settings_override", "stdout", "stderr", "exit", "error", "deadline"}
                or value["kind"] not in ("skills", "mcp") or type(value["argv"]) is not list or len(value["argv"]) != 8
                or any(type(arg) is not str for arg in value["argv"]) or type(value["cwd"]) is not str
                or value["argv"][1:] != ["--auto-approve", "false", "--cwd", value["cwd"], "config", value["kind"], "--json"]
                or type(value["mcp_settings_override"]) is not bool or type(value["exit"]) is not int
                or type(value["error"]) is not bool or type(value["deadline"]) is not bool):
            raise ValueError("Cline cycle discovery receipt differs")
        cline_cycle_stream(value["stdout"])
        stderr = cline_cycle_stream(value["stderr"])
        if complete and (value["exit"] != 0 or value["error"] is not False or value["deadline"] is not False or stderr):
            raise ValueError("Cline cycle discovery command did not complete")
    expected = [(stage, kind) for stage in CLINE_CYCLE_LISTINGS for kind in ("skills", "mcp")]
    actual = [(row["stage"], row["value"]["kind"]) for row in rows["cycle-discovery.jsonl"]]
    if actual != expected[:len(actual)] or (complete and actual != expected):
        raise ValueError("Cline cycle exact listing pair closure")
    for row in rows["cycle-proof.jsonl"]:
        value = row["value"]
        if row["stage"].startswith("source-"):
            if set(value) != {"source", "digest", "files"} or type(value["files"]) is not dict:
                raise ValueError("Cline cycle independent source recipe receipt")
        else:
            if set(value) != {"scope", "revision", "removed", "state", "binding", "source_digest"} or value["scope"] not in ("user", "a", "b") or type(value["removed"]) is not bool:
                raise ValueError("Cline cycle installed proof receipt")
            cline_cycle_stream(value["state"])
    for row in rows["cycle-native-commands.jsonl"]:
        value = row["value"]
        if (set(value) != {"argv", "cwd", "mcp_settings_override", "auto_approve", "injected_mcp_servers", "error", "deadline", "error_stream", "started_at", "ended_at", "session", "cleanup"}
                or value["auto_approve"] is not False or value["injected_mcp_servers"] != []
                or type(value["argv"]) is not list or len(value["argv"]) != 6 or any(type(arg) is not str for arg in value["argv"])
                or type(value["cwd"]) is not str or value["argv"][1:] != ["--auto-approve", "false", "--cwd", value["cwd"], "--acp"]
                or type(value["mcp_settings_override"]) is not str or type(value["started_at"]) is not str or type(value["ended_at"]) is not str
                or type(value["error"]) is not bool or type(value["deadline"]) is not bool
                or value["cleanup"] != "existing OS strict planned duplex shutdown; entire delegated descendant cgroup"):
            raise ValueError("Cline cycle native command authority receipt")
        diagnostic = cline_cycle_stream(value["error_stream"])
        if complete and (value["error"] is not False or value["deadline"] is not False or diagnostic
                or not isinstance(value["session"], str) or re.fullmatch(r"\d{13}_[A-Za-z0-9_-]{5}_cli", value["session"]) is None):
            raise ValueError("Cline cycle native process/session/cleanup failure")


def cline_members(bodies, source):
    if "runtime-attempt.json" in bodies:
        result = strict_json(bodies["runtime-attempt.json"])
        if isinstance(result, dict) and result.get("capture_mode") in ("cline-complete-cycle-first-capture", "cline-complete-cycle"):
            return cline_cycle_members(bodies, source)
    allowed = {"runtime-attempt.json", "probe-events.jsonl", "provider.json",
               "native-acp.jsonl", "native-acp.command.json", "native-acp.error.log"}
    public_rows, acp_row, provider = {}, None, None
    for label in CLINE_COMMANDS:
        triple = {label + suffix for suffix in (".command.json", ".stdout.log", ".stderr.log")}
        allowed |= triple
        if set(bodies) & triple and not triple <= set(bodies):
            raise ValueError("incomplete Cline public command triple")
        if triple <= set(bodies):
            row = strict_json(bodies[label + ".command.json"])
            if (not isinstance(row, dict) or set(row) != {"argv", "cwd", "exit", "error", "deadline"}
                    or not isinstance(row["argv"], list) or not row["argv"]
                    or any(not isinstance(value, str) for value in row["argv"])
                    or not isinstance(row["cwd"], str) or type(row["exit"]) is not int
                    or type(row["error"]) is not bool or type(row["deadline"]) is not bool):
                raise ValueError("Cline public command receipt differs")
            public_rows[label] = row
    for number in (1, 2):
        allowed |= {f"provider-{number}.request.json", f"provider-{number}.response.log"}
    if "runtime-attempt.json" not in bodies or set(bodies) - allowed:
        raise ValueError("unrecorded Cline ACP artifact member")
    result = strict_json(bodies["runtime-attempt.json"])
    if (not isinstance(result, dict) or result.get("client") != "cline" or result.get("version") != "3.0.73"
            or not ((result.get("runtime_attempt_complete") is False and result.get("test_failed") is True)
                    or (result.get("runtime_attempt_complete") is True and result.get("test_failed") is False))
            or result.get("activation") != "prepared" or result.get("vendor_authentication_qualified") is not False
            or result.get("real_model_qualified") is not False):
        raise ValueError("Cline ACP result pair or fixed identity differs")
    qualification = result.get("qualification")
    if (not isinstance(qualification, dict) or set(qualification) != set(shared.QUALIFICATION_CASES)
            or any(value is not False for value in qualification.values())):
        raise ValueError("Cline attempt qualification must remain false")
    if result.get("source_identity") != {"installer_base_commit": source["source_commit"], "installer_tree": source["source_tree"],
                                        "installer_source_state": "committed", "installer_patch_sha256": []}:
        raise ValueError("Cline evidence source differs")
    acp = {"native-acp.jsonl", "native-acp.command.json"}
    if set(bodies) & acp and not acp <= set(bodies):
        raise ValueError("incomplete Cline ACP command capture")
    if acp <= set(bodies):
        row = strict_json(bodies["native-acp.command.json"])
        acp_row = row
        if (not isinstance(row, dict) or set(row) != {"argv", "cwd", "mcp_settings_override", "auto_approve",
                "injected_mcp_servers", "error", "elapsed_ms", "cleanup", "stderr_contract"}
                or not isinstance(row["argv"], list) or not row["argv"]
                or any(not isinstance(value, str) for value in row["argv"])
                or not isinstance(row["cwd"], str) or not isinstance(row["mcp_settings_override"], str)
                or row["auto_approve"] is not False or row["injected_mcp_servers"] != []
                or type(row["error"]) is not bool or type(row["elapsed_ms"]) is not int or row["elapsed_ms"] < 0
                or row["cleanup"] != "existing OS strict planned duplex shutdown; entire delegated descendant cgroup"
                or row["stderr_contract"] != "existing OS returns bounded 32KiB diagnostic on error; successful planned shutdown exposes no diagnostic stream"
                or ("native-acp.error.log" in bodies) != row["error"]):
            raise ValueError("Cline ACP identity, approval or cleanup receipt differs")
    elif "native-acp.error.log" in bodies:
        raise ValueError("orphaned Cline ACP diagnostic")
    requests = [number for number in (1, 2) if f"provider-{number}.request.json" in bodies]
    responses = [number for number in (1, 2) if f"provider-{number}.response.log" in bodies]
    if requests != list(range(1, len(requests) + 1)) or responses != list(range(1, len(responses) + 1)) or len(responses) > len(requests):
        raise ValueError("Cline provider sequence differs")
    if requests or responses:
        if "provider.json" not in bodies:
            raise ValueError("Cline provider receipt missing")
    if "provider.json" in bodies:
        provider = strict_json(bodies["provider.json"])
        if (not isinstance(provider, dict) or set(provider) - {"requests", "responses", "exact_call_issued", "single_call_approved", "error", "error_text", "transport"}
                or type(provider.get("requests")) is not int or provider["requests"] != len(requests)
                or type(provider.get("responses")) is not int or provider["responses"] != len(responses)
                or any(type(provider.get(key)) is not bool for key in ("exact_call_issued", "single_call_approved", "error"))
                or provider.get("transport") != "loopback scripted OpenAI chat-completions SSE; no real model"):
            raise ValueError("Cline provider receipt differs")
    if result["runtime_attempt_complete"]:
        required = {"runtime-attempt.json", "probe-events.jsonl", "provider.json", *acp}
        required |= {label + suffix for label in CLINE_COMMANDS
                     for suffix in (".command.json", ".stdout.log", ".stderr.log")}
        required |= {f"provider-{number}.{suffix}" for number in (1, 2)
                     for suffix in ("request.json", "response.log")}
        nonempty = {"native-acp.jsonl", "probe-events.jsonl", "provider-1.request.json", "provider-2.request.json",
                    "provider-1.response.log", "provider-2.response.log"}
        if (set(bodies) != required or any(not bodies.get(name, b"").strip() for name in nonempty)
                or set(public_rows) != set(CLINE_COMMANDS)
                or any(row["exit"] != 0 or row["error"] or row["deadline"] for row in public_rows.values())
                or acp_row is None or acp_row["error"] is not False
                or provider is None or provider["requests"] != 2 or provider["responses"] != 2
                or provider["exact_call_issued"] is not True or provider["single_call_approved"] is not True
                or provider["error"] is not False or "error_text" in provider
                or result.get("native_exchange_observed") is not True
                or result.get("attempt_stage") != "native-installed-acp"
                or not isinstance(result.get("native_session_id"), str)
                or re.fullmatch(r"\d{13}_[A-Za-z0-9_-]{5}_cli", result["native_session_id"]) is None):
            raise ValueError("complete Cline installed ACP closure and successful receipts required")
    if any(re.search(rb"uap-cline-private-[0-9a-f]+", body) for body in bodies.values()):
        raise ValueError("unredacted fixture provider key refused")
    return result


def kimi_call_members(bodies, source):
    if any(b"uap-kimi-key-" in body for body in bodies.values()):
        raise ValueError("unredacted Kimi fixture provider key refused")
    allowed = {"runtime-attempt.json", "probe-events.jsonl", "python-api.json",
               ".kimi-code-mcp.json.installed", "uap-kimi-native-a-SKILL.md.installed"}
    allowed |= {f"provider-request-{number}.json" for number in range(1, 9)}
    for label in KIMI_CALL_COMMANDS:
        triple = {label + suffix for suffix in (".command.json", ".stdout.log", ".stderr.log")}
        allowed |= triple
        if set(bodies) & triple and not triple <= set(bodies):
            raise ValueError("incomplete Kimi single-call command evidence")
        if triple <= set(bodies):
            row = strict_json(bodies[label + ".command.json"])
            if (not isinstance(row, dict) or set(row) != {"argv", "cwd", "exit", "error", "deadline", "cleanup"}
                    or not isinstance(row["argv"], list) or not row["argv"]
                    or any(not isinstance(value, str) for value in row["argv"])
                    or not isinstance(row["cwd"], str) or type(row["exit"]) is not int
                    or type(row["error"]) is not bool or type(row["deadline"]) is not bool
                    or row["cleanup"] != "ordinary full-descendant supervision; forced termination fails"):
                raise ValueError("Kimi single-call command receipt differs")
    if "runtime-attempt.json" not in bodies or set(bodies) - allowed:
        raise ValueError("unrecorded Kimi single-call evidence member")
    result = strict_json(bodies["runtime-attempt.json"])
    if (not isinstance(result, dict) or result.get("client") != "kimi" or result.get("version") != "2.3.2"
            or result.get("phase") != "installed-single-tool-attempt" or result.get("activation") != "prepared"
            or result.get("provider_kind") != "scripted-openai-chat-completions"
            or result.get("runtime_attempt_complete") is not False or result.get("test_failed") is not True
            or result.get("vendor_authentication_qualified") is not False or result.get("real_model_qualified") is not False
            or type(result.get("elapsed_ms")) is not int or result["elapsed_ms"] < 0):
        raise ValueError("initial Kimi single-call capture must remain explicitly unqualified")
    qualification = result.get("qualification")
    if (not isinstance(qualification, dict) or set(qualification) != set(shared.QUALIFICATION_CASES)
            or any(value is not False for value in qualification.values())):
        raise ValueError("Kimi single-call capture cannot inherit lifecycle qualifications")
    if result.get("source_identity") != {"installer_base_commit": source["source_commit"], "installer_tree": source["source_tree"],
                                        "installer_source_state": "committed", "installer_patch_sha256": []}:
        raise ValueError("Kimi single-call source identity differs")
    numbers = sorted(int(name.removeprefix("provider-request-").removesuffix(".json"))
                     for name in bodies if name.startswith("provider-request-"))
    if numbers != list(range(1, len(numbers) + 1)):
        raise ValueError("Kimi provider request capture sequence has gaps")
    return result


def admit_runtime(path, client, source):
    body = ordinary_bytes(path, MAX_ZIP)
    bodies, total = {}, 0
    with zipfile.ZipFile(io.BytesIO(body)) as archive:
        entries = archive.infolist()
        if not 1 <= len(entries) <= 80:
            raise ValueError("native ZIP member count refused")
        for member in entries:
            mode = member.external_attr >> 16
            name = member.filename
            if (name in bodies or Path(name).name != name or not re.fullmatch(r"[A-Za-z0-9_.-]{1,128}", name)
                    or member.is_dir() or stat.S_IFMT(mode) not in (0, stat.S_IFREG) or member.flag_bits & 1
                    or member.compress_type not in (zipfile.ZIP_STORED, zipfile.ZIP_DEFLATED)
                    or member.file_size > MAX_MEMBER or total + member.file_size > (24 if client == "kimi" else 16) << 20):
                raise ValueError("native ZIP member type/name/size refused")
            with archive.open(member) as stream:
                value = shared.bounded_read(stream, MAX_MEMBER)
            if len(value) != member.file_size:
                raise ValueError("native ZIP size differs")
            value.decode("utf-8", errors="strict")
            bodies[name] = value
            total += len(value)
    if client == "kimi-call":
        result = kimi_call_members(bodies, source)
    elif client == "cline":
        result = cline_members(bodies, source)
    elif client == "kimi":
        result = kimi_members(bodies, source)
    elif client in CLI_ATTEMPTS:
        result = cli_attempt_members(bodies, source, client)
    else:
        result = attempt_members(bodies, source, client)
    return bodies, result


def selected_cases():
    rows = []
    for selector, case in CASES.items():
        client = "kimi" if selector == "kimi-call" else selector
        leaf, test = SOURCE / "scripts/project_native_collectors" / (client + ".py"), SOURCE / case["file"]
        if not leaf.exists() or not test.exists():
            continue
        if selector == "kimi-call" and not (SOURCE / "scripts/project_native_collectors/kimi_attempt.py").exists():
            continue
        if not re.search(r"(?m)^func " + re.escape(case["test"]) + r"\(t \*testing.T\) \{", test.read_text()):
            raise ValueError("declared exact native test is absent")
        rows.append({"client": selector, "phase": case["phase"]})
    return {"include": rows}


def capture(ctx, label, argv, cwd, env, seconds):
    record, stdout, stderr = ctx.command(label, argv, cwd, env, seconds)
    if record["exit"] != 0:
        raise ValueError(label + " failed; bounded streams retained")
    return stdout, stderr


def freeze(path, identities, limit=256 << 20):
    body = ordinary_bytes(path, limit)
    if not body:
        raise ValueError("empty executable/input refused")
    digest = hashlib.sha256(body).hexdigest()
    identities[path] = digest
    return {"path": str(path), "bytes": len(body), "sha256": digest}


def prepare(selector, ctx):
    # This method performs only Git source queries before the checked compiler
    # or any acquired client can be executed.
    ctx.source_and_projects()
    case = CASES[selector]
    client = "kimi" if selector == "kimi-call" else selector
    leaf = shared.adjacent("project_native_runtime_leaf_" + client, "project_native_collectors/" + client + ".py")
    if leaf.CLIENT != client or getattr(leaf, "ACQUISITION_ONLY", False):
        raise ValueError("unreviewed native client")
    receipt, contents = shared.acquire(leaf.ACQUISITION, ctx.budget)
    binary, client_ids = shared.materialize(ctx.root / "client", leaf.ACQUISITION, receipt, contents)
    identity = {"client": client, "acquisition": receipt}
    node = None
    if leaf.ACQUISITION["runtime"] == "node":
        node_receipt, node_contents = shared.acquire(shared.NODE, ctx.budget, node=True)
        node, node_ids = shared.materialize(ctx.root / "node", shared.NODE, node_receipt, node_contents, selected_only=True)
        client_ids.update(node_ids)
        identity["node"] = shared.selected_node_receipt(node_receipt)
    elif leaf.ACQUISITION["runtime"] != "native":
        raise ValueError("unreviewed client runtime")
    identity["runtime_files"] = [{"path": str(path), "sha256": digest} for path, digest in client_ids.items()]
    ctx.identities.update(client_ids)
    del contents
    kind, repository, version, archive, pin, basename = shared.native.PINS["linux-amd64"]["lintai"]
    if kind != "github" or not archive.endswith(".tar.gz"):
        raise ValueError("pinned scanner acquisition differs")
    spec = {"url": "https://github.com/" + repository + "/releases/download/" + version + "/" + archive,
            "integrity": pin, "format": "tar.gz", "max_bytes": 96 << 20, "max_expanded_bytes": 256 << 20,
            "entrypoint": basename, "runtime": "native"}
    scanner_receipt, scanner_contents = shared.acquire(spec, ctx.budget)
    candidates = [row["name"] for row in scanner_receipt["members"] if row["kind"] == "file" and Path(row["name"]).name == basename]
    if len(candidates) != 1:
        raise ValueError("exact scanner basename is absent or ambiguous")
    spec["entrypoint"] = candidates[0]
    scanner, scanner_ids = shared.materialize(ctx.root / "scanner", spec, scanner_receipt, scanner_contents, selected_only=True)
    ctx.identities.update(scanner_ids)
    del scanner_contents
    selected = shutil.which("go")
    if not selected:
        raise ValueError("pinned Go compiler not supplied")
    go = Path(selected).resolve(strict=True)
    # setup-go owns the compiler installation, which may be root-owned. Copy
    # its hash using its existing ordinary file identity; do not run a wrapper.
    before = go.lstat()
    if not stat.S_ISREG(before.st_mode) or before.st_nlink != 1 or before.st_size > 64 << 20:
        raise ValueError("ordinary Go compiler required")
    go_digest = hashlib.sha256(ordinary_bytes(go, 64 << 20, owners=(0, os.getuid()))).hexdigest()
    def verify_compiler():
        if (shared.artifacts.identity(go.lstat()) != shared.artifacts.identity(before)
                or hashlib.sha256(ordinary_bytes(go, 64 << 20, owners=(0, os.getuid()))).hexdigest() != go_digest):
            raise ValueError("Go compiler changed during build")
    env = dict(ctx.env)
    env.update({"PATH": str(go.parent) + ":/usr/bin:/bin", "GOTOOLCHAIN": "local", "GOMAXPROCS": "2",
                "GOCACHE": str(ctx.root / "go-cache"), "GOMODCACHE": str(ctx.root / "go-modules"),
                "GOPROXY": "https://proxy.golang.org", "GOSUMDB": "sum.golang.org"})
    verify_compiler()
    stdout, stderr = capture(ctx, "build-go-version", [go, "version"], SOURCE, env, 10)
    verify_compiler()
    if stdout != b"go version go1.26.10 linux/amd64\n" or stderr:
        raise ValueError("exact Go 1.26.10 Linux amd64 compiler required")
    tools = ctx.root / "bin"
    tools.mkdir(mode=0o700)
    installer, probe, tests = (tools / name for name in ("agentplugins", "native-probe", "project-native.test"))
    builds = (("build-installer", [go, "build", "-p=1", "-trimpath", "-o", installer, "./cmd/agentplugins"], SOURCE / "cli"),
              ("build-probe", [go, "build", "-p=1", "-trimpath", "-o", probe, "./repotests/testdata/agentplugins_native_probe"], SOURCE),
              ("build-tests", [go, "test", "-p=1", "-c", "-trimpath", "-o", tests, "./repotests"], SOURCE))
    for label, argv, cwd in builds:
        verify_compiler()
        capture(ctx, label, argv, cwd, env, 240)
        verify_compiler()
    ctx.summary["compiler"] = {"version": "1.26.10", "sha256": go_digest}
    ctx.summary["builds"] = {path.name: freeze(path, ctx.identities) for path in (installer, probe, tests)}
    identity_path = ctx.root / "runtime-identity.json"
    write_new(identity_path, json.dumps(identity, sort_keys=True).encode() + b"\n")
    freeze(identity_path, ctx.identities, 1 << 20)
    ctx.summary["client_identity"] = identity
    ctx.summary["scanner_archive_sha256"] = scanner_receipt["sha256"]
    ctx.summary["build_complete"] = True
    native_output = ctx.root / "native-evidence"
    native_output.mkdir(mode=0o700)
    selected_env = dict(ctx.env)
    selected_env.update(shared.native.disposable_runtime_environment("linux-amd64"))
    selected_env.update({"AGENTPLUGINS_INSTALLER_BIN": str(installer),
                         "AGENTPLUGINS_INSTALLER_COMMIT": ctx.summary["source_commit"],
                         "AGENTPLUGINS_INSTALLER_TREE": ctx.summary["source_tree"],
                         "AGENTPLUGINS_NATIVE_PROBE_BIN": str(probe),
                         "AGENTPLUGINS_LINTAI_BIN": str(scanner),
                         "AGENTPLUGINS_LINTAI_SHA256": scanner_ids[scanner],
                         "AGENTPLUGINS_LINTAI_ARCHIVE_SHA256": scanner_receipt["sha256"],
                         "AGENTPLUGINS_" + client.upper() + "_BIN": str(binary),
                         "AGENTPLUGINS_PROJECT_NATIVE_IDENTITY": str(identity_path),
                         "AGENTPLUGINS_PROJECT_NATIVE_OUTPUT": str(native_output), case["optin"]: "1"})
    selected_env["PLUGIN_KIT_AI_REPO_ROOT"] = str(SOURCE)
    if client == "kimi":
        python = Path("/usr/bin/python3").resolve(strict=True)
        python_body = ordinary_bytes(python, 64 << 20, owners=(0, os.getuid()))
        ctx.summary["python"] = {"path": str(python), "sha256": hashlib.sha256(python_body).hexdigest()}
        selected_env["AGENTPLUGINS_PROJECT_NATIVE_PYTHON"] = str(python)
    if node is not None:
        selected_env["AGENTPLUGINS_NODE_BIN"] = str(node)
    return tests, selected_env, native_output


def cline_host():
    shared.native.require_hosted("linux-amd64")
    if os.getuid() <= 0 or os.getgid() <= 0 or os.geteuid() != os.getuid() or os.getegid() != os.getgid():
        raise ValueError("Cline requires the original ordinary hosted user")
    for path in (Path("/run/systemd/system"), Path("/sys/fs/cgroup"), Path("/sys/fs/cgroup/system.slice")):
        info = path.lstat()
        if path.resolve() != path or not stat.S_ISDIR(info.st_mode) or info.st_uid != 0:
            raise ValueError("actual canonical system manager and cgroup hierarchy required")


def cline_service_argv(tests, env, output, unit, uid, gid, origin_ns):
    if (not CLINE_UNIT.fullmatch(unit) or type(uid) is not int or uid <= 0 or type(gid) is not int or gid <= 0
            or type(origin_ns) is not int or origin_ns <= 0 or set(env) != CLINE_ENV
            or any(not isinstance(value, str) or len(value) > 4096 or any(c in value for c in "\x00\r\n") for value in env.values())
            or env["PATH"] != "/usr/bin:/bin" or env["AGENTPLUGINS_CLINE_PROJECT_RUNTIME_ATTEMPT"] != "1"
            or not tests.is_absolute() or tests.resolve() != tests
            or not output.is_absolute() or output.resolve() != output or SOURCE == output or SOURCE in output.parents):
        raise ValueError("closed Cline delegated service arguments required")
    selected = {**env, "AGENTPLUGINS_PROJECT_NATIVE_STARTED_NS": str(origin_ns),
                "AGENTPLUGINS_CLINE_SYSTEMD_UNIT": unit, "AGENTPLUGINS_CLINE_ORIGINAL_UID": str(uid),
                "AGENTPLUGINS_CLINE_ORIGINAL_GID": str(gid)}
    return ["/usr/bin/sudo", "-n", "/usr/bin/systemd-run", "--unit=" + unit, "--wait", "--pipe", "--collect", "--quiet",
            "--service-type=exec", "--expand-environment=no", "--uid=" + str(uid), "--gid=" + str(gid), "--property=Delegate=yes",
            "--property=NoNewPrivileges=yes", "--property=KillMode=control-group", "--property=RuntimeMaxSec=180s",
            "--property=TimeoutStopSec=30s", "--working-directory=" + str(output), "/usr/bin/env", "-i",
            *(key + "=" + selected[key] for key in sorted(selected)), str(tests),
            "-test.run=^TestAgentpluginsClineNativeProjectCompleteCycle$", "-test.count=1", "-test.v", "-test.timeout=180s"]


def cline_unit_state(body, unit):
    if not CLINE_UNIT.fullmatch(unit):
        raise ValueError("unregistered Cline unit")
    rows = body.decode("ascii", "strict").splitlines()
    value = {}
    for row in rows:
        key, separator, item = row.partition("=")
        if not separator or key not in ("LoadState", "ActiveState", "ControlGroup") or key in value:
            raise ValueError("unexpected exact-unit manager receipt")
        value[key] = item
    if (set(value) != {"LoadState", "ActiveState", "ControlGroup"}
            or value["LoadState"] not in ("loaded", "not-found")
            or value["ActiveState"] not in ("active", "inactive", "failed", "activating", "deactivating")
            or value["ControlGroup"] not in ("", "/system.slice/" + unit)
            or (value["LoadState"] == "not-found" and (value["ActiveState"] != "inactive" or value["ControlGroup"]))):
        raise ValueError("Cline unit state or control group differs")
    return value


def cline_cgroup_empty(unit, uid):
    if not CLINE_UNIT.fullmatch(unit):
        raise ValueError("unregistered Cline cgroup")
    parent = Path("/sys/fs/cgroup/system.slice")
    if parent.resolve() != parent:
        raise ValueError("cgroup parent alias refused")
    parent_fd = os.open(parent, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        try:
            directory = os.open(unit, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=parent_fd)
        except FileNotFoundError:
            return True
        try:
            if os.fstat(directory).st_uid != uid:
                raise ValueError("owned delegated Cline cgroup required")
            descriptor = os.open("cgroup.events", os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=directory)
            try:
                if not stat.S_ISREG(os.fstat(descriptor).st_mode):
                    raise ValueError("ordinary cgroup event control required")
                body = os.read(descriptor, 4097)
            finally:
                os.close(descriptor)
        except FileNotFoundError:
            # A concurrently collected unit is accepted only when its exact
            # original directory has disappeared from this held parent.
            try:
                os.stat(unit, dir_fd=parent_fd, follow_symlinks=False)
            except FileNotFoundError:
                return True
            raise
        finally:
            os.close(directory)
    finally:
        os.close(parent_fd)
    if len(body) > 4096 or not re.fullmatch(rb"populated [01]\nfrozen [01]\n", body):
        raise ValueError("unexpected cgroup event state")
    return body.startswith(b"populated 0\n")


def cline_service(ctx, tests, env, output):
    """Delegate only this finite test service; native code remains non-root.

    The existing Go OS supervisor still discovers and admits its actual cgroup.
    There is no supplied delegation-root override or replacement child launcher.
    """
    cline_host()
    uid, gid = os.getuid(), os.getgid()
    held = {}
    for name in ("sudo", "systemd-run", "systemctl", "env"):
        path = Path("/usr/bin") / name
        info = path.lstat()
        body = ordinary_bytes(path, 32 << 20, owners=(0,))
        if not info.st_mode & 0o111 or info.st_mode & 0o022:
            raise ValueError("root-owned immutable system executable required")
        held[path] = (shared.artifacts.identity(info), hashlib.sha256(body).hexdigest())
    def unchanged():
        for path, (identity, digest) in held.items():
            if (shared.artifacts.identity(path.lstat()) != identity
                    or hashlib.sha256(ordinary_bytes(path, 32 << 20, owners=(0,))).hexdigest() != digest):
                raise ValueError("Cline service system executable changed")
    system_env = {"PATH": "/usr/bin:/bin", "LANG": "C", "SYSTEMD_PAGER": "cat", "SYSTEMD_COLORS": "0"}
    unit = "uap-cline-project-" + secrets.token_hex(16) + ".service"
    def query(label):
        unchanged()
        record, stdout, stderr = ctx.command(label, ["/usr/bin/systemctl", "--no-pager", "--no-ask-password", "show",
                    "--property=LoadState,ActiveState,ControlGroup", unit], output, system_env, 2)
        unchanged()
        if record["exit"] != 0 or stderr:
            raise ValueError("exact owned unit query failed")
        return cline_unit_state(stdout, unit)
    state = query("cline-service-absent")
    if state["LoadState"] != "not-found" or not cline_cgroup_empty(unit, uid):
        raise ValueError("fresh Cline unit already exists")
    record, stdout, stderr = ctx.command("cline-systemd-version", ["/usr/bin/systemd-run", "--version"], output, system_env, 5)
    unchanged()
    if record["exit"] != 0 or stderr or not re.match(rb"systemd 255(?: \([^\r\n]*\))?\n", stdout):
        raise ValueError("reviewed systemd 255 host required")
    receipt = {"unit": unit, "uid": uid, "gid": gid, "forced_stop": False, "forced_kill": False,
               "closed": False, "system_executables": {str(path): digest for path, (_, digest) in held.items()}}
    ctx.summary["delegated_service"] = receipt
    origin = time.monotonic_ns()
    argv = cline_service_argv(tests, env, output, unit, uid, gid, origin)
    receipt["original_start_monotonic_ns"] = origin
    result, problem = None, None
    with shared.Budget(180 - (time.monotonic_ns() - origin) / 1e9) as budget:
        ctx.budget = budget
        try:
            # Five seconds for the capture group to reap and ten for exact
            # service cleanup remain within this original 180-second origin.
            result = ctx.command("runtime-test", argv, output, system_env, 165 - (time.monotonic_ns() - origin) / 1e9)
        except Exception as error:
            problem = error
        finally:
            try:
                state = query("cline-service-state")
                closed = state["ActiveState"] in ("inactive", "failed") and cline_cgroup_empty(unit, uid)
            except Exception as error:
                closed, problem = False, problem or error
            if not closed:
                receipt["forced_stop"] = True
                try:
                    unchanged()
                    ctx.command("cline-service-stop", ["/usr/bin/sudo", "-n", "/usr/bin/systemctl", "--no-ask-password",
                                "--no-block", "stop", unit], output, system_env, 1)
                    pause_end = min(origin / 1e9 + 172, time.monotonic() + 0.2)
                    while time.monotonic() < pause_end and not cline_cgroup_empty(unit, uid):
                        time.sleep(0.02)
                    closed = cline_cgroup_empty(unit, uid)
                except Exception as error:
                    problem = problem or error
                if not closed:
                    receipt["forced_kill"] = True
                    try:
                        unchanged()
                        ctx.command("cline-service-kill", ["/usr/bin/sudo", "-n", "/usr/bin/systemctl", "--no-ask-password",
                                    "kill", "--kill-whom=all", "--signal=SIGKILL", unit], output, system_env, 1)
                    except Exception as error:
                        problem = problem or error
                try:
                    state = query("cline-service-final-state")
                    closed = state["ActiveState"] in ("inactive", "failed") and cline_cgroup_empty(unit, uid)
                except Exception as error:
                    problem = problem or error
            receipt["closed"] = closed
            unchanged()
            shared.verify_files(ctx.identities, budget)
            if not closed or receipt["forced_stop"] or receipt["forced_kill"]:
                problem = ValueError("Cline service required forced cleanup or retained descendants")
            if time.monotonic_ns() - origin >= 180_000_000_000:
                problem = ValueError("Cline original service deadline exhausted")
    if problem is not None:
        raise problem
    return result


def run(client, output):
    shared.native.require_hosted("linux-amd64")
    if client == "cline":
        cline_host()
    if client not in CASES or not any(row["client"] == client for row in selected_cases()["include"]):
        raise ValueError("no reviewed exact native test selected")
    if not output.is_absolute() or output.resolve() != output or output.exists() or SOURCE == output or SOURCE in output.parents:
        raise ValueError("fresh canonical output outside checkout required")
    output.mkdir(mode=0o700, parents=True)
    case = CASES[client]
    summary = {"client": "kimi" if client == "kimi-call" else client, "phase": case["phase"], "commands": [], "cases": [],
               "qualification": {name: False for name in shared.QUALIFICATION_CASES},
               "build_complete": False, "test_selected": False, "test_passed": False,
               "runtime_artifact_admitted": False, "collection_complete": False}
    ctx, native_output, admitted = None, None, None
    try:
        with shared.Budget(480) as budget:
            ctx = shared.Context(output, summary, budget)
            tests, env, native_output = prepare(client, ctx)
            stdout, stderr = capture(ctx, "build-source-clean", ["/usr/bin/git", "status", "--porcelain", "--untracked-files=normal"], SOURCE, ctx.env, 10)
            if stdout or stderr:
                raise ValueError("build changed the source checkout")
            shared.verify_files(ctx.identities, budget)
        # This deadline is created once before the selected test; no stage,
        # retry or acquisition can reset the client's own original deadline.
        def accept_test(record, stdout):
            summary["test_selected"] = ("=== RUN   " + case["test"] + "\n").encode() in stdout
            summary["test_passed"] = bool(summary["test_selected"] and record["exit"] == 0
                                          and ("--- PASS: " + case["test"] + " ").encode() in stdout
                                          and ("--- SKIP: " + case["test"]).encode() not in stdout)
            if not summary["test_passed"]:
                raise ValueError("selected native test failed or did not execute; inspect bounded evidence")
        if client == "cline":
            with shared.Budget(15) as budget:
                ctx.budget = budget
                record, stdout, _ = cline_service(ctx, tests, env, output)
            accept_test(record, stdout)
        else:
            with shared.Budget(case["seconds"] + 15) as budget:
                ctx.budget = budget
                record, stdout, _ = ctx.command("runtime-test", [tests, "-test.run=^" + case["test"] + "$", "-test.count=1", "-test.v", "-test.timeout=" + str(case["seconds"]) + "s"], output, env, case["seconds"] + 5)
                shared.verify_files(ctx.identities, budget)
                accept_test(record, stdout)
    except Exception as error:
        summary["error"] = type(error).__name__ + ": " + str(error)[:512]
    finally:
        with shared.Budget(60):
            if native_output is not None:
                try:
                    admitted, result = admit_runtime(native_output / "research.zip", client, summary)
                    summary["runtime_artifact_admitted"] = True
                    summary["runtime_test_failed"] = result["test_failed"]
                    if result["test_failed"]:
                        summary["test_passed"] = False
                    if summary["test_passed"]:
                        summary["qualification"] = result["qualification"]
                except Exception as error:
                    summary["runtime_admission_error"] = type(error).__name__
                    summary["test_passed"] = False
            summary["collection_complete"] = bool(summary["test_passed"] and summary["runtime_artifact_admitted"])
            shared.publish(summary, ctx, output)
            (output / "upload/research.zip").rename(output / "upload/frontend.zip")
            if admitted is not None:
                shared.artifacts.write_zip(output / "upload/runtime.zip", admitted)
        print(json.dumps({key: summary.get(key) for key in ("client", "phase", "source_commit", "build_complete", "test_selected", "test_passed", "runtime_artifact_admitted")}))
    return 0 if summary["collection_complete"] else 1


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--matrix", action="store_true")
    parser.add_argument("--client", choices=CASES)
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    if args.matrix:
        print(json.dumps(selected_cases(), separators=(",", ":")))
        return 0
    if args.client is None or args.output is None:
        parser.error("--client and --output required")
    return run(args.client, args.output)


if __name__ == "__main__":
    sys.exit(main())
