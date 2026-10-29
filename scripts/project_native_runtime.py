"""Run a closed current-source native test on an actual disposable hosted runner.

Acquisition and bounded process capture reuse the existing research runner. A
valid artifact never changes a failed test into a passed qualification.
"""
import argparse
import base64
from datetime import datetime, timezone
from urllib.parse import urlencode
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
    "copilot": {"test": "TestAgentpluginsCopilotNativeProjectCompleteCycle",
                "file": "repotests/agentplugins_copilot_cycle_lifecycle_test.go",
                "optin": "AGENTPLUGINS_COPILOT_PROJECT_RUNTIME_ATTEMPT", "seconds": 180,
                "phase": "runtime-attempt"},
    "gemini": {"test": "TestAgentpluginsGeminiNativeProjectCompleteCycle",
               "file": "repotests/agentplugins_gemini_cycle_lifecycle_test.go",
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
    "qoder": {"test": "TestAgentpluginsQoderNativeProjectCompleteSkillsCycle",
              "file": "repotests/agentplugins_qoder_cycle_lifecycle_test.go",
              "optin": "AGENTPLUGINS_QODER_PROJECT_RUNTIME_ATTEMPT", "seconds": 180, "phase": "runtime-attempt"},
    "grok": {"test": "TestAgentpluginsGrokNativeProjectCompleteCycle",
             "file": "repotests/agentplugins_grok_cycle_lifecycle_test.go",
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
    envelope = strict_json(bodies.get("runtime-attempt.json", b"{}"))
    if type(envelope) is dict and "capture_mode" in envelope:
        if client != "gemini":
            raise ValueError("unregistered native attempt capture mode")
        from project_native_gemini_cycle import MODE, members
        if envelope["capture_mode"] == MODE:
            return members(bodies, source, envelope)
        from project_native_gemini_complete import MODE as complete_mode, members as complete_members
        if envelope["capture_mode"] == complete_mode:
            return complete_members(bodies, source, envelope)
        raise ValueError("unregistered native attempt capture mode")
    if client == "copilot" and "runtime-attempt.json" in bodies:
        result = strict_json(bodies["runtime-attempt.json"])
        if isinstance(result, dict) and result.get("mode") == "copilot-project-complete-cycle":
            from project_native_copilot_cycle import members
            return members(bodies, source)
        if isinstance(result, dict) and result.get("mode") == "copilot-installed-tool-readiness":
            from project_native_copilot_readiness import members
            return members(bodies, source)
        if isinstance(result, dict) and result.get("mode") == "copilot-user-registry-first-capture":
            from project_native_copilot_user import members
            return members(bodies, source)
        if isinstance(result, dict) and result.get("mode") == "copilot-empty-registry-first-capture":
            from project_native_copilot_empty_registry import members
            return members(bodies, source)
        if isinstance(result, dict) and result.get("mode") == "copilot-post-add-registry-first-capture":
            from project_native_copilot_post_add_registry import members
            return members(bodies, source)
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


def qoder_log_run_stamp(text):
    if (type(text) is not str or text.endswith(("+00:00", "-00:00"))
            or re.fullmatch(r"[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}\.[0-9]{3}(?:Z|[+-][0-9]{2}:[0-9]{2})", text) is None
            or int(text[17:19]) > 59 or len(text) == 29 and (int(text[24:26]) > 23 or int(text[27:29]) > 59)):
        raise ValueError("Qoder run timestamp shape refused")
    time.strptime(text, "%Y-%m-%dT%H:%M:%S.%f%z")


def qoder_log_run_observation(result, client):
    field = "qoder_log_run_observation"
    if field not in result:
        return
    if client != "qoder":
        raise ValueError("Qoder log projection cannot apply to another client")
    observation = result[field]
    if type(observation) is not dict or type(observation.get("status")) is not str:
        raise ValueError("finite Qoder log projection required")
    statuses = {"context-refused", "profile-refused", "layout-refused", "manifest-refused", "observation-bound"}
    if observation["status"] in statuses:
        if set(observation) != {"status"}:
            raise ValueError("Qoder log refusal cannot retain private details")
        return
    if observation["status"] != "captured" or set(observation) != {"status", "run"}:
        raise ValueError("unregistered Qoder log projection status")
    run = observation["run"]
    flags = {"run_id_matches_manifest", "cwd_matches", "argv_matches", "exact_added_run_closure", "preexisting_records_stable"}
    fields = flags | {"run_id", "latest_target", "manifest_bytes", "manifest_sha256", "pid", "ppid", "started_at",
                      "cli_version", "platform", "arch", "process_role", "pid_provenance"}
    if (type(run) is not dict or set(run) != fields or any(type(run[key]) is not bool for key in flags)
            or run["cli_version"] != "1.1.78" or run["platform"] != "linux" or run["arch"] != "x64"
            or run["process_role"] != "main" or run["pid_provenance"] != "manifest-only"
            or type(run["manifest_bytes"]) is not int or not 1 <= run["manifest_bytes"] <= 4096
            or type(run["manifest_sha256"]) is not str or re.fullmatch(r"[0-9a-f]{64}", run["manifest_sha256"]) is None
            or any(type(run[key]) is not int or not 0 < run[key] < 2 ** 31 for key in ("pid", "ppid"))
            or type(run["run_id"]) is not str or type(run["latest_target"]) is not str
            or run["latest_target"] != "runs/" + run["run_id"]):
        raise ValueError("bounded Qoder log identity projection differs")
    match = re.fullmatch(r"([0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}-[0-9]{2}-[0-9]{2}-[0-9]{3}(?:Z|[+-][0-9]{2}-[0-9]{2}))-([0-9a-z]{0,6})-p([1-9][0-9]{0,9})", run["run_id"])
    if match is None or int(match[3]) >= 2 ** 31:
        raise ValueError("source-format Qoder run identity required")
    stamp = match[1]
    stamp = stamp[:13] + ":" + stamp[14:16] + ":" + stamp[17:19] + "." + stamp[20:]
    if len(stamp) == 29:
        stamp = stamp[:26] + ":" + stamp[27:]
    qoder_log_run_stamp(stamp)
    qoder_log_run_stamp(run["started_at"])
    if run["run_id_matches_manifest"] and (stamp != run["started_at"] or int(match[3]) != run["pid"]):
        raise ValueError("Qoder manifest-only run identity join contradicts its projection")
    if len(json.dumps(observation, separators=(",", ":"), ensure_ascii=False).encode("utf-8")) > 2048:
        raise ValueError("Qoder log projection bound exceeded")
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
    summary = strict_json(bodies.get("runtime-attempt.json", b"{}"))
    if isinstance(summary, dict) and "capture_mode" in summary:
        if (client, summary["capture_mode"]) == ("grok", "grok-complete-skills-cycle"):
            from project_native_grok_cycle import members
        elif (client, summary["capture_mode"]) == ("qoder", "qoder-complete-skills-cycle"):
            from project_native_qoder_cycle import members
        else:
            raise ValueError("unknown fixed native CLI capture mode")
        return members(bodies, source)
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
    qoder_log_run_observation(result, client)
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


def cline_cycle_private(body, marker=b"uap-cline-private-"):
    if len(body) > 1 << 20 or marker in body:
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
            if marker in raw:
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



def kimi_call_json(body):
    if type(body) is not bytes or not 0 < len(body) <= 1 << 20:
        raise ValueError("Kimi complete raw JSON byte bound differs")
    value = strict_json(body)
    nodes = 0
    def visit(item, depth):
        nonlocal nodes
        nodes += 1
        if depth > 32 or nodes > 16384:
            raise ValueError("Kimi complete raw JSON structure bound differs")
        if type(item) is dict:
            if any(len(key.encode("utf-8")) > 4096 for key in item):
                raise ValueError("Kimi complete raw JSON key bound differs")
            for child in item.values(): visit(child, depth + 1)
        elif type(item) is list:
            for child in item: visit(child, depth + 1)
        elif type(item) in (int, float):
            try:
                finite = math.isfinite(float(item))
            except OverflowError:
                finite = False
            if not finite:
                raise ValueError("Kimi complete raw JSON finite float64 number differs")
    visit(value, 0)
    return value


def kimi_call_object(value, required, optional=()):
    if type(value) is not dict or not set(required) <= set(value) or set(value) - set(required) - set(optional):
        raise ValueError("Kimi complete raw object differs")
    return value


def kimi_call_instant(value, started=None, ended=None):
    if (type(value) is not str or len(value) > 64
            or not re.fullmatch(r"\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d{1,9})?(?:Z|[+-]\d\d:\d\d)", value)):
        raise ValueError("Kimi complete raw timestamp differs")
    stamp = datetime.fromisoformat(value.replace("Z", "+00:00"))
    if (started is not None and stamp < started) or (ended is not None and stamp > ended):
        raise ValueError("Kimi complete raw timestamp outside original origin")
    return stamp


def kimi_call_messages(page, expected):
    kimi_call_object(page, ("items", "has_more"))
    rows = page["items"]
    if page["has_more"] is not False or type(rows) is not list or len(rows) not in (4, 5):
        raise ValueError("Kimi complete finite native messages absent")
    seen, previous = set(), expected["started"]
    for row in rows:
        kimi_call_object(row, ("id", "session_id", "role", "content", "created_at"), ("metadata",))
        if (not kimi_call_text(row["id"], 256) or not row["id"] or row["id"] in seen
                or row["session_id"] != expected["session"]):
            raise ValueError("Kimi complete native message identity differs")
        seen.add(row["id"])
        previous = kimi_call_instant(row["created_at"], previous, expected["ended"])
    if (rows[0]["id"] != expected["prompt_id"] or rows[0]["role"] != "user"
            or rows[0]["content"] != [{"type": "text", "text": expected["prompt"]}]
            or rows[0].get("metadata") != {"origin": {"kind": "user"}}):
        raise ValueError("Kimi complete native prompt differs")
    index = 1
    if len(rows) == 5:
        row = rows[1]
        metadata = kimi_call_object(row.get("metadata"), ("origin",))
        origin = kimi_call_object(metadata["origin"], ("kind", "variant", "disclosure"))
        disclosure = kimi_call_object(origin["disclosure"], ("kind", "renderGeneration", "localDate", "timeZone"))
        date = kimi_call_instant(row["created_at"]).astimezone(timezone.utc).strftime("%Y-%m-%d")
        text = ("<system-reminder>\nToday's date is " + date
                + ". The current date is restated in a reminder whenever it changes; rely on the latest such reminder for the current date. DO NOT mention this to the user explicitly.\n</system-reminder>")
        if (row["role"] != "user" or origin["kind"] != "injection" or origin["variant"] != "date_change"
                or disclosure["kind"] != "date" or type(disclosure["renderGeneration"]) is not int
                or disclosure["renderGeneration"] != 1 or disclosure["timeZone"] != "UTC"
                or disclosure["localDate"] != date or row["content"] != [{"type": "text", "text": text}]):
            raise ValueError("Kimi complete native date injection differs")
        index += 1
    args = {"nonce": expected["facts"]["nonce"], "operation": "write", "marker": expected["facts"]["marker"]}
    call, result, final = rows[index:]
    if (call["role"] != "assistant" or "metadata" in call or call["content"] != [{"type": "tool_use",
            "tool_call_id": expected["call"], "tool_name": "mcp__uap-kimi-native-a__inspect_runtime", "input": args}]):
        raise ValueError("Kimi complete native owned call differs")
    if result["role"] != "tool" or "metadata" in result or type(result["content"]) is not list or len(result["content"]) != 1:
        raise ValueError("Kimi complete native owned result absent")
    part = kimi_call_object(result["content"][0], ("type", "tool_call_id", "output"), ("is_error",))
    if (part["type"] != "tool_result" or part["tool_call_id"] != expected["call"]
            or ("is_error" in part and part["is_error"] is not False) or type(part["output"]) is not str
            or kimi_call_json(part["output"].encode()) != expected["facts"]):
        raise ValueError("Kimi complete native result/probe facts differ")
    if (final["role"] != "assistant" or "metadata" in final
            or final["content"] != [{"type": "text", "text": expected["sentinel"]}]):
        raise ValueError("Kimi complete native sentinel absent")
    return rows


def kimi_call_state(row, expected, workspace, project, *, initial=False, terminal=False):
    kimi_call_object(row, ("id", "workspace_id", "title", "created_at", "updated_at", "busy", "metadata", "agent_config", "usage", "permission_rules", "message_count", "last_seq"),
                     ("main_turn_active", "pending_interaction", "last_turn_reason", "archived", "archived_at", "current_prompt_id", "last_prompt"))
    if (row["id"] != expected["session"] or row["workspace_id"] != workspace or type(row["title"]) is not str
            or row["title"] not in ("", expected["prompt"]) or row["metadata"] != {"cwd": project}
            or row["agent_config"] != {"model": "" if initial else "uap-kimi-native-model"} or row["permission_rules"] != []
            or type(row["busy"]) is not bool or "archived_at" in row):
        raise ValueError("Kimi complete finite native session authority differs")
    created = kimi_call_instant(row["created_at"], expected["started"], expected["ended"])
    kimi_call_instant(row["updated_at"], created, expected["ended"])
    for key in ("message_count", "last_seq"):
        if type(row[key]) is not int or not 0 <= row[key] <= 1 << 40:
            raise ValueError("Kimi complete finite session sequence differs")
    for key in ("main_turn_active", "archived"):
        if key in row and (type(row[key]) is not bool or key == "archived" and row[key] is not False):
            raise ValueError("Kimi complete finite session boolean differs")
    if (("pending_interaction" in row and row["pending_interaction"] != "none")
            or ("last_turn_reason" in row and row["last_turn_reason"] != "completed")
            or ("last_prompt" in row and row["last_prompt"] != expected["prompt"])):
        raise ValueError("Kimi complete native failure/additional interaction")
    if "current_prompt_id" in row and (initial or row["current_prompt_id"] != expected["prompt_id"]):
        raise ValueError("Kimi complete native current prompt differs")
    usage = kimi_call_object(row["usage"], ("input_tokens", "output_tokens", "cache_read_tokens", "cache_creation_tokens", "context_tokens"), ("total_cost_usd", "context_limit", "turn_count"))
    for key, value in usage.items():
        if (type(value) not in ((int, float) if key == "total_cost_usd" else (int,)) or not 0 <= value <= 1 << 40):
            raise ValueError("Kimi complete finite native usage differs")
    if terminal and (row["busy"] is not False or row.get("main_turn_active") is not False or row.get("pending_interaction") != "none"
                     or row.get("last_turn_reason") != "completed" or row.get("archived") is not False):
        raise ValueError("Kimi complete native terminal completion absent")


def kimi_call_partial(page, terminal, sentinel):
    kimi_call_object(page, ("items", "has_more"))
    rows = page["items"]
    if page["has_more"] is not False or type(rows) is not list or not 1 <= len(rows) <= len(terminal):
        raise ValueError("Kimi complete intermediate history page differs")
    for index, row in enumerate(rows):
        if row == terminal[index]: continue
        kimi_call_object(row, ("id", "session_id", "role", "content", "created_at"))
        if index != len(terminal) - 1 or row["role"] != "assistant" or type(row["content"]) is not list or len(row["content"]) > 1:
            raise ValueError("Kimi complete intermediate native history prefix differs")
        if row["content"]:
            part = kimi_call_object(row["content"][0], ("type", "text"))
            if part["type"] != "text" or type(part["text"]) is not str or not sentinel.startswith(part["text"]):
                raise ValueError("Kimi complete intermediate native sentinel differs")
        if {**row, "content": terminal[index]["content"]} != terminal[index]:
            raise ValueError("Kimi complete intermediate native message identity differs")


def kimi_call_text(value, limit):
    return type(value) is str and len(value.encode("utf-8")) <= limit and "\x00" not in value


def kimi_call_number(value, low, high, integer=False):
    return type(value) in (int, float) and low <= value <= high and math.isfinite(value) and (not integer or value % 1 == 0)


def kimi_call_binding(value):
    # This fixed Kimi project add has no profile/delivery/pending-intent authority.
    fields = ("client_binding_id", "client_id", "scope", "target_locator", "physical_artifact_id", "materialization",
              "activation", "authentication", "policy", "verification", "data_receipt_id", "native_profile_root", "updated_at")
    binding = kimi_call_object(value, fields, ("package_revision", "affected_surfaces", "native_objects", "receipts"))
    if any(type(binding[key]) is not str for key in fields):
        raise ValueError("Kimi complete binding scalar type differs")
    profile = binding["native_profile_root"]
    if (not profile.startswith("/") or profile.startswith("//") or os.path.normpath(profile) != profile
            or Path(profile).name != ".kimi-code"):
        raise ValueError("Kimi complete canonical native profile/project differs")
    if "package_revision" in binding:
        revision = kimi_call_object(binding["package_revision"], ("tree_digest", "manifest_digest"), ("version",))
        if any(type(value) is not str for value in revision.values()):
            raise ValueError("Kimi complete binding revision type differs")
    if "affected_surfaces" in binding and (type(binding["affected_surfaces"]) is not list or any(type(value) is not str for value in binding["affected_surfaces"])):
        raise ValueError("Kimi complete binding surfaces type differs")
    for key in ("native_objects", "receipts"):
        if key not in binding:
            continue
        if type(binding[key]) is not list:
            raise ValueError("Kimi complete binding records type differs")
        for record in binding[key]:
            if key == "native_objects":
                record = kimi_call_object(record, ("object_id", "kind", "protection_class"),
                           ("logical_name", "path", "source_relative", "before_digest", "managed_digest", "user_modified"))
                if "user_modified" in record and type(record["user_modified"]) is not bool:
                    raise ValueError("Kimi complete binding native object type differs")
                scalars = set(record) - {"user_modified"}
            else:
                record = kimi_call_object(record, ("operation_id", "sequence", "mutation_type", "client_binding_id", "phase"),
                           ("data_receipt_id", "operation_group_id", "active_path", "staging_path", "backup_path", "before_digest", "after_digest"))
                if type(record["sequence"]) is not int or not -(1 << 63) <= record["sequence"] < 1 << 63:
                    raise ValueError("Kimi complete binding receipt sequence differs")
                scalars = set(record) - {"sequence"}
            if any(type(record[key]) is not str for key in scalars):
                raise ValueError("Kimi complete binding record scalar type differs")
    return binding


def kimi_call_request(body):
    # Pure parity with unchanged Go kimiAttemptRequest; no requester is invoked.
    request = kimi_call_object(kimi_call_json(body), ("model", "messages", "tools", "stream", "stream_options"),
                  ("max_tokens", "max_completion_tokens", "temperature", "top_p", "parallel_tool_calls", "prompt_cache_key"))
    if (request["model"] != "uap-kimi-native-model" or request["stream"] is not True
            or request["stream_options"] != {"include_usage": True}
            or request["stream_options"]["include_usage"] is not True):
        raise ValueError("Kimi complete provider envelope differs")
    if "prompt_cache_key" in request and (type(request["prompt_cache_key"]) is not str
            or not re.fullmatch(r"session_[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}", request["prompt_cache_key"])):
        raise ValueError("Kimi complete provider cache metadata differs")
    for key in ("max_tokens", "max_completion_tokens"):
        if key in request and not kimi_call_number(request[key], 1, 1 << 20, True):
            raise ValueError("Kimi complete provider token bound differs")
    for key, high in (("temperature", 2), ("top_p", 1)):
        if key in request and not kimi_call_number(request[key], 0, high):
            raise ValueError("Kimi complete provider sampling bound differs")
    if "parallel_tool_calls" in request and type(request["parallel_tool_calls"]) is not bool:
        raise ValueError("Kimi complete provider parallel flag differs")
    tools = request["tools"]
    if type(tools) is not list or len(tools) != 1:
        raise ValueError("Kimi complete provider sole installed declaration absent")
    declaration = kimi_call_object(tools[0], ("type", "function"))
    function = kimi_call_object(declaration["function"], ("name", "description", "parameters"))
    parameters = {"type": "object", "properties": {key: {"type": "string"} for key in ("nonce", "operation", "marker")}, "required": ["nonce"], "additionalProperties": False}
    if (declaration["type"] != "function" or function["name"] != "mcp__uap-kimi-native-a__inspect_runtime"
            or function["description"] != "Inspect only disposable fixture runtime facts."
            or function["parameters"] != parameters or function["parameters"]["additionalProperties"] is not False):
        raise ValueError("Kimi complete provider installed declaration differs")
    messages = request["messages"]
    if type(messages) is not list or not 1 <= len(messages) <= 256:
        raise ValueError("Kimi complete provider message inventory differs")
    for message in messages:
        kimi_call_object(message, ("role", "content"), ("tool_call_id", "tool_calls"))
        role, content = message["role"], message["content"]
        if role not in ("system", "user", "assistant", "tool"):
            raise ValueError("Kimi complete provider message role differs")
        if content is None:
            if role != "assistant" or message.get("tool_calls") is None:
                raise ValueError("Kimi complete provider null content differs")
        elif not kimi_call_text(content, 1 << 20):
            if type(content) is not list or not 1 <= len(content) <= 128:
                raise ValueError("Kimi complete provider content inventory differs")
            for part in content:
                kimi_call_object(part, ("type", "text"))
                if part["type"] != "text" or not kimi_call_text(part["text"], 1 << 20):
                    raise ValueError("Kimi complete provider content text differs")
        if "tool_call_id" in message and (role != "tool" or not kimi_call_text(message["tool_call_id"], 256)):
            raise ValueError("Kimi complete provider result identity differs")
        if role == "tool" and message.get("tool_call_id") is None:
            raise ValueError("Kimi complete provider result identity absent")
        if "tool_calls" in message:
            calls = message["tool_calls"]
            if role != "assistant" or type(calls) is not list or len(calls) != 1:
                raise ValueError("Kimi complete provider assistant call inventory differs")
            call = kimi_call_object(calls[0], ("id", "type", "function"))
            function = kimi_call_object(call["function"], ("name", "arguments"))
            if (call["type"] != "function" or not kimi_call_text(call["id"], 256)
                    or function["name"] != "mcp__uap-kimi-native-a__inspect_runtime"
                    or not kimi_call_text(function["arguments"], 4096)):
                raise ValueError("Kimi complete provider assistant function differs")
            kimi_call_json(function["arguments"].encode())
    return request


def kimi_call_complete(bodies, result):
    """Pure success admission recomputes raw joins; proof/flags are not authority."""
    result = kimi_call_json(bodies["runtime-attempt.json"])
    required = {"runtime-attempt.json", "probe-events.jsonl", "python-api.json", "native-data-proof.json",
                ".kimi-code-mcp.json.installed", "uap-kimi-native-a-SKILL.md.installed",
                "provider-request-1.json", "provider-request-2.json"}
    required |= {label + suffix for label in KIMI_CALL_COMMANDS for suffix in (".command.json", ".stdout.log", ".stderr.log")}
    if (set(bodies) != required or result.get("native_capture_complete") is not True
            or result.get("native_history_grammar_frozen") is not True or result.get("provider_returned") is not True
            or bodies["native-attempt.stdout.log"] != bodies["python-api.json"] or bodies["native-attempt.stderr.log"]):
        raise ValueError("Kimi complete closed successful capture required")
    for label in KIMI_CALL_COMMANDS:
        command = kimi_call_object(kimi_call_json(bodies[label + ".command.json"]), ("argv", "cwd", "exit", "error", "deadline", "cleanup"))
        if (type(command["exit"]) is not int or command["exit"] != 0 or command["error"] is not False or command["deadline"] is not False
                or command["cleanup"] != "ordinary full-descendant supervision; forced termination fails"):
            raise ValueError("Kimi complete public/native command failed")
        if (type(command["argv"]) is not list or not 1 <= len(command["argv"]) <= 32
                or any(not kimi_call_text(arg, 4096) or not arg for arg in command["argv"])
                or not kimi_call_text(command["cwd"], 4096) or not command["cwd"]):
            raise ValueError("Kimi complete public/native command text/arity differs")
    cleanup = kimi_call_object(result.get("provider_cleanup"), ("shutdown_complete", "forced_close", "error", "requests", "issued", "returned"))
    if (cleanup["shutdown_complete"] is not True or cleanup["forced_close"] is not False or cleanup["error"] is not False
            or type(cleanup["requests"]) is not int or cleanup["requests"] != 2
            or cleanup["issued"] is not True or cleanup["returned"] is not True):
        raise ValueError("Kimi complete ordinary provider closure absent")
    proof = kimi_call_object(result.get("native_history"), ("native_session_id", "native_turn_id", "provider_call_id", "approval_id",
        "prompt_id", "terminal_reason", "message_count", "snapshot_count", "history_sha256", "terminal_snapshot_sha256",
        "probe_pid", "probe_session", "started_at", "ended_at", "data_proof_sha256"))
    if (type(proof["message_count"]) is not int or proof["message_count"] not in (4, 5)
            or type(proof["probe_pid"]) is not int or not 1 <= proof["probe_pid"] <= 1 << 30):
        raise ValueError("Kimi complete native proof integer types/bounds differ")
    started, ended = kimi_call_instant(proof["started_at"]), kimi_call_instant(proof["ended_at"])
    if (not 0 <= result["elapsed_ms"] <= 180000 or not 0 <= (ended - started).total_seconds() <= 180
            or (ended - started).total_seconds() * 1000 > result["elapsed_ms"]
            or type(proof["native_turn_id"]) is not int or proof["native_turn_id"] != 0
            or type(result.get("native_turn_id")) is not int or result["native_turn_id"] != 0
            or proof["terminal_reason"] != "completed" or type(proof["snapshot_count"]) is not int
            or not 1 <= proof["snapshot_count"] <= 10):
        raise ValueError("Kimi complete original native turn/time differs")
    call = result.get("provider_call_id")
    owned = re.fullmatch(r"uap_([0-9a-f]{32})_attempt", call) if type(call) is str else None
    session = result.get("native_session_id")
    if (not owned or type(session) is not str or not re.fullmatch(r"session_[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}", session)
            or proof["native_session_id"] != session or proof["provider_call_id"] != call
            or type(proof["approval_id"]) is not str or not proof["approval_id"]):
        raise ValueError("Kimi complete session/call ownership differs")
    nonce = owned[1]
    public = kimi_call_object(kimi_call_json(bodies["add-a.stdout.log"]), ("schema_version", "command", "result", "data"))
    if type(public["schema_version"]) is not int or public["schema_version"] != 1 or public["command"] != "add" or public["result"] != "success" or type(public["data"]) is not dict:
        raise ValueError("Kimi complete public add result differs")
    targets = public["data"].get("targets")
    if type(targets) is not list or len(targets) != 1 or type(targets[0]) is not dict or targets[0].get("target") != "kimi" or type(targets[0].get("output")) is not dict:
        raise ValueError("Kimi complete public selected target differs")
    installation = targets[0]["output"].get("result")
    if (type(installation) is not dict or installation.get("mutated") is not True
            or not kimi_call_text(installation.get("installation_id"), 64) or not installation["installation_id"]):
        raise ValueError("Kimi complete public installation result differs")
    installation_id = installation["installation_id"]
    physical_id = "uap-kimi-native-a-" + hashlib.sha256(installation_id.encode()).hexdigest()[:12]
    binding = kimi_call_binding(result.get("binding"))
    if (binding.get("client_id") != "kimi" or binding.get("scope") != "project"
            or binding.get("activation") != "prepared" or binding.get("authentication") != "not_checked"
            or binding.get("verification") != "installation_verified" or binding.get("materialization") != "materialized"):
        raise ValueError("Kimi complete selected project binding absent")
    active, data = binding.get("target_locator"), result.get("data_locator")
    if (type(active) is not str or binding.get("physical_artifact_id") != physical_id
            or binding.get("client_binding_id") != "client_" + hashlib.sha256("\x00".join((installation_id, "kimi", "project", active)).encode()).hexdigest()[:24]
            or binding.get("data_receipt_id") != "data_" + hashlib.sha256("\x00".join((installation_id, physical_id, "project")).encode()).hexdigest()[:24]):
        raise ValueError("Kimi complete public installation/binding/data identity differs")
    facts = kimi_call_object(result.get("probe_facts"), ("nonce", "revision", "cwd", "argv", "plugin_root", "plugin_data", "literal", "once", "marker", "pid", "session"))
    expected_facts = {"nonce": nonce + "-a", "revision": "a-one", "cwd": active, "argv": ["Kimi owned a", str(active) + "/argument"],
                      "plugin_root": active, "plugin_data": data, "literal": "", "once": "", "marker": "retained-" + nonce,
                      "pid": facts["pid"], "session": facts["session"]}
    if (type(active) is not str or not active.startswith("/") or type(data) is not str or not data.startswith("/")
            or facts != expected_facts or type(facts["pid"]) is not int or not 1 <= facts["pid"] <= 1 << 30
            or type(facts["session"]) is not str or not re.fullmatch(str(facts["pid"]) + r"-[0-9]{15,20}", facts["session"])
            or proof["probe_pid"] != facts["pid"] or proof["probe_session"] != facts["session"]):
        raise ValueError("Kimi complete returned runtime authority differs")
    events = bodies["probe-events.jsonl"]
    if not events.endswith(b"\n") or not 3 <= len(events.splitlines()) <= 6:
        raise ValueError("Kimi complete probe stream differs")
    previous = started
    for index, line in enumerate(events.splitlines()):
        event = kimi_call_object(kimi_call_json(line), ("method", "pid", "session", "nonce", "timestamp", "facts"))
        method = "initialize" if index == 0 else "tools/call" if index == len(events.splitlines()) - 1 else "tools/list"
        if (event["method"] != method or type(event["pid"]) is not int or event["pid"] != facts["pid"]
                or event["session"] != facts["session"] or event["nonce"] != facts["nonce"]
                or event["facts"] != (facts if method == "tools/call" else None)):
            raise ValueError("Kimi complete fresh probe/result join differs")
        previous = kimi_call_instant(event["timestamp"], previous, ended)
    requests = [kimi_call_request(bodies[f"provider-request-{number}.json"]) for number in (1, 2)]
    if any(request.get("prompt_cache_key") != session for request in requests):
        raise ValueError("Kimi complete provider session metadata changed")
    first, second = requests[0]["messages"], requests[1]["messages"]
    if (len(second) != len(first) + 2 or first != second[:-2] or any(row.get("role") == "tool" or "tool_calls" in row for row in first)
            or second[-2].get("role") != "assistant" or second[-1].get("role") != "tool" or second[-1].get("tool_call_id") != call):
        raise ValueError("Kimi complete provider two-turn ordering differs")
    calls = second[-2].get("tool_calls")
    args = {"nonce": nonce + "-a", "operation": "write", "marker": "retained-" + nonce}
    if (type(calls) is not list or len(calls) != 1 or calls[0].get("id") != call or calls[0].get("type") != "function"
            or calls[0].get("function", {}).get("name") != "mcp__uap-kimi-native-a__inspect_runtime"
            or kimi_call_json(calls[0]["function"]["arguments"].encode()) != args):
        raise ValueError("Kimi complete provider owned assistant call differs")
    returned = second[-1].get("content")
    match = re.fullmatch(r"Wall time: ((?:0|[1-9][0-9]?)\.[0-9]{3}) seconds\n([\s\S]+)", returned) if type(returned) is str else None
    if not match or float(match[1]) > 60 or kimi_call_json(match[2].encode()) != facts:
        raise ValueError("Kimi complete model/native result framing differs")
    data_body = bodies["native-data-proof.json"]
    data_proof = kimi_call_object(kimi_call_json(data_body), ("schema_version", "before", "after_without_marker", "marker_body", "marker_bytes", "marker_sha256"))
    before = data_proof["before"]
    if (type(data_proof["schema_version"]) is not int or data_proof["schema_version"] != 1
            or data_proof["marker_body"] != facts["marker"] or type(data_proof["marker_bytes"]) is not int
            or data_proof["marker_bytes"] != len(facts["marker"].encode())
            or data_proof["marker_sha256"] != hashlib.sha256(facts["marker"].encode()).hexdigest()
            or proof["data_proof_sha256"] != hashlib.sha256(data_body).hexdigest()
            or type(before) is not dict or not 1 <= len(before) <= 4096 or before != data_proof["after_without_marker"]
            or "." not in before or ".agentplugins-data-owner.json" not in before or "native-marker.txt" in before):
        raise ValueError("Kimi complete observed marker/unrelated-data proof differs")
    for name, value in before.items():
        path = Path(name)
        if (not kimi_call_text(name, 4096) or path.is_absolute() or str(path) != name or ".." in path.parts
                or "\\" in name or len(path.parts) > 32 or type(value) is not str
                or not re.fullmatch(r"0[0-7]{1,12}:(?:directory|[0-9a-f]{64})", value)):
            raise ValueError("Kimi complete owned inventory entry differs")
        if any(not before.get(str(parent), "").endswith(":directory") for parent in path.parents):
            raise ValueError("Kimi complete owned inventory parent missing")
    if not before["."].endswith(":directory") or before[".agentplugins-data-owner.json"].endswith(":directory"):
        raise ValueError("Kimi complete owned inventory root/owner differs")
    envelope = kimi_call_object(kimi_call_json(bodies["python-api.json"]), ("schema_version", "files"))
    raw_files = envelope["files"]
    if (len(bodies["python-api.json"]) > 1 << 20 or type(envelope["schema_version"]) is not int or envelope["schema_version"] != 1
            or type(raw_files) is not dict or not 4 <= len(raw_files) <= 64
            or any(type(value) is not str or str(Path(name)) != name or len(Path(name).parts) != 1 or "\\" in name for name, value in raw_files.items())):
        raise ValueError("Kimi complete closed raw envelope differs")
    raw_files = {name: value.encode() for name, value in raw_files.items()}
    research = kimi_call_object(kimi_call_json(raw_files["research.json"]), ("schema_version", "client", "phase", "commands", "http_requests", "observations", "qualification"))
    if (type(research["schema_version"]) is not int or research["schema_version"] != 1
            or research["client"] != "kimi" or research["phase"] != "installed-single-tool-attempt"
            or type(research["qualification"]) is not dict or set(research["qualification"]) != set(shared.QUALIFICATION_CASES)
            or any(value is not False for value in research["qualification"].values())):
        raise ValueError("Kimi complete observer capture boundary differs")
    observation = kimi_call_object(research["observations"], ("native_capture_complete", "native_history_grammar_frozen", "native_session_id", "native_turn_id", "provider_call_id", "approval_id", "snapshot_count", "api_request_count", "boundary"))
    if (type(observation) is not dict or observation.get("native_capture_complete") is not False or observation.get("native_history_grammar_frozen") is not False
            or observation.get("native_session_id") != session or observation.get("provider_call_id") != call
            or observation.get("approval_id") != proof["approval_id"] or type(observation.get("native_turn_id")) is not int or observation["native_turn_id"] != 0
            or type(observation.get("snapshot_count")) is not int or observation["snapshot_count"] != proof["snapshot_count"]
            or observation["boundary"] != "Bounded initial capture; no native history or qualification inferred from provider/probe success"):
        raise ValueError("Kimi complete observer owned session differs")
    commands = research["commands"]
    native_cleanup = {"forced_kill": False, "group_reaped": True, "port_closed": True, "shutdown_acknowledged": True, "sigterm_sent": False}
    if type(commands) is not list or len(commands) != 1:
        raise ValueError("Kimi complete native command inventory differs")
    command = kimi_call_object(commands[0], ("argv", "cwd", "probe", "elapsed_seconds", "error", "exit", "cleanup",
                               "stdout_file", "stdout_bytes", "stdout_sha256", "stderr_file", "stderr_bytes", "stderr_sha256"))
    if (type(command["exit"]) is not int or command["exit"] != 0 or command["error"] is not None or command["probe"] != "kimi-single-tool"
            or not kimi_call_number(command["elapsed_seconds"], 0, 60) or command["cleanup"] != native_cleanup
            or any(type(value) is not bool for value in command["cleanup"].values())):
        raise ValueError("Kimi complete native command schema/ordinary closure differs")
    used = {"research.json"}
    for channel in ("stdout", "stderr"):
        command = commands[0]
        name = command.get(channel + "_file")
        stream = raw_files.get(name)
        if (name != "kimi-single-tool." + channel + ".log" or stream is None
                or type(command.get(channel + "_bytes")) is not int or len(stream) != command[channel + "_bytes"]
                or hashlib.sha256(stream).hexdigest() != command.get(channel + "_sha256") or channel == "stderr" and stream):
            raise ValueError("Kimi complete native stream receipt differs")
        used.add(name)
    http_rows = research["http_requests"]
    if (type(http_rows) is not list or not 1 <= len(http_rows) <= 60 or type(observation.get("api_request_count")) is not int
            or observation["api_request_count"] != len(http_rows)):
        raise ValueError("Kimi complete native HTTP budget differs")
    responses, response_bodies, labels, http_receipts = {}, {}, [], {}
    for row in http_rows:
        kimi_call_object(row, ("authenticated", "error", "label", "method", "path", "response_bytes", "response_file", "response_sha256", "status"))
        label, name = row["label"], row["response_file"]
        stream = raw_files.get(name)
        if (type(label) is not str or label in responses or name in used or name != "kimi-single-tool-" + label + ".http.json" or stream is None
                or row["error"] is not None or type(row["response_bytes"]) is not int or len(stream) != row["response_bytes"]
                or hashlib.sha256(stream).hexdigest() != row["response_sha256"] or type(row["authenticated"]) is not bool
                or type(row["status"]) is not int or row["status"] != (401 if label == "unauthorized" else 200)
                or row["authenticated"] is not (label not in ("health", "unauthorized"))):
            raise ValueError("Kimi complete native HTTP receipt differs")
        used.add(name); labels.append(label); response_bodies[label] = stream; http_receipts[label] = row
        response = kimi_call_object(kimi_call_json(stream), ("code", "msg", "data", "request_id"), ("details", "stack"))
        if type(response["code"]) is not int or response["code"] != (40101 if label == "unauthorized" else 0):
            raise ValueError("Kimi complete native HTTP envelope differs")
        if (not kimi_call_text(response["request_id"], 256) or not response["request_id"]
                or not kimi_call_text(response["msg"], 1024) or label == "unauthorized" and response["data"] is not None):
            raise ValueError("Kimi complete native HTTP response identity/data differs")
        if "stack" in response and response["stack"] is not None and not kimi_call_text(response["stack"], 32768):
            raise ValueError("Kimi complete native negative stack differs")
        if label != "unauthorized" and (response["msg"] != "success" or response.get("details") is not None or response.get("stack") is not None):
            raise ValueError("Kimi complete native HTTP success differs")
        responses[label] = response["data"]
    if set(raw_files) != used:
        raise ValueError("Kimi complete unrecorded raw member")
    for label in ("health", "shutdown"):
        value = kimi_call_object(responses.get(label), ("ok",))
        if value["ok"] is not True:
            raise ValueError("Kimi complete native health/shutdown acknowledgement differs")
    meta = kimi_call_object(responses.get("meta"), ("server_version", "capabilities", "server_id", "started_at", "open_in_apps", "dangerous_bypass_auth", "backend"),
                            ("experimental_flags", "features", "web_title"))
    if meta["server_version"] != "2.3.2" or meta["backend"] != "v2" or meta["dangerous_bypass_auth"] is not False:
        raise ValueError("Kimi complete pinned native metadata differs")
    expected = {"facts": facts, "session": session, "call": call, "prompt_id": proof["prompt_id"], "started": started, "ended": ended,
                "prompt": "Call the one installed fixture tool once with the exact nonce and marker; then report completion. " + nonce + "-a", "sentinel": "UAP_KIMI_DONE_" + nonce}
    project = str(Path(binding["native_profile_root"]).parent)
    root = str(Path(project).parent)
    projects = {"a": project, "b": root + "/project-b", "c": root + "/project-c", "user": root + "/outside"}
    physical = binding.get("physical_artifact_id")
    if (type(physical) is not str or not re.fullmatch(r"uap-kimi-native-a-[0-9a-f]{12}", physical)
            or active != project + "/.uap/managed/clients/kimi/" + physical
            or data != project + "/.uap/plugin-data/" + physical):
        raise ValueError("Kimi complete fixed managed/data layout differs")
    server = {"transport": "stdio", "command": active + "/io.github.777genius.agentplugins/managed-stdio-v1/agentplugins",
              "args": ["--internal-stdio-v1", active, data, active, "plugin", "--", "./bin/probe", "Kimi owned a", active + "/argument"],
              "env": {"PLUGIN_ROOT": active, "PLUGIN_DATA": data, "UAP_TEST_ROOT": root, "UAP_TEST_EVENTS": root + "/a-events.jsonl",
                      "UAP_TEST_NONCE": nonce + "-a", "UAP_TEST_REVISION": "a-one"}}
    if kimi_call_json(bodies[".kimi-code-mcp.json.installed"]) != {"mcpServers": {"uap-kimi-native-a": server}}:
        raise ValueError("Kimi complete installed source projection differs")
    skill = "---\nname: uap-kimi-native-a\ndescription: Kimi installed a-one\n---\nUAP_KIMI_a-one_" + nonce + "\n"
    if bodies["uap-kimi-native-a-SKILL.md.installed"] != skill.encode():
        raise ValueError("Kimi complete installed authored skill differs")
    workspaces = {}
    for scope in ("a", "b", "c"):
        trust = kimi_call_object(responses.get("trust-before-" + scope), ("trusted",))
        if trust["trusted"] is not False:
            raise ValueError("Kimi complete native workspace/trust differs")
    for label in ("trust-set-a", "trust-after-a", "trust-set-b", "trust-after-b", "trust-untrusted-c"):
        trust = kimi_call_object(responses.get(label), ("trusted",))
        if trust["trusted"] is not (label != "trust-untrusted-c"):
            raise ValueError("Kimi complete owned native trust differs")
    for scope in ("a", "b", "c"):
        workspace = kimi_call_object(responses.get("workspace-" + scope), ("id", "root", "name", "created_at", "last_opened_at", "session_count"))
        if (not kimi_call_text(workspace["id"], 256) or not re.fullmatch(r"wd_[a-z0-9._-]+_[0-9a-f]{12}", workspace["id"])
                or workspace["root"] != projects[scope] or workspace["name"] != "uap-kimi-native-" + scope
                or type(workspace["session_count"]) is not int or workspace["session_count"] != 0):
            raise ValueError("Kimi complete native workspace/trust differs")
        workspaces[scope] = workspace["id"]
        for key in ("created_at", "last_opened_at"): kimi_call_instant(workspace[key], started, ended)
    if len(set(workspaces.values())) != 3:
        raise ValueError("Kimi complete distinct native workspace/trust differs")
    projected = {key: value for key, value in server.items() if key != "env"}
    projected["envKeys"] = sorted(server["env"])
    expected_catalog = [{"name": "uap-kimi-native-a", "config": projected, "source": "global",
                         "origin": project + "/.kimi-code/mcp.json", "mutable": False}]
    if (responses.get("catalog-a") != expected_catalog or responses["catalog-a"][0]["mutable"] is not False
            or any(responses.get("catalog-" + scope) != [] for scope in ("b", "c", "user"))):
        raise ValueError("Kimi complete receipt-selected native catalog differs")
    skills = kimi_call_object(responses.get("skills-a"), ("skills",))["skills"]
    if type(skills) is not list or len(skills) > 512:
        raise ValueError("Kimi complete native skill inventory differs")
    selected = []
    for skill in skills:
        kimi_call_object(skill, ("name", "description", "path", "source"), ("type", "disable_model_invocation", "scopes"))
        if not kimi_call_text(skill["name"], 4096):
            raise ValueError("Kimi complete native skill name differs")
        if skill["name"].startswith("uap-kimi-native-"):
            selected.append(skill)
    if (len(selected) != 1 or selected[0]["name"] != "uap-kimi-native-a" or selected[0]["description"] != "Kimi installed a-one"
            or selected[0]["path"] != project + "/.kimi-code/skills/uap-kimi-native-a/SKILL.md" or selected[0]["source"] != "project"):
        raise ValueError("Kimi complete selected native skill identity differs")
    # Pinned prompts.ts projects only clientMetadata[0]. The fixed request has
    # no metadata, so even an origin wrapper/null would be unrequested here.
    prompt = kimi_call_object(responses.get("prompt"), ("prompt_id", "user_message_id", "status", "content", "created_at"))
    if (prompt["prompt_id"] != proof["prompt_id"] or prompt["user_message_id"] != proof["prompt_id"]
            or prompt["status"] not in ("running", "queued", "blocked") or prompt["content"] != [{"type": "text", "text": expected["prompt"]}]):
        raise ValueError("Kimi complete accepted native prompt differs")
    kimi_call_instant(prompt["created_at"], started, ended)
    base = "/api/v1/sessions/" + session
    fixed_endpoints = {"health": ("GET", "/api/v1/healthz"), "unauthorized": ("GET", "/api/v1/meta"), "meta": ("GET", "/api/v1/meta"),
                       "create-session": ("POST", "/api/v1/sessions"), "prompt": ("POST", base + "/prompts"),
                       "approve-once": ("POST", base + "/approvals/" + proof["approval_id"]), "history": ("GET", base + "/messages?page_size=20"),
                       "shutdown": ("POST", "/api/v1/shutdown"), "skills-a": ("GET", "/api/v1/workspaces/" + workspaces["a"] + "/skills")}
    for scope in ("a", "b", "c"):
        fixed_endpoints["workspace-" + scope] = ("POST", "/api/v1/workspaces")
        fixed_endpoints["trust-before-" + scope] = ("GET", "/api/v1/workspaces/" + workspaces[scope] + "/trust")
    for scope in ("a", "b"):
        fixed_endpoints["trust-set-" + scope] = ("POST", "/api/v1/workspaces/" + workspaces[scope] + "/trust")
        fixed_endpoints["trust-after-" + scope] = ("GET", "/api/v1/workspaces/" + workspaces[scope] + "/trust")
    fixed_endpoints["trust-untrusted-c"] = ("GET", "/api/v1/workspaces/" + workspaces["c"] + "/trust")
    for scope in ("a", "b", "c", "user"):
        fixed_endpoints["catalog-" + scope] = ("GET", "/api/v2/mcp/servers?" + urlencode({"cwd": projects[scope]}))
    for label, receipt in http_receipts.items():
        endpoint = fixed_endpoints.get(label)
        if label.startswith("approvals-"): endpoint = ("GET", base + "/approvals?status=pending")
        if label.startswith("snapshot-"): endpoint = ("GET", base + "/snapshot")
        if endpoint is None or (receipt["method"], receipt["path"]) != endpoint:
            raise ValueError("Kimi complete exact native endpoint differs")
    version = kimi_call_json(bodies["version.command.json"])
    argv = version["argv"]
    if (type(argv) is not list or len(argv) != 2 or type(argv[0]) is not str or not argv[0].startswith("/")
            or argv[1] != "--version" or version["cwd"] != projects["user"]
            or bodies["version.stdout.log"] != b"2.3.2\n" or bodies["version.stderr.log"]):
        raise ValueError("Kimi complete exact native version command differs")
    client = argv[0]
    if commands[0].get("argv") != [client, "web", "--no-open", "--host", "127.0.0.1", "--port", "0"] or commands[0].get("cwd") != project:
        raise ValueError("Kimi complete original native service command differs")
    for scope in ("a", "b", "c"):
        command = kimi_call_json(bodies["git-" + scope + ".command.json"])
        argv = command["argv"]
        if len(argv) != 5 or not argv[0].startswith("/") or argv[1:] != ["-c", "core.hooksPath=/dev/null", "init", "--quiet"] or command["cwd"] != projects[scope]:
            raise ValueError("Kimi complete fixed Git command differs")
    command = kimi_call_json(bodies["add-a.command.json"])
    if (len(command["argv"]) != 12 or not command["argv"][0].startswith("/")
            or command["argv"][1:] != ["add", root + "/sources/a", "--no-save", "--target", "kimi", "--format", "json", "--scope", "project", "--project-root", project]
            or command["cwd"] != project):
        raise ValueError("Kimi complete fixed public add command differs")
    command = kimi_call_json(bodies["native-attempt.command.json"])
    argv = command["argv"]
    if (len(argv) != 4 or not argv[0].startswith("/") or not argv[1].endswith("/scripts/project_native_collectors/kimi_attempt.py")
            or argv[2:] != ["--attempt-request", root + "/attempt-request.json"] or command["cwd"] != projects["user"]):
        raise ValueError("Kimi complete fixed native collector command differs")
    kimi_call_state(responses.get("create-session"), expected, workspaces["a"], project, initial=True)
    previous_seq, epoch, terminal, snapshots = -1, None, None, []
    for index in range(proof["snapshot_count"]):
        snapshot = kimi_call_object(responses.get("snapshot-" + str(index)), ("as_of_seq", "epoch", "session", "messages", "in_flight_turn", "pending_approvals", "pending_questions", "subagents"))
        state = snapshot["session"]
        if (type(snapshot["as_of_seq"]) is not int or not 0 <= snapshot["as_of_seq"] <= 1 << 40
                or snapshot["as_of_seq"] < previous_seq or type(snapshot["epoch"]) is not str
                or not 1 <= len(snapshot["epoch"].encode("utf-8")) <= 256 or epoch is not None and snapshot["epoch"] != epoch
                or snapshot["pending_approvals"] != [] or snapshot["pending_questions"] != [] or snapshot["subagents"] != []
                or type(state) is not dict or state.get("id") != session or state.get("last_turn_reason") in ("failed", "cancelled")
                or state.get("metadata") != {"cwd": str(Path(binding["native_profile_root"]).parent)}
                or state.get("agent_config") != {"model": "uap-kimi-native-model"} or state.get("permission_rules") != []
                or state.get("workspace_id") != workspaces["a"]):
            raise ValueError("Kimi complete native snapshot authority/state differs")
        kimi_call_state(state, expected, workspaces["a"], project, terminal=index == proof["snapshot_count"] - 1)
        in_flight = snapshot["in_flight_turn"]
        if index != proof["snapshot_count"] - 1:
            kimi_call_object(in_flight, ("turn_id", "assistant_text", "thinking_text", "running_tools"), ("current_prompt_id",))
            if (type(in_flight["turn_id"]) is not int or in_flight["turn_id"] != 0 or type(in_flight["assistant_text"]) is not str
                    or not expected["sentinel"].startswith(in_flight["assistant_text"]) or in_flight["thinking_text"] != ""
                    or type(in_flight["running_tools"]) is not list or len(in_flight["running_tools"]) > 1
                    or ("current_prompt_id" in in_flight and in_flight["current_prompt_id"] != proof["prompt_id"])):
                raise ValueError("Kimi complete finite in-flight turn differs")
            for tool in in_flight["running_tools"]:
                kimi_call_object(tool, ("tool_call_id", "name"), ("args",))
                if tool["tool_call_id"] != call or tool["name"] != "mcp__uap-kimi-native-a__inspect_runtime" or ("args" in tool and tool["args"] != args):
                    raise ValueError("Kimi complete additional in-flight tool")
        previous_seq, epoch = snapshot["as_of_seq"], snapshot["epoch"]
        terminal = snapshot
        snapshots.append(snapshot)
    if (terminal["in_flight_turn"] is not None or terminal["session"].get("last_turn_reason") != "completed"
            or terminal["session"].get("busy") is not False or terminal["session"].get("main_turn_active") is not False
            or terminal["session"].get("pending_interaction") != "none" or terminal["session"].get("archived") is not False):
        raise ValueError("Kimi complete successful terminal native turn absent")
    rows = kimi_call_messages(terminal["messages"], expected)
    for snapshot in snapshots[:-1]: kimi_call_partial(snapshot["messages"], rows, expected["sentinel"])
    history = kimi_call_object(responses.get("history"), ("items", "has_more"))
    if history["has_more"] is not False or history["items"] != rows[::-1] or proof["message_count"] != len(rows):
        raise ValueError("Kimi complete snapshot/history identity differs")
    if (proof["history_sha256"] != hashlib.sha256(response_bodies["history"]).hexdigest()
            or proof["terminal_snapshot_sha256"] != hashlib.sha256(response_bodies["snapshot-" + str(proof["snapshot_count"] - 1)]).hexdigest()):
        raise ValueError("Kimi complete raw history proof digest differs")
    pending = [label for label in labels if label.startswith("approvals-")]
    if not 1 <= len(pending) <= 20 or pending != ["approvals-" + str(index) for index in range(len(pending))]:
        raise ValueError("Kimi complete native approval sequence differs")
    for index, label in enumerate(pending):
        page = kimi_call_object(responses[label], ("items",))
        if type(page["items"]) is not list or len(page["items"]) != (1 if index == len(pending) - 1 else 0):
            raise ValueError("Kimi complete sole native approval absent")
    approval = kimi_call_object(responses[pending[-1]]["items"][0], ("approval_id", "session_id", "agent_id", "turn_id", "tool_call_id", "tool_name", "action", "tool_input_display", "created_at", "expires_at"))
    kimi_call_instant(approval["created_at"], started, ended)
    if kimi_call_instant(approval["expires_at"]) <= kimi_call_instant(approval["created_at"]):
        raise ValueError("Kimi complete native approval expiry differs")
    resolved = kimi_call_object(responses.get("approve-once"), ("resolved", "resolved_at"))
    kimi_call_instant(resolved["resolved_at"], started, ended)
    action = "Approve mcp__uap-kimi-native-a__inspect_runtime"
    if (approval.get("approval_id") != proof["approval_id"] or approval.get("session_id") != session or approval.get("agent_id") != "main"
            or type(approval.get("turn_id")) is not int or approval["turn_id"] != 0 or approval.get("tool_call_id") != call
            or approval.get("tool_name") != "mcp__uap-kimi-native-a__inspect_runtime" or approval.get("action") != action
            or approval.get("tool_input_display") != {"kind": "generic", "summary": action, "detail": args}
            or responses.get("approve-once", {}).get("resolved") is not True):
        raise ValueError("Kimi complete native one-call approval differs")
    prefix = ["health", "unauthorized", "meta"]
    for scope in ("a", "b", "c"): prefix += ["workspace-" + scope, "trust-before-" + scope]
    prefix += ["trust-set-a", "trust-after-a", "trust-set-b", "trust-after-b", "trust-untrusted-c", "catalog-a", "catalog-b", "catalog-c", "catalog-user", "skills-a", "create-session", "prompt"]
    if labels != prefix + pending + ["approve-once"] + ["snapshot-" + str(index) for index in range(proof["snapshot_count"])] + ["history", "shutdown"]:
        raise ValueError("Kimi complete fixed native HTTP inventory differs")


def kimi_call_members(bodies, source):
    if any(b"uap-kimi-key-" in body for body in bodies.values()):
        raise ValueError("unredacted Kimi fixture provider key refused")
    allowed = {"runtime-attempt.json", "probe-events.jsonl", "python-api.json",
               ".kimi-code-mcp.json.installed", "uap-kimi-native-a-SKILL.md.installed", "native-data-proof.json"}
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
    success = result.get("runtime_attempt_complete") is True and result.get("test_failed") is False if isinstance(result, dict) else False
    failed = result.get("runtime_attempt_complete") is False and result.get("test_failed") is True if isinstance(result, dict) else False
    if (not isinstance(result, dict) or result.get("client") != "kimi" or result.get("version") != "2.3.2"
            or result.get("phase") != "installed-single-tool-attempt" or result.get("activation") != "prepared"
            or result.get("provider_kind") != "scripted-openai-chat-completions"
            or not (success or failed)
            or result.get("vendor_authentication_qualified") is not False or result.get("real_model_qualified") is not False
            or type(result.get("elapsed_ms")) is not int or result["elapsed_ms"] < 0):
        raise ValueError("Kimi single-call completion/failure pair differs")
    qualification = result.get("qualification")
    if (not isinstance(qualification, dict) or set(qualification) != set(shared.QUALIFICATION_CASES)
            or any(qualification[key] is not (success and key in ("mcp-initialize", "mcp-tools-list", "mcp-tools-call")) for key in shared.QUALIFICATION_CASES)):
        raise ValueError("Kimi single-call capture cannot inherit lifecycle qualifications")
    if result.get("source_identity") != {"installer_base_commit": source["source_commit"], "installer_tree": source["source_tree"],
                                        "installer_source_state": "committed", "installer_patch_sha256": []}:
        raise ValueError("Kimi single-call source identity differs")
    numbers = sorted(int(name.removeprefix("provider-request-").removesuffix(".json"))
                     for name in bodies if name.startswith("provider-request-"))
    if numbers != list(range(1, len(numbers) + 1)):
        raise ValueError("Kimi provider request capture sequence has gaps")
    if success:
        try:
            kimi_call_complete(bodies, result)
        except (KeyError, TypeError, AttributeError, IndexError, RecursionError) as error:
            raise ValueError("Kimi complete raw container refused") from error
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
                         ("AGENTPLUGINS_PROJECT_NATIVE_GEMINI_EXECUTABLE" if client == "gemini" else "AGENTPLUGINS_" + client.upper() + "_BIN"): str(binary),
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
