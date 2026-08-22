from __future__ import annotations

import copy
import hashlib
import json
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from jsonschema import Draft202012Validator


ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "scripts"))

from openai_app_bindings import (  # noqa: E402
    app_document,
    load_app_bindings,
    validate_binding_target,
)
import build_openai_compat as builder  # noqa: E402
import build_registry as registry  # noqa: E402
from build_openai_compat import openai_manifest  # noqa: E402
from validate_openai_compat import ValidationError, validate_plugin  # noqa: E402


APP_ID = "plugin_asdk_app_6a78e90cf73481918ef10cdb87cd4bb4"
MCP_URL = "https://docs.mcp.cloudflare.com/mcp"
EVIDENCE_PATH = "tests/e2e/results/chatgpt-cloudflare-docs-direct-2026-08-10.json"
PERSONAL_APP_EVIDENCE_PATH = (
    "tests/e2e/results/chatgpt-cloudflare-docs-personal-app-2026-08-10.json"
)
CATALOG_REVISION = "fd77a74fa85724a57b328157ab82ef4dd991cda5"
CATALOG_DIGEST = "sha256:2293e95d41fd44daf2058696f985775847c2f4e779c8458e0e98f185e3864b0a"
PERSONAL_APP_EVIDENCE_REVISION = "2ddbb99dd190c1792b79904f9875e6322bccd243"


def valid_document() -> dict[str, object]:
    return {
        "$schema": "../../schemas/openai-app-bindings.schema.json",
        "schema_version": 1,
        "bindings": {
            "cloudflare-docs": {
                "app_key": "cloudflare-docs",
                "id": APP_ID,
                "mcp_server": "cloudflare-docs",
                "mcp_url": MCP_URL,
                "runtime_evidence": EVIDENCE_PATH,
                "runtime_evidence_revision": "fd77a74fa85724a57b328157ab82ef4dd991cda5",
                "runtime_evidence_digest": "sha256:050a18c56cf3f6b98d12ad35ac3c4642bd18d9e862956447dc3dad8e3189bcc5",
                "personal_app_evidence": PERSONAL_APP_EVIDENCE_PATH,
                "personal_app_evidence_revision": PERSONAL_APP_EVIDENCE_REVISION,
                "personal_app_evidence_digest": "sha256:97ddb41b887eebb7629bff1ae88937448b0c23073688122ab8939c3d96372b37",
                "registration": {
                    "surface": "chatgpt_developer_mode",
                    "status": "development",
                    "authentication": "none",
                },
            }
        },
    }


def valid_evidence() -> dict[str, object]:
    return {
        "client": "ChatGPT Developer Mode",
        "version": "rolling web release; build identifier not exposed",
        "date": "2026-08-10",
        "date_timezone": "Europe/Kyiv",
        "evidence_type": "interactive_direct_mcp_runtime",
        "binding": {
            "plugin": "cloudflare-docs",
            "app_id": APP_ID,
            "mcp_url": MCP_URL,
        },
        "source": {
            "plugin": "cloudflare-docs",
            "delivery": "direct registered connection; repository package not installed",
        },
        "checks": [
            {"scenario": "connect", "operation": "connect", "status": "passed"},
            {
                "scenario": "list resources",
                "operation": "list_resources",
                "status": "passed",
            },
            {
                "scenario": "search docs",
                "operation": "search_cloudflare_documentation",
                "status": "passed",
            },
            {
                "scenario": "install package",
                "operation": "package_ui_install",
                "status": "skipped",
            },
        ],
    }


def valid_personal_app_evidence() -> dict[str, object]:
    return {
        "client": "ChatGPT web Plugins UI",
        "version": "rolling web release; build identifier not exposed",
        "date": "2026-08-10",
        "date_timezone": "Europe/Kyiv",
        "evidence_type": "interactive_personal_app_runtime",
        "binding": {
            "plugin": "cloudflare-docs",
            "app_id": APP_ID,
            "mcp_url": MCP_URL,
        },
        "catalog": {"revision": CATALOG_REVISION, "digest": CATALOG_DIGEST},
        "source": {
            "plugin": "cloudflare-docs",
            "delivery": (
                "registered personal app; local .codex-plugin package ingestion "
                "not observed"
            ),
        },
        "ui": {
            "directory_source": "Personal",
            "display_name": "Universal Agent Plugins Cloudflare Docs E2E",
            "installation_state": "Installed",
            "detail_action": "Попробовать в чате",
            "opened_mode": "Chat",
            "plugin_chip_selected": True,
            "activation_evidence": "user_attested_manual",
        },
        "runtime": {"prompt_count": 1, "tool_call_count": 2, "read_only": True},
        "scope": {
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
        },
        "checks": [
            {
                "scenario": "installed",
                "operation": "plugins_personal_installed",
                "status": "passed",
            },
            {
                "scenario": "try in chat",
                "operation": "plugin_detail_try_in_chat",
                "status": "passed",
            },
            {
                "scenario": "chip selected",
                "operation": "new_chat_plugin_chip_selected",
                "status": "passed",
            },
            {
                "scenario": "list resources",
                "operation": "list_resources",
                "call_count": 1,
                "status": "passed",
            },
            {
                "scenario": "search docs",
                "operation": "search_cloudflare_documentation",
                "query": "Durable Objects SQLite storage API",
                "call_count": 1,
                "status": "passed",
            },
            {
                "scenario": "response marker",
                "operation": "assistant_response_marker",
                "marker": "E2E_OK Rules Of_",
                "status": "passed",
            },
            {
                "scenario": "local package ingestion",
                "operation": "local_codex_plugin_package_ingestion",
                "status": "skipped",
            },
            {
                "scenario": "manager lifecycle",
                "operation": "agentplugins_manager_lifecycle",
                "status": "skipped",
            },
        ],
    }


class OpenAIAppBindingTests(unittest.TestCase):
    def write_document(
        self,
        root: Path,
        document: object,
        evidence: object | None = None,
        evidence_path_value: str = EVIDENCE_PATH,
        personal_evidence: object | None = None,
    ) -> Path:
        evidence_path = root / evidence_path_value
        evidence_path.parent.mkdir(parents=True, exist_ok=True)
        evidence_path.write_text(json.dumps(valid_evidence() if evidence is None else evidence))
        personal_path = root / PERSONAL_APP_EVIDENCE_PATH
        personal_path.parent.mkdir(parents=True, exist_ok=True)
        personal_path.write_text(
            json.dumps(
                valid_personal_app_evidence()
                if personal_evidence is None
                else personal_evidence
            )
        )
        binding = (
            document.get("bindings", {}).get("cloudflare-docs", {})
            if isinstance(document, dict)
            else {}
        )
        if "runtime_evidence_digest" in binding:
            binding["runtime_evidence_digest"] = "sha256:" + hashlib.sha256(evidence_path.read_bytes()).hexdigest()
        if "personal_app_evidence_digest" in binding:
            binding["personal_app_evidence_digest"] = "sha256:" + hashlib.sha256(personal_path.read_bytes()).hexdigest()
        path = root / "app-bindings.json"
        path.write_text(json.dumps(document))
        return path

    def copy_generated_plugin(self, root: Path) -> Path:
        source = ROOT / "compat" / "openai" / "plugins" / "cloudflare-docs"
        target = root / "cloudflare-docs"
        shutil.copytree(source, target)
        return target

    def test_missing_sidecar_is_optional(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            self.assertEqual(load_app_bindings(Path(tmp) / "missing.json"), {})

    def test_valid_development_binding_loads(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            path = self.write_document(root, valid_document())

            bindings = load_app_bindings(path, root)

        self.assertEqual(bindings["cloudflare-docs"]["id"], APP_ID)
        self.assertEqual(
            app_document(bindings["cloudflare-docs"]),
            {"apps": {"cloudflare-docs": {"id": APP_ID}}},
        )

    def test_sidecar_accepts_opaque_safe_app_id_families(self) -> None:
        for app_id in ("connector_example123", "asdk_app_example123"):
            with self.subTest(app_id=app_id), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                document = valid_document()
                document["bindings"]["cloudflare-docs"]["id"] = app_id
                direct_evidence = valid_evidence()
                direct_evidence["binding"]["app_id"] = app_id
                personal_evidence = valid_personal_app_evidence()
                personal_evidence["binding"]["app_id"] = app_id
                path = self.write_document(
                    root,
                    document,
                    evidence=direct_evidence,
                    personal_evidence=personal_evidence,
                )

                bindings = load_app_bindings(path, root)

                self.assertEqual(bindings["cloudflare-docs"]["id"], app_id)

    def test_sidecar_rejects_unknown_or_unsafe_values(self) -> None:
        cases: dict[str, tuple[object, str]] = {}
        unknown = valid_document()
        unknown["unexpected"] = True
        cases["unknown top-level field"] = (unknown, "only \\$schema")
        invalid_id = valid_document()
        invalid_id["bindings"]["cloudflare-docs"]["id"] = "unsafe app id"
        cases["app ID whitespace"] = (invalid_id, "invalid ChatGPT app ID token")
        path_id = valid_document()
        path_id["bindings"]["cloudflare-docs"]["id"] = "../unsafe"
        cases["app ID path syntax"] = (path_id, "invalid ChatGPT app ID token")
        mismatched_name = valid_document()
        mismatched_name["bindings"]["cloudflare-docs"]["app_key"] = "other"
        cases["mismatched app key"] = (mismatched_name, "must match the plugin name")
        auth = valid_document()
        auth["bindings"]["cloudflare-docs"]["registration"]["authentication"] = "oauth"
        cases["unapproved auth"] = (auth, "unsupported registration metadata")
        query = valid_document()
        query["bindings"]["cloudflare-docs"]["mcp_url"] = MCP_URL + "?token=x"
        cases["endpoint query"] = (query, "without query or fragment")
        missing_revision = valid_document()
        del missing_revision["bindings"]["cloudflare-docs"][
            "personal_app_evidence_revision"
        ]
        cases["missing evidence revision"] = (
            missing_revision,
            "unexpected or missing fields",
        )
        short_revision = valid_document()
        short_revision["bindings"]["cloudflare-docs"][
            "personal_app_evidence_revision"
        ] = "2ddbb99"
        cases["short evidence revision"] = (
            short_revision,
            "expected a full commit SHA",
        )
        for name, (document, message) in cases.items():
            with self.subTest(name=name), tempfile.TemporaryDirectory() as tmp:
                path = self.write_document(Path(tmp), document)
                with self.assertRaisesRegex(ValueError, message):
                    load_app_bindings(path, Path(tmp))

    def test_sidecar_rejects_evidence_digest_mismatch(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            path = self.write_document(root, valid_document())
            document = json.loads(path.read_text())
            document["bindings"]["cloudflare-docs"]["runtime_evidence_digest"] = (
                "sha256:" + "0" * 64
            )
            path.write_text(json.dumps(document))
            with self.assertRaisesRegex(ValueError, "evidence digest does not match"):
                load_app_bindings(path, root)

    def test_evidence_schema_accepts_opaque_safe_app_ids(self) -> None:
        schema = json.loads(
            (ROOT / "schemas" / "e2e" / "client-evidence.schema.json").read_text()
        )
        evidence = json.loads(
            (
                ROOT
                / "tests"
                / "e2e"
                / "results"
                / "chatgpt-cloudflare-docs-personal-app-2026-08-10.json"
            ).read_text()
        )
        validator = Draft202012Validator(schema)
        for app_id in ("connector_example123", "asdk_app_example123"):
            document = copy.deepcopy(evidence)
            document["binding"]["app_id"] = app_id
            with self.subTest(app_id=app_id):
                self.assertEqual(list(validator.iter_errors(document)), [])
        evidence["binding"]["app_id"] = "unsafe app id"
        self.assertNotEqual(list(validator.iter_errors(evidence)), [])

    def test_committed_sidecar_evidence_is_revision_and_digest_bound(self) -> None:
        document = json.loads((ROOT / "compat/openai/app-bindings.json").read_text())
        binding = document["bindings"]["cloudflare-docs"]
        for field in ("runtime_evidence", "personal_app_evidence"):
            revision = binding[f"{field}_revision"]
            pinned = subprocess.check_output(
                ["git", "show", f"{revision}:{binding[field]}"], cwd=ROOT
            )
            self.assertEqual(
                binding[f"{field}_digest"],
                "sha256:" + hashlib.sha256(pinned).hexdigest(),
            )
            self.assertEqual(pinned, (ROOT / binding[field]).read_bytes())

    def test_sidecar_rejects_duplicate_app_id(self) -> None:
        document = valid_document()
        duplicate = copy.deepcopy(document["bindings"]["cloudflare-docs"])
        duplicate["app_key"] = "other"
        duplicate["mcp_server"] = "other"
        document["bindings"]["other"] = duplicate

        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            path = self.write_document(root, document)
            with self.assertRaisesRegex(ValueError, "duplicate ChatGPT app ID"):
                load_app_bindings(path, root)

    def test_sidecar_rejects_runtime_evidence_drift(self) -> None:
        cases: dict[str, tuple[dict[str, object], str]] = {}
        wrong_id = valid_evidence()
        wrong_id["binding"]["app_id"] = "plugin_asdk_app_different"
        cases["wrong app ID"] = (wrong_id, "binding identity does not match sidecar")
        wrong_endpoint = valid_evidence()
        wrong_endpoint["binding"]["mcp_url"] = "https://example.test/mcp"
        cases["wrong endpoint"] = (
            wrong_endpoint,
            "binding identity does not match sidecar",
        )
        failed_runtime = valid_evidence()
        failed_runtime["checks"][2]["status"] = "failed"
        cases["failed direct check"] = (
            failed_runtime,
            "direct runtime checks do not match binding",
        )

        for name, (evidence, message) in cases.items():
            with self.subTest(name=name), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                path = self.write_document(root, valid_document(), evidence)
                with self.assertRaisesRegex(ValueError, message):
                    load_app_bindings(path, root)

    def test_sidecar_rejects_future_dated_runtime_evidence(self) -> None:
        future_path = (
            "tests/e2e/results/chatgpt-cloudflare-docs-direct-2999-01-01.json"
        )
        document = valid_document()
        document["bindings"]["cloudflare-docs"]["runtime_evidence"] = future_path
        evidence = valid_evidence()
        evidence["date"] = "2999-01-01"

        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            path = self.write_document(root, document, evidence, future_path)
            with self.assertRaisesRegex(ValueError, "future-dated runtime evidence"):
                load_app_bindings(path, root)

    def test_sidecar_rejects_personal_app_evidence_drift(self) -> None:
        cases: dict[str, tuple[dict[str, object], str]] = {}
        wrong_id = valid_personal_app_evidence()
        wrong_id["binding"]["app_id"] = "plugin_asdk_app_different"
        cases["wrong app ID"] = (wrong_id, "binding identity does not match sidecar")
        wrong_ui = valid_personal_app_evidence()
        wrong_ui["ui"]["installation_state"] = "Available"
        cases["not installed"] = (wrong_ui, "Plugins UI observations do not match")
        wrong_count = valid_personal_app_evidence()
        wrong_count["runtime"]["tool_call_count"] = 3
        cases["wrong tool count"] = (wrong_count, "runtime call counts do not match")
        wrong_query = valid_personal_app_evidence()
        wrong_query["checks"][4]["query"] = "different query"
        cases["wrong query"] = (
            wrong_query,
            "documentation search evidence does not match",
        )
        false_ingestion = valid_personal_app_evidence()
        false_ingestion["checks"][6]["status"] = "passed"
        cases["false package ingestion"] = (
            false_ingestion,
            "personal app checks do not match binding",
        )

        for name, (personal_evidence, message) in cases.items():
            with self.subTest(name=name), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                path = self.write_document(
                    root,
                    valid_document(),
                    personal_evidence=personal_evidence,
                )
                with self.assertRaisesRegex(ValueError, message):
                    load_app_bindings(path, root)

    def test_binding_requires_exact_single_streamable_http_server(self) -> None:
        binding = valid_document()["bindings"]["cloudflare-docs"]
        valid_mcp = {
            "mcpServers": {
                "cloudflare-docs": {
                    "type": "streamable-http",
                    "url": MCP_URL,
                }
            }
        }
        validate_binding_target("cloudflare-docs", binding, valid_mcp)

        for name, document in {
            "endpoint mismatch": {
                "mcpServers": {
                    "cloudflare-docs": {
                        "type": "streamable-http",
                        "url": "https://example.test/mcp",
                    }
                }
            },
            "stdio": {
                "mcpServers": {
                    "cloudflare-docs": {"type": "stdio", "command": "demo"}
                }
            },
            "extra server": {
                "mcpServers": {
                    **valid_mcp["mcpServers"],
                    "other": {"type": "streamable-http", "url": MCP_URL},
                }
            },
        }.items():
            with self.subTest(name=name), self.assertRaises(ValueError):
                validate_binding_target("cloudflare-docs", binding, document)

    def test_openai_manifest_declares_apps_only_for_bound_package(self) -> None:
        portable = {
            "name": "cloudflare-docs",
            "version": "0.1.0",
            "description": "Cloudflare documentation",
            "author": {"name": "777genius"},
            "homepage": "https://example.test",
            "repository": "https://example.test/repository",
            "license": "Apache-2.0",
            "keywords": ["docs"],
        }

        unbound = openai_manifest(portable, False, True, False)
        bound = openai_manifest(portable, False, True, True)

        self.assertNotIn("apps", unbound)
        self.assertEqual(bound["apps"], "./.app.json")

    def test_unbound_generated_plugin_rejects_rogue_app_file(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            plugin = Path(tmp) / "demo"
            (plugin / ".codex-plugin").mkdir(parents=True)
            (plugin / "assets").mkdir()
            for name in ("icon.png", "logo.png"):
                (plugin / "assets" / name).write_bytes(b"asset")
            manifest = {
                "name": "demo",
                "version": "0.1.0",
                "description": "Demo plugin",
                "license": "Apache-2.0",
                "interface": {
                    "capabilities": ["Read"],
                    "composerIcon": "./assets/icon.png",
                    "logo": "./assets/logo.png",
                    "logoDark": "./assets/logo.png",
                },
            }
            (plugin / ".codex-plugin" / "plugin.json").write_text(
                json.dumps(manifest)
            )
            (plugin / ".app.json").write_text(
                json.dumps({"apps": {"demo": {"id": APP_ID}}})
            )

            with self.assertRaisesRegex(ValidationError, "rogue .app.json"):
                validate_plugin(plugin, {})

    def test_bound_generated_plugin_rejects_wrong_app_id(self) -> None:
        bindings = load_app_bindings()
        with tempfile.TemporaryDirectory() as tmp:
            plugin = self.copy_generated_plugin(Path(tmp))
            app_path = plugin / ".app.json"
            app = json.loads(app_path.read_text())
            app["apps"]["cloudflare-docs"]["id"] = "plugin_asdk_app_different"
            app_path.write_text(json.dumps(app))

            with self.assertRaisesRegex(ValidationError, "app binding drift"):
                validate_plugin(plugin, bindings)

    def test_bound_generated_plugin_rejects_missing_app_file(self) -> None:
        bindings = load_app_bindings()
        with tempfile.TemporaryDirectory() as tmp:
            plugin = self.copy_generated_plugin(Path(tmp))
            (plugin / ".app.json").unlink()

            with self.assertRaisesRegex(ValidationError, "invalid JSON"):
                validate_plugin(plugin, bindings)

    def test_bound_generated_plugin_rejects_missing_manifest_apps(self) -> None:
        bindings = load_app_bindings()
        with tempfile.TemporaryDirectory() as tmp:
            plugin = self.copy_generated_plugin(Path(tmp))
            manifest_path = plugin / ".codex-plugin" / "plugin.json"
            manifest = json.loads(manifest_path.read_text())
            del manifest["apps"]
            manifest_path.write_text(json.dumps(manifest))

            with self.assertRaisesRegex(ValidationError, "invalid apps path"):
                validate_plugin(plugin, bindings)

    def test_bound_generated_plugin_rejects_mcp_endpoint_drift(self) -> None:
        bindings = load_app_bindings()
        with tempfile.TemporaryDirectory() as tmp:
            plugin = self.copy_generated_plugin(Path(tmp))
            mcp_path = plugin / ".mcp.json"
            mcp = json.loads(mcp_path.read_text())
            mcp["mcpServers"]["cloudflare-docs"]["url"] = "https://example.test/mcp"
            mcp_path.write_text(json.dumps(mcp))

            with self.assertRaisesRegex(ValidationError, "app endpoint drift"):
                validate_plugin(plugin, bindings)

    def test_builder_rejects_unknown_plugin_binding(self) -> None:
        binding = valid_document()["bindings"]["cloudflare-docs"]
        with tempfile.TemporaryDirectory() as tmp, patch.object(
            builder,
            "load_app_bindings",
            return_value={"unknown-plugin": binding},
        ):
            root = Path(tmp)
            with self.assertRaisesRegex(ValueError, "unknown plugins"):
                builder.build(root / "plugins", root / "marketplace.json")

    def test_marketplace_packages_exactly_follow_codex_target_eligibility(self) -> None:
        source = registry.load_directory_source()
        expected = {
            product["id"] for product in source["products"]
            if self.resolves(source, product["id"], "codex")
        }
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            plugins = root / "plugins"
            marketplace = root / "marketplace.json"
            builder.build(plugins, marketplace)
            generated = {path.name for path in plugins.iterdir() if path.is_dir()}
            listed = {
                entry["name"]
                for entry in json.loads(marketplace.read_text())["plugins"]
                if entry["policy"]["installation"] == "AVAILABLE"
            }

        self.assertEqual(generated, expected)
        self.assertEqual(listed, expected)
        self.assertTrue({"chrome-devtools", "context7", "firebase", "hubspot-developer"}.isdisjoint(expected))

    @staticmethod
    def resolves(source: dict[str, object], product: str, target: str) -> bool:
        try:
            registry.resolve_directory(source, product, [target])
        except registry.RegistryError:
            return False
        return True

    def generated_names(self, source: dict[str, object]) -> set[str]:
        with tempfile.TemporaryDirectory() as tmp, patch.object(
            registry, "load_directory_source", return_value=source
        ):
            root = Path(tmp)
            plugins = root / "plugins"
            builder.build(plugins, root / "marketplace.json")
            return {
                path.name for path in plugins.iterdir() if path.is_dir()
            } if plugins.exists() else set()

    def test_upstream_fallback_cannot_substitute_suspended_local_bytes(self) -> None:
        source = copy.deepcopy(registry.load_directory_source())
        product = next(item for item in source["products"] if item["id"] == "atlassian")
        local = next(
            item for item in source["distributions"]
            if item["id"] == product["default_distribution"]
        )
        local["status"] = "suspended"
        upstream = copy.deepcopy(local)
        upstream.update({
            "id": "atlassian/atlassian",
            "kind": "upstream",
            "status": "active",
            "packager": "atlassian",
        })
        release = upstream["releases"][0]
        release["package_source"] = {
            "repository": "atlassian/atlassian",
            "revision": "a" * 40,
            "path": "agent-plugin",
        }
        policy = upstream["release_policies"][0]
        policy["targets"] = [
            target for target in policy["targets"] if target["client"] == "codex"
        ]
        evidence_id = "atlassian/atlassian/materialization-codex"
        policy["current_evidence"] = [evidence_id]
        source["evidence"].append({
            "id": evidence_id,
            "distribution_id": upstream["id"],
            "release_sequence": release["sequence"],
            "package_tree_digest": release["tree_digest"],
            "level": "materialization",
            "outcome": "passed",
            "client": "codex",
        })
        product["distributions"].append(upstream["id"])
        product["distributions"].sort()
        source["distributions"].append(upstream)
        source["distributions"].sort(key=lambda item: item["id"])

        selection = registry.resolve_directory(source, "atlassian", ["codex"])
        self.assertEqual(selection["distribution_id"], upstream["id"])
        self.assertNotIn("atlassian", self.generated_names(source))

    def test_non_openai_only_eligibility_cannot_create_marketplace_entry(self) -> None:
        source = copy.deepcopy(registry.load_directory_source())
        distribution = next(
            item for item in source["distributions"]
            if item["id"] == "777genius/atlassian"
        )
        distribution["release_policies"][0]["targets"] = [
            target for target in distribution["release_policies"][0]["targets"]
            if target["client"] == "cursor"
        ]

        self.assertTrue(self.resolves(source, "atlassian", "cursor"))
        self.assertFalse(self.resolves(source, "atlassian", "codex"))
        self.assertNotIn("atlassian", self.generated_names(source))

    def test_exact_local_codex_selected_release_still_generates(self) -> None:
        source = registry.load_directory_source()
        selection = registry.resolve_directory(source, "cloudflare-docs", ["codex"])
        _, release = builder.selected_release(source, selection)

        self.assertEqual(release["package_source"]["repository"], builder.LOCAL_REPOSITORY)
        self.assertIsNone(release["package_source"]["revision"])
        self.assertIn("cloudflare-docs", self.generated_names(source))

    def test_bound_historical_selection_uses_its_exact_git_bytes(self) -> None:
        source = registry.load_directory_source()
        selection = registry.resolve_directory(source, "atlassian", ["codex"])
        _, release = builder.selected_release(source, selection)
        package_source = release["package_source"]
        expected = subprocess.check_output(
            [
                "git",
                "show",
                f"{package_source['revision']}:{package_source['path']}/README.md",
            ],
            cwd=ROOT,
        )
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            builder.build(root / "plugins", root / "marketplace.json")
            generated = (root / "plugins" / "atlassian" / "README.md").read_bytes()

        self.assertEqual(generated, expected)
        self.assertNotEqual(generated, (ROOT / package_source["path"] / "README.md").read_bytes())

    def test_chatgpt_unresolvable_while_codex_eligible_fails_generation(self) -> None:
        source = copy.deepcopy(registry.load_directory_source())
        for distribution in source["distributions"]:
            if distribution["product_id"] != "cloudflare-docs":
                continue
            for policy in distribution["release_policies"]:
                policy["targets"] = [
                    target for target in policy["targets"]
                    if target["client"] != "chatgpt"
                ]

        self.assertTrue(self.resolves(source, "cloudflare-docs", "codex"))
        self.assertFalse(self.resolves(source, "cloudflare-docs", "chatgpt"))
        with self.assertRaisesRegex(
            ValueError,
            "cloudflare-docs: configured app sidecar.*ChatGPT target cannot resolve.*"
            "update or remove.*app-bindings.json",
        ):
            self.generated_names(source)

    def test_differing_codex_and_chatgpt_immutable_selections_fail_generation(self) -> None:
        source = copy.deepcopy(registry.load_directory_source())
        bridge = next(
            item for item in source["distributions"]
            if item["id"] == "777genius/cloudflare-docs-bridge"
        )
        bridge["release_policies"][0]["targets"] = [
            target for target in bridge["release_policies"][0]["targets"]
            if target["client"] != "chatgpt"
        ]
        codex = registry.resolve_directory(source, "cloudflare-docs", ["codex"])
        chatgpt = registry.resolve_directory(source, "cloudflare-docs", ["chatgpt"])

        self.assertNotEqual(
            builder.selection_identity(codex), builder.selection_identity(chatgpt)
        )
        with self.assertRaisesRegex(
            ValueError,
            "cloudflare-docs: configured app sidecar.*ChatGPT selects.*Codex selects.*"
            "update or remove.*app-bindings.json",
        ):
            self.generated_names(source)

    def test_mismatched_signed_chatgpt_app_binding_fails_generation(self) -> None:
        source = copy.deepcopy(registry.load_directory_source())
        selection = registry.resolve_directory(source, "cloudflare-docs", ["chatgpt"])
        distribution, release = builder.selected_release(source, selection)
        target = builder.selected_target(distribution, release, "chatgpt")
        self.assertIsNotNone(target)
        target["app_binding"]["id"] = "plugin_asdk_app_different"

        with self.assertRaisesRegex(
            ValueError,
            "cloudflare-docs: configured app sidecar.*signed ChatGPT target "
            "app_binding.*does not equal sidecar binding.*update or remove.*"
            "app-bindings.json",
        ):
            self.generated_names(source)

    def test_unavailable_external_codex_selection_still_validates_sidecar(self) -> None:
        source = copy.deepcopy(registry.load_directory_source())
        selection = registry.resolve_directory(source, "cloudflare-docs", ["codex"])
        distribution, release = builder.selected_release(source, selection)
        release["package_source"] = {
            "repository": "external/cloudflare-docs",
            "revision": "a" * 40,
            "path": "agent-plugin",
        }
        target = builder.selected_target(distribution, release, "chatgpt")
        self.assertIsNotNone(target)
        target["app_binding"]["id"] = "plugin_asdk_app_different"

        with tempfile.TemporaryDirectory() as tmp:
            self.assertIsNone(
                builder.exact_selected_package(source, selection, Path(tmp))
            )
        with self.assertRaisesRegex(
            ValueError,
            "cloudflare-docs: configured app sidecar.*signed ChatGPT target "
            "app_binding.*does not equal sidecar binding.*update or remove.*"
            "app-bindings.json",
        ):
            self.generated_names(source)

    def test_valid_same_selection_emits_codex_package_and_app_document(self) -> None:
        source = registry.load_directory_source()
        codex = registry.resolve_directory(source, "cloudflare-docs", ["codex"])
        chatgpt = registry.resolve_directory(source, "cloudflare-docs", ["chatgpt"])
        self.assertEqual(
            builder.selection_identity(codex), builder.selection_identity(chatgpt)
        )

        with tempfile.TemporaryDirectory() as tmp, patch.object(
            registry, "load_directory_source", return_value=source
        ):
            root = Path(tmp)
            plugins = root / "plugins"
            builder.build(plugins, root / "marketplace.json")
            package = plugins / "cloudflare-docs"
            manifest = json.loads(
                (package / ".codex-plugin" / "plugin.json").read_text()
            )
            app = json.loads((package / ".app.json").read_text())

        self.assertEqual(manifest["apps"], "./.app.json")
        self.assertEqual(app, {"apps": {"cloudflare-docs": {"id": APP_ID}}})

    def test_codex_only_package_without_sidecar_still_emits_normally(self) -> None:
        source = registry.load_directory_source()
        self.assertTrue(self.resolves(source, "github", "codex"))
        self.assertFalse(self.resolves(source, "github", "chatgpt"))

        with tempfile.TemporaryDirectory() as tmp, patch.object(
            registry, "load_directory_source", return_value=source
        ):
            root = Path(tmp)
            plugins = root / "plugins"
            builder.build(plugins, root / "marketplace.json")
            package = plugins / "github"
            manifest = json.loads(
                (package / ".codex-plugin" / "plugin.json").read_text()
            )
            package_exists = package.is_dir()
            app_exists = (package / ".app.json").exists()

        self.assertTrue(package_exists)
        self.assertNotIn("apps", manifest)
        self.assertFalse(app_exists)


if __name__ == "__main__":
    unittest.main()
