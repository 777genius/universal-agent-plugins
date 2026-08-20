from __future__ import annotations

import copy
import hashlib
import importlib.util
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest import mock

from jsonschema import Draft202012Validator


ROOT = Path(__file__).resolve().parents[1]
SCRIPTS = ROOT / "scripts"
sys.path.insert(0, str(SCRIPTS))
SPEC = importlib.util.spec_from_file_location("build_agentplugins_catalog", SCRIPTS / "build_agentplugins_catalog.py")
assert SPEC and SPEC.loader
builder = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(builder)


class AgentpluginsCatalogBuilderTests(unittest.TestCase):
    def test_committed_legacy_catalogs_are_byte_frozen(self) -> None:
        expected = {
            1: "9ed64038a8a1b1eab6956008f94b3ffa16f1b6ddf01e8b2809b202656423f183",
            2: "66199c87bd68c65e39d15aa2c5c6e6c7830c9b116d8ed3590123031b32357050",
        }
        for schema_version, digest in expected.items():
            with self.subTest(schema_version=schema_version):
                body = (ROOT / "catalog" / f"v{schema_version}" / "catalog.json").read_bytes()
                self.assertEqual(hashlib.sha256(body).hexdigest(), digest)
                current = json.loads(body)
                self.assertEqual(len(current["plugins"]), 26)

    def test_released_catalog_v1_contract_is_byte_for_byte_unchanged(self) -> None:
        self.assertEqual(
            hashlib.sha256((ROOT / "catalog/v1/catalog.json").read_bytes()).hexdigest(),
            "9ed64038a8a1b1eab6956008f94b3ffa16f1b6ddf01e8b2809b202656423f183",
        )
        self.assertEqual(
            hashlib.sha256((ROOT / "schemas/catalog-v1.schema.json").read_bytes()).hexdigest(),
            "e734974864228f07330ecbbb85e1ed50cf4c20ec23cdb16e6bbf9a24183f1b8f",
        )

    def test_tree_digest_matches_engine_header_contract(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "bin").mkdir()
            executable = root / "bin" / "run"
            executable.write_bytes(b"run\n")
            executable.chmod(0o755)
            (root / "plugin.json").write_bytes(b"{}\n")
            digest = hashlib.sha256()
            digest.update(b"dir\0bin\0false\0" + b"0\0")
            digest.update(b"file\0bin/run\0true\0" + b"4\0run\n")
            digest.update(b"file\0plugin.json\0false\0" + b"3\0{}\n")
            self.assertEqual(builder.package_tree_digest(root), "sha256:" + digest.hexdigest())

    def test_same_version_catalog_entries_are_content_pinned(self) -> None:
        current = json.loads((ROOT / "catalog" / "v2" / "catalog.json").read_text())
        base_clients = {"codex", "cursor", "copilot", "vscode", "kiro"}
        for plugin in current["plugins"]:
            self.assertRegex(plugin["tree_digest"], r"^sha256:[0-9a-f]{64}$")
            self.assertRegex(plugin["manifest_digest"], r"^sha256:[0-9a-f]{64}$")
            self.assertEqual(plugin["minimum_cli_version"], "0.1.6")
            expected = base_clients | ({"chatgpt"} if plugin["name"] == "cloudflare-docs" else set())
            self.assertEqual(set(plugin["compatibility"]), expected)

    def test_runtime_tested_claims_match_pinned_hero_evidence(self) -> None:
        current = json.loads((ROOT / "catalog" / "v2" / "catalog.json").read_text())
        evidence = json.loads(
            (ROOT / "tests" / "e2e" / "results" / "agentplugins-hero-runtime-matrix-2026-08-08.json").read_text()
        )
        equivalence = evidence["source"]["runtime_equivalence"]
        catalog_history = subprocess.check_output(
            ["git", "rev-list", current["revision"], "--", "catalog/v1/catalog.json"],
            cwd=ROOT,
            text=True,
        ).splitlines()
        historical_digests = {
            "sha256:"
            + hashlib.sha256(
                subprocess.check_output(
                    ["git", "show", f"{commit}:catalog/v1/catalog.json"],
                    cwd=ROOT,
                )
            ).hexdigest()
            for commit in catalog_history
        }
        self.assertIn(equivalence["catalog_digest"], historical_digests)
        self.assertEqual(equivalence["allowed_delta"], "plugins/*/README.md")

        evidence_revisions = {
            check.get("source_commit", evidence["source"]["commit_sha"])
            for check in evidence["checks"]
            if check["status"] == "passed"
        }
        for revision in evidence_revisions:
            ancestor = subprocess.run(
                ["git", "merge-base", "--is-ancestor", revision, current["revision"]],
                cwd=ROOT,
                check=False,
            )
            self.assertEqual(ancestor.returncode, 0, "runtime evidence commit is not an ancestor of the catalog revision")
            comparison = subprocess.run(
                [
                    "git",
                    "diff",
                    "--quiet",
                    revision,
                    current["revision"],
                    "--",
                    "plugins",
                    ":(glob,exclude)plugins/*/README.md",
                ],
                cwd=ROOT,
                check=False,
            )
            self.assertEqual(
                comparison.returncode,
                0,
                "runtime-bearing package files differ from the tested evidence commit",
            )
        client_ids = {"Codex CLI": "codex", "Cursor Agent": "cursor", "Kiro CLI": "kiro"}
        evidenced = {
            (check["plugin"], client_ids[check["client"]])
            for check in evidence["checks"]
            if check["status"] == "passed" and check["client"] in client_ids
        }
        chatgpt_evidence = json.loads(
            (
                ROOT
                / "tests"
                / "e2e"
                / "results"
                / "chatgpt-cloudflare-docs-personal-app-2026-08-10.json"
            ).read_text()
        )
        cloudflare_docs = next(
            plugin for plugin in current["plugins"] if plugin["name"] == "cloudflare-docs"
        )
        app_binding = cloudflare_docs["compatibility"]["chatgpt"]["app_binding"]
        evidence_revision = app_binding["runtime_evidence_revision"]
        pinned_evidence = subprocess.check_output(
            ["git", "show", f"{evidence_revision}:{app_binding['runtime_evidence']}"],
            cwd=ROOT,
        )
        self.assertEqual(
            pinned_evidence,
            (ROOT / app_binding["runtime_evidence"]).read_bytes(),
        )
        historical_catalog = subprocess.check_output(
            ["git", "show", f"{evidence_revision}:catalog/v1/catalog.json"],
            cwd=ROOT,
        )
        historical_catalog_document = json.loads(historical_catalog)
        self.assertEqual(
            chatgpt_evidence["catalog"],
            {
                "revision": historical_catalog_document["revision"],
                "digest": "sha256:" + hashlib.sha256(historical_catalog).hexdigest(),
            },
        )
        self.assertIn("local_codex_plugin_package_ingestion", chatgpt_evidence["scope"]["not_proved"])
        self.assertIn("agentplugins_manager_lifecycle", chatgpt_evidence["scope"]["not_proved"])
        evidenced.add((chatgpt_evidence["binding"]["plugin"], "chatgpt"))
        claimed = {
            (plugin["name"], client)
            for plugin in current["plugins"]
            for client, status in plugin["compatibility"].items()
            if status["verification"] == "tested"
        }
        self.assertEqual(claimed, evidenced)

    def test_chatgpt_compatibility_requires_validated_app_evidence(self) -> None:
        current = json.loads((ROOT / "catalog" / "v2" / "catalog.json").read_text())
        with mock.patch.object(builder, "load_app_bindings", return_value={}):
            rebuilt = builder.build(current["revision"], current["published_at"], 2)
        cloudflare_docs = next(
            plugin for plugin in rebuilt["plugins"] if plugin["name"] == "cloudflare-docs"
        )
        self.assertNotIn("chatgpt", cloudflare_docs["compatibility"])

        committed = next(
            plugin for plugin in current["plugins"] if plugin["name"] == "cloudflare-docs"
        )
        self.assertEqual(
            committed["compatibility"]["chatgpt"],
            {
                "package": "projected",
                "verification": "tested",
                "authentication": "not_required",
                "app_binding": {
                    "app_key": "cloudflare-docs",
                    "id": "plugin_asdk_app_6a78e90cf73481918ef10cdb87cd4bb4",
                    "mcp_server": "cloudflare-docs",
                    "mcp_url": "https://docs.mcp.cloudflare.com/mcp",
                    "runtime_evidence": (
                        "tests/e2e/results/"
                        "chatgpt-cloudflare-docs-personal-app-2026-08-10.json"
                    ),
                    "runtime_evidence_revision": (
                        "2ddbb99dd190c1792b79904f9875e6322bccd243"
                    ),
                },
            },
        )

        invalid_binding = {
            "cloudflare-docs": {
                "registration": {"authentication": "oauth"},
            }
        }
        with self.assertRaisesRegex(ValueError, "explicit auth evidence"):
            builder.compatibility("cloudflare-docs", invalid_binding, 2)

    def test_chatgpt_app_binding_schema_fails_closed(self) -> None:
        schema = json.loads((ROOT / "schemas" / "catalog-v2.schema.json").read_text())
        catalog = json.loads((ROOT / "catalog" / "v2" / "catalog.json").read_text())
        validator = Draft202012Validator(schema)
        self.assertEqual(list(validator.iter_errors(catalog)), [])
        cloudflare_index = next(
            index
            for index, plugin in enumerate(catalog["plugins"])
            if plugin["name"] == "cloudflare-docs"
        )

        def mutated_chatgpt() -> tuple[dict[str, object], dict[str, object]]:
            document = copy.deepcopy(catalog)
            chatgpt = document["plugins"][cloudflare_index]["compatibility"]["chatgpt"]
            return document, chatgpt

        for app_id in ("connector_example123", "asdk_app_example123"):
            document, chatgpt = mutated_chatgpt()
            chatgpt["app_binding"]["id"] = app_id
            with self.subTest(valid_app_id=app_id):
                self.assertEqual(list(validator.iter_errors(document)), [])

        cases: dict[str, dict[str, object]] = {}
        document, chatgpt = mutated_chatgpt()
        del chatgpt["app_binding"]["id"]
        cases["missing ID"] = document
        document, _ = mutated_chatgpt()
        document["plugins"][cloudflare_index]["minimum_cli_version"] = "0.1.5"
        cases["pre-v2 CLI version"] = document
        document, chatgpt = mutated_chatgpt()
        chatgpt["package"] = "native"
        cases["non-projected package"] = document
        document, chatgpt = mutated_chatgpt()
        chatgpt["app_binding"]["mcp_url"] = "https://user@example.com/mcp"
        cases["URL userinfo"] = document
        document, chatgpt = mutated_chatgpt()
        chatgpt["app_binding"]["mcp_url"] = "https://example.com/mcp?token=secret"
        cases["URL query"] = document
        document, chatgpt = mutated_chatgpt()
        chatgpt["app_binding"]["mcp_url"] = "https://example.com/mcp#fragment"
        cases["URL fragment"] = document
        document, chatgpt = mutated_chatgpt()
        chatgpt["app_binding"]["runtime_evidence"] = "../outside.json"
        cases["unsafe evidence path"] = document
        document, chatgpt = mutated_chatgpt()
        del chatgpt["app_binding"]["runtime_evidence_revision"]
        cases["missing evidence revision"] = document
        document, chatgpt = mutated_chatgpt()
        chatgpt["app_binding"]["runtime_evidence_revision"] = "71cc947"
        cases["short evidence revision"] = document
        document, chatgpt = mutated_chatgpt()
        chatgpt["app_binding"]["unexpected"] = "value"
        cases["unknown binding field"] = document
        document, chatgpt = mutated_chatgpt()
        chatgpt["app_binding"]["id"] = "unsafe app id"
        cases["app ID whitespace"] = document
        document = copy.deepcopy(catalog)
        codex = document["plugins"][cloudflare_index]["compatibility"]["codex"]
        codex["app_binding"] = copy.deepcopy(
            document["plugins"][cloudflare_index]["compatibility"]["chatgpt"]["app_binding"]
        )
        cases["binding on non-ChatGPT client"] = document

        for name, document in cases.items():
            with self.subTest(name=name):
                self.assertNotEqual(list(validator.iter_errors(document)), [])

    def test_chatgpt_runtime_evidence_must_match_catalog_identity(self) -> None:
        catalog = json.loads((ROOT / "catalog" / "v2" / "catalog.json").read_text())
        builder.validate_chatgpt_catalog_evidence(catalog)
        drifted = copy.deepcopy(catalog)
        drifted["revision"] = "0" * 40
        with self.assertRaisesRegex(ValueError, "portable packages differ"):
            builder.validate_chatgpt_catalog_evidence(drifted)

    def test_pinned_runtime_evidence_rejects_missing_or_changed_git_blob(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            subprocess.run(["git", "init", "-q"], cwd=root, check=True)
            evidence_relative = Path("tests/e2e/results/evidence.json")
            evidence_path = root / evidence_relative
            evidence_path.parent.mkdir(parents=True)
            evidence_path.write_bytes(b"pinned evidence\n")
            subprocess.run(["git", "add", evidence_relative.as_posix()], cwd=root, check=True)
            subprocess.run(
                [
                    "git",
                    "-c",
                    "user.name=Catalog Test",
                    "-c",
                    "user.email=catalog@example.test",
                    "commit",
                    "-qm",
                    "test evidence",
                ],
                cwd=root,
                check=True,
            )
            revision = subprocess.check_output(
                ["git", "rev-parse", "HEAD"], cwd=root, text=True
            ).strip()
            binding = {
                "personal_app_evidence": evidence_relative.as_posix(),
                "personal_app_evidence_revision": revision,
            }
            self.assertEqual(
                builder.validate_pinned_runtime_evidence(binding, root),
                b"pinned evidence\n",
            )

            evidence_path.write_bytes(b"changed evidence\n")
            with self.assertRaisesRegex(ValueError, "differs from its pinned revision"):
                builder.validate_pinned_runtime_evidence(binding, root)

            evidence_path.write_bytes(b"pinned evidence\n")
            binding["personal_app_evidence_revision"] = "0" * 40
            with self.assertRaisesRegex(ValueError, "not an exact local commit"):
                builder.validate_pinned_runtime_evidence(binding, root)

            binding["personal_app_evidence_revision"] = revision
            binding["personal_app_evidence"] = "tests/e2e/results/missing.json"
            with self.assertRaisesRegex(ValueError, "missing at the pinned revision"):
                builder.validate_pinned_runtime_evidence(binding, root)

    def test_manifest_name_must_match_hashed_directory(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            plugins = Path(tmp)
            package = plugins / "actual-directory"
            package.mkdir()
            (package / "plugin.json").write_text(
                json.dumps(
                    {
                        "$schema": builder.PLUGIN_SCHEMA,
                        "name": "different-name",
                        "version": "1.0.0",
                    }
                )
            )
            with mock.patch.object(builder, "PLUGINS", plugins), self.assertRaisesRegex(
                ValueError, "does not match directory"
            ):
                builder.build("a" * 40, "2026-08-08T00:00:00Z")

    def test_git_diff_distinguishes_drift_from_execution_failure(self) -> None:
        commit = SimpleNamespace(returncode=0, stdout="a" * 40 + "\n", stderr="")
        success = SimpleNamespace(returncode=0, stdout="", stderr="")
        with mock.patch.object(
            builder.subprocess,
            "run",
            side_effect=[
                commit,
                success,
                success,
                SimpleNamespace(returncode=1, stderr=""),
            ],
        ), self.assertRaisesRegex(ValueError, "differs from"):
            builder.ensure_plugins_match_revision("a" * 40)
        with mock.patch.object(
            builder.subprocess,
            "run",
            side_effect=[
                commit,
                success,
                success,
                SimpleNamespace(returncode=128, stderr="fatal: bad object"),
            ],
        ), self.assertRaisesRegex(ValueError, "fatal: bad object"):
            builder.ensure_plugins_match_revision("a" * 40)

    def test_catalog_revision_must_be_an_ancestor_of_the_catalog_commit(self) -> None:
        with mock.patch.object(
            builder.subprocess,
            "run",
            side_effect=[
                SimpleNamespace(returncode=0, stdout="a" * 40 + "\n", stderr=""),
                SimpleNamespace(returncode=1, stdout="", stderr=""),
            ],
        ), self.assertRaisesRegex(ValueError, "must be an ancestor"):
            builder.ensure_plugins_match_revision("a" * 40)

    def test_catalog_revision_must_resolve_to_the_exact_commit(self) -> None:
        with mock.patch.object(
            builder.subprocess,
            "run",
            return_value=SimpleNamespace(returncode=128, stdout="", stderr="fatal: bad object"),
        ), self.assertRaisesRegex(ValueError, "must resolve to the exact commit"):
            builder.ensure_plugins_match_revision("a" * 40)

    def test_catalog_build_rejects_untracked_or_ignored_plugin_paths(self) -> None:
        with mock.patch.object(
            builder.subprocess,
            "run",
            side_effect=[
                SimpleNamespace(returncode=0, stdout="a" * 40 + "\n", stderr=""),
                SimpleNamespace(returncode=0, stdout="", stderr=""),
                SimpleNamespace(returncode=0, stdout="?? plugins/demo/extra\n", stderr=""),
            ],
        ), self.assertRaisesRegex(ValueError, "untracked paths"):
            builder.ensure_plugins_match_revision("a" * 40)


if __name__ == "__main__":
    unittest.main()
