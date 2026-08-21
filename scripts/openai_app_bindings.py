#!/usr/bin/env python3
"""Load and validate host-only ChatGPT app bindings."""

from __future__ import annotations

import hashlib
import json
import re
from datetime import date, datetime
from pathlib import Path
from urllib.parse import urlsplit
from zoneinfo import ZoneInfo


ROOT = Path(__file__).resolve().parents[1]
APP_BINDINGS = ROOT / "compat" / "openai" / "app-bindings.json"
PLUGIN_NAME = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")
APP_ID_TOKEN = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._~:-]{0,255}$")
RUNTIME_EVIDENCE_PATH = re.compile(
    r"^tests/e2e/results/[a-z0-9-]+-(?P<date>[0-9]{4}-[0-9]{2}-[0-9]{2})\.json$"
)
EVIDENCE_TIMEZONE = ZoneInfo("Europe/Kyiv")
ENTRY_FIELDS = {
    "app_key",
    "id",
    "mcp_server",
    "mcp_url",
    "runtime_evidence",
    "runtime_evidence_revision",
    "runtime_evidence_digest",
    "personal_app_evidence",
    "personal_app_evidence_revision",
    "personal_app_evidence_digest",
    "registration",
}
DIRECT_RUNTIME_CHECKS = {
    "connect": "passed",
    "list_resources": "passed",
    "search_cloudflare_documentation": "passed",
    "package_ui_install": "skipped",
}
PERSONAL_APP_CHECKS = {
    "plugins_personal_installed": "passed",
    "plugin_detail_try_in_chat": "passed",
    "new_chat_plugin_chip_selected": "passed",
    "list_resources": "passed",
    "search_cloudflare_documentation": "passed",
    "assistant_response_marker": "passed",
    "local_codex_plugin_package_ingestion": "skipped",
    "agentplugins_manager_lifecycle": "skipped",
}


def _unique_object(pairs: list[tuple[str, object]]) -> dict[str, object]:
    result: dict[str, object] = {}
    for key, value in pairs:
        if key in result:
            raise ValueError(f"duplicate JSON field: {key}")
        result[key] = value
    return result


def _require_https_url(value: object, field: str) -> str:
    if not isinstance(value, str):
        raise ValueError(f"{field} must be a string")
    parsed = urlsplit(value)
    if (
        parsed.scheme != "https"
        or not parsed.hostname
        or parsed.username is not None
        or parsed.password is not None
        or parsed.query
        or parsed.fragment
    ):
        raise ValueError(f"{field} must be credential-free HTTPS without query or fragment")
    return value


def _load_object(path: Path, description: str) -> dict[str, object]:
    try:
        value = json.loads(path.read_text(), object_pairs_hook=_unique_object)
    except (OSError, json.JSONDecodeError, ValueError) as error:
        raise ValueError(f"{path}: invalid {description} JSON: {error}") from error
    if not isinstance(value, dict):
        raise ValueError(f"{path}: {description} must contain an object")
    return value


def _runtime_evidence_path(value: object, root: Path, field: str) -> Path:
    if not isinstance(value, str) or not RUNTIME_EVIDENCE_PATH.fullmatch(value):
        raise ValueError(f"{field} must be a repository-relative client evidence path")
    relative = Path(value)
    resolved_root = root.resolve()
    resolved = (resolved_root / relative).resolve()
    if not resolved.is_relative_to(resolved_root) or not resolved.is_file():
        raise ValueError(f"{field} must reference an existing in-repository file")
    return resolved


def _validate_evidence_identity(
    binding: dict[str, object], evidence_field: str, evidence_path: Path, prefix: str
) -> None:
    revision_field = f"{evidence_field}_revision"
    digest_field = f"{evidence_field}_digest"
    revision = binding.get(revision_field)
    digest = binding.get(digest_field)
    if not isinstance(revision, str) or not re.fullmatch(r"[0-9a-f]{40}", revision):
        raise ValueError(f"{prefix}.{revision_field}: expected a full commit SHA")
    if not isinstance(digest, str) or not re.fullmatch(r"sha256:[0-9a-f]{64}", digest):
        raise ValueError(f"{prefix}.{digest_field}: expected a SHA-256 digest")
    actual = "sha256:" + hashlib.sha256(evidence_path.read_bytes()).hexdigest()
    if actual != digest:
        raise ValueError(f"{prefix}.{digest_field}: evidence digest does not match")


def _validate_evidence_date(
    binding: dict[str, object],
    evidence_field: str,
    evidence: dict[str, object],
    evidence_path: Path,
) -> None:
    evidence_date = evidence.get("date")
    try:
        observed_date = date.fromisoformat(str(evidence_date))
    except ValueError as error:
        raise ValueError(f"{evidence_path}: invalid runtime evidence date") from error
    path_match = RUNTIME_EVIDENCE_PATH.fullmatch(str(binding[evidence_field]))
    if path_match is None or evidence_date != path_match.group("date"):
        raise ValueError(f"{evidence_path}: runtime evidence date does not match filename")
    if evidence.get("date_timezone") != EVIDENCE_TIMEZONE.key:
        raise ValueError(f"{evidence_path}: runtime evidence timezone must be Europe/Kyiv")
    if observed_date > datetime.now(EVIDENCE_TIMEZONE).date():
        raise ValueError(f"{evidence_path}: future-dated runtime evidence is forbidden")


def _operation_checks(
    evidence: dict[str, object],
    evidence_path: Path,
) -> dict[str, dict[str, object]]:
    checks = evidence.get("checks")
    if not isinstance(checks, list):
        raise ValueError(f"{evidence_path}: runtime checks are required")
    actual: dict[str, dict[str, object]] = {}
    for check in checks:
        if not isinstance(check, dict) or not isinstance(check.get("operation"), str):
            raise ValueError(f"{evidence_path}: every runtime check needs an operation")
        operation = check["operation"]
        if operation in actual:
            raise ValueError(f"{evidence_path}: duplicate runtime check: {operation}")
        actual[operation] = check
    return actual


def _validate_runtime_evidence(
    plugin_name: str,
    binding: dict[str, object],
    evidence_path: Path,
) -> None:
    evidence = _load_object(evidence_path, "runtime evidence")
    _validate_evidence_date(binding, "runtime_evidence", evidence, evidence_path)
    expected_binding = {
        "plugin": plugin_name,
        "app_id": binding["id"],
        "mcp_url": binding["mcp_url"],
    }
    if evidence.get("binding") != expected_binding:
        raise ValueError(f"{evidence_path}: binding identity does not match sidecar")
    if evidence.get("client") != "ChatGPT Developer Mode" or evidence.get(
        "evidence_type"
    ) != "interactive_direct_mcp_runtime":
        raise ValueError(f"{evidence_path}: expected direct ChatGPT Developer Mode evidence")
    source = evidence.get("source")
    if not isinstance(source, dict) or source.get("plugin") != plugin_name:
        raise ValueError(f"{evidence_path}: evidence plugin does not match sidecar")
    if source.get("delivery") != (
        "direct registered connection; repository package not installed"
    ):
        raise ValueError(f"{evidence_path}: evidence must keep package installation pending")
    checks = _operation_checks(evidence, evidence_path)
    actual_checks = {operation: check.get("status") for operation, check in checks.items()}
    if actual_checks != DIRECT_RUNTIME_CHECKS:
        raise ValueError(f"{evidence_path}: direct runtime checks do not match binding")


def _validate_personal_app_evidence(
    plugin_name: str,
    binding: dict[str, object],
    evidence_path: Path,
) -> None:
    evidence = _load_object(evidence_path, "personal app evidence")
    _validate_evidence_date(binding, "personal_app_evidence", evidence, evidence_path)
    expected_binding = {
        "plugin": plugin_name,
        "app_id": binding["id"],
        "mcp_url": binding["mcp_url"],
    }
    if evidence.get("binding") != expected_binding:
        raise ValueError(f"{evidence_path}: binding identity does not match sidecar")
    catalog = evidence.get("catalog")
    if (
        not isinstance(catalog, dict)
        or set(catalog) != {"revision", "digest"}
        or not re.fullmatch(r"[0-9a-f]{40}", str(catalog.get("revision", "")))
        or not re.fullmatch(r"sha256:[0-9a-f]{64}", str(catalog.get("digest", "")))
    ):
        raise ValueError(f"{evidence_path}: pinned catalog identity is required")
    if evidence.get("client") != "ChatGPT web Plugins UI" or evidence.get(
        "evidence_type"
    ) != "interactive_personal_app_runtime":
        raise ValueError(f"{evidence_path}: expected ChatGPT personal app UI evidence")
    source = evidence.get("source")
    if not isinstance(source, dict) or source.get("plugin") != plugin_name:
        raise ValueError(f"{evidence_path}: evidence plugin does not match sidecar")
    if source.get("delivery") != (
        "registered personal app; local .codex-plugin package ingestion not observed"
    ):
        raise ValueError(f"{evidence_path}: local package ingestion must remain unproved")
    if evidence.get("ui") != {
        "directory_source": "Personal",
        "display_name": "Universal Agent Plugins Cloudflare Docs E2E",
        "installation_state": "Installed",
        "detail_action": "Попробовать в чате",
        "opened_mode": "Chat",
        "plugin_chip_selected": True,
        "activation_evidence": "user_attested_manual",
    }:
        raise ValueError(f"{evidence_path}: Plugins UI observations do not match")
    if evidence.get("runtime") != {
        "prompt_count": 1,
        "tool_call_count": 2,
        "read_only": True,
    }:
        raise ValueError(f"{evidence_path}: runtime call counts do not match")
    if evidence.get("scope") != {
        "proved": [
            "registered_personal_app_installed_state",
            "plugins_ui_discovery",
            "chat_activation",
            "exact_app_id_linkage",
            "read_only_runtime",
        ],
        "not_proved": [
            "local_codex_plugin_package_ingestion",
            "repository_marketplace_install",
            "agentplugins_manager_lifecycle",
        ],
    }:
        raise ValueError(f"{evidence_path}: proof boundary does not match")
    checks = _operation_checks(evidence, evidence_path)
    actual_checks = {operation: check.get("status") for operation, check in checks.items()}
    if actual_checks != PERSONAL_APP_CHECKS:
        raise ValueError(f"{evidence_path}: personal app checks do not match binding")
    if checks["list_resources"].get("call_count") != 1:
        raise ValueError(f"{evidence_path}: list_resources must be called exactly once")
    search = checks["search_cloudflare_documentation"]
    if search.get("call_count") != 1 or search.get("query") != (
        "Durable Objects SQLite storage API"
    ):
        raise ValueError(f"{evidence_path}: documentation search evidence does not match")
    if checks["assistant_response_marker"].get("marker") != "E2E_OK Rules Of_":
        raise ValueError(f"{evidence_path}: sanitized response marker does not match")


def load_app_bindings(
    path: Path = APP_BINDINGS,
    root: Path = ROOT,
) -> dict[str, dict[str, object]]:
    """Return validated bindings, or no bindings when the sidecar is absent."""
    if not path.exists():
        return {}
    document = _load_object(path, "app bindings")
    if not isinstance(document, dict) or set(document) != {
        "$schema",
        "schema_version",
        "bindings",
    }:
        raise ValueError(f"{path}: only $schema, schema_version, and bindings are allowed")
    if document["$schema"] != "../../schemas/openai-app-bindings.schema.json":
        raise ValueError(f"{path}: unexpected $schema")
    if document["schema_version"] != 1:
        raise ValueError(f"{path}: schema_version must be 1")
    bindings = document["bindings"]
    if not isinstance(bindings, dict):
        raise ValueError(f"{path}: bindings must be an object")

    validated: dict[str, dict[str, object]] = {}
    seen_app_ids: set[str] = set()
    for plugin_name, raw in bindings.items():
        prefix = f"{path}: bindings.{plugin_name}"
        if not isinstance(plugin_name, str) or not PLUGIN_NAME.fullmatch(plugin_name):
            raise ValueError(f"{prefix}: invalid plugin name")
        if not isinstance(raw, dict) or set(raw) != ENTRY_FIELDS:
            raise ValueError(f"{prefix}: unexpected or missing fields")
        if raw["app_key"] != plugin_name or raw["mcp_server"] != plugin_name:
            raise ValueError(f"{prefix}: app_key and mcp_server must match the plugin name")
        app_id = raw["id"]
        if not isinstance(app_id, str) or not APP_ID_TOKEN.fullmatch(app_id):
            raise ValueError(f"{prefix}.id: invalid ChatGPT app ID token")
        if app_id in seen_app_ids:
            raise ValueError(f"{prefix}.id: duplicate ChatGPT app ID")
        seen_app_ids.add(app_id)
        _require_https_url(raw["mcp_url"], f"{prefix}.mcp_url")
        registration = raw["registration"]
        if not isinstance(registration, dict) or registration != {
            "surface": "chatgpt_developer_mode",
            "status": "development",
            "authentication": "none",
        }:
            raise ValueError(f"{prefix}.registration: unsupported registration metadata")
        evidence_path = _runtime_evidence_path(
            raw["runtime_evidence"], root, f"{prefix}.runtime_evidence"
        )
        _validate_evidence_identity(raw, "runtime_evidence", evidence_path, prefix)
        _validate_runtime_evidence(plugin_name, raw, evidence_path)
        personal_evidence_path = _runtime_evidence_path(
            raw["personal_app_evidence"], root, f"{prefix}.personal_app_evidence"
        )
        _validate_evidence_identity(
            raw, "personal_app_evidence", personal_evidence_path, prefix
        )
        _validate_personal_app_evidence(plugin_name, raw, personal_evidence_path)
        validated[plugin_name] = raw
    return validated


def validate_binding_target(
    plugin_name: str,
    binding: dict[str, object],
    portable_mcp: dict[str, object],
) -> None:
    """Bind only one exact public Streamable HTTP MCP endpoint."""
    servers = portable_mcp.get("mcpServers")
    server_name = str(binding["mcp_server"])
    if not isinstance(servers, dict) or set(servers) != {server_name}:
        raise ValueError(f"{plugin_name}: app binding requires one matching MCP server")
    server = servers[server_name]
    if not isinstance(server, dict) or server.get("type") != "streamable-http":
        raise ValueError(f"{plugin_name}: ChatGPT app binding requires Streamable HTTP")
    if server.get("url") != binding["mcp_url"]:
        raise ValueError(f"{plugin_name}: app binding endpoint does not match portable MCP")


def app_document(binding: dict[str, object]) -> dict[str, object]:
    """Return the official `.app.json` compatibility shape."""
    return {
        "apps": {
            str(binding["app_key"]): {
                "id": binding["id"],
            }
        }
    }
