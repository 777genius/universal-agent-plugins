"""Run a closed current-source native test on an actual disposable hosted runner.

Acquisition and bounded process capture reuse the existing research runner. A
valid artifact never changes a failed test into a passed qualification.
"""
import argparse
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

SOURCE = shared.SOURCE
CASES = {
    "cline": {"test": "TestAgentpluginsClineNativeProjectRuntimeAttempt",
              "file": "repotests/agentplugins_cline_project_native_attempt_test.go",
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


def cli_attempt_members(bodies, source, client):
    if client not in CLI_ATTEMPTS:
        raise ValueError("unknown fixed native CLI attempt")
    allowed = {"runtime-attempt.json", "probe-events.jsonl"}
    commands = CLI_COMMANDS + (("native-catalog",) if client == "qoder" else ())
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
    return result


def cline_members(bodies, source):
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
            "-test.run=^TestAgentpluginsClineNativeProjectRuntimeAttempt$", "-test.count=1", "-test.v", "-test.timeout=180s"]


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
