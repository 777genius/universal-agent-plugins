from __future__ import annotations

import gzip
import copy
import importlib.util
import io
import json
import shutil
import subprocess
import tarfile
import tempfile
import unittest
from pathlib import Path
from unittest import mock

import yaml


MODULE_PATH = Path(__file__).resolve().parents[1] / "scripts" / "build_registry.py"
SPEC = importlib.util.spec_from_file_location("build_registry", MODULE_PATH)
assert SPEC and SPEC.loader
registry = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(registry)

CANONICAL_PRODUCT_IDS = {
    "agent-code-navigator",
    "atlassian",
    "chrome-devtools",
    "cloudflare",
    "cloudflare-bindings",
    "cloudflare-docs",
    "cloudflare-observability",
    "cloudflare-radar",
    "context7",
    "docker-hub",
    "figma",
    "firebase",
    "github",
    "gitlab",
    "greptile",
    "heroku",
    "hubspot-crm",
    "hubspot-developer",
    "linear",
    "neon",
    "notion",
    "sentry",
    "statsig",
    "stripe",
    "supabase",
    "vercel",
}


class FakeResponse:
    def __init__(self, body: bytes, url: str, *, status: int = 200, length: str | None = None):
        self._body = io.BytesIO(body)
        self._url = url
        self.status = status
        self.headers = {} if length is None else {"Content-Length": length}

    def read(self, size: int = -1) -> bytes:
        return self._body.read(size)

    def geturl(self) -> str:
        return self._url

    def __enter__(self):
        return self

    def __exit__(self, *_args):
        return None


class FakeOpener:
    def __init__(self, *responses: FakeResponse):
        self.responses = list(responses)
        self.requests = []
        self.timeouts = []

    def open(self, request, timeout=None):
        self.requests.append(request)
        self.timeouts.append(timeout)
        return self.responses.pop(0)

    @property
    def request(self):
        return self.requests[-1]

    @property
    def timeout(self):
        return self.timeouts[-1]


def archive_bytes(entries: list[tuple[str, bytes | None, str]], root: str = "repo-deadbeef") -> bytes:
    output = io.BytesIO()
    with tarfile.open(fileobj=output, mode="w:gz") as archive:
        for name, body, kind in entries:
            info = tarfile.TarInfo(f"{root}/{name}")
            if kind == "dir":
                info.type = tarfile.DIRTYPE
                info.size = 0
                archive.addfile(info)
            elif kind == "symlink":
                info.type = tarfile.SYMTYPE
                info.linkname = "../../escape"
                archive.addfile(info)
            elif kind == "fifo":
                info.type = tarfile.FIFOTYPE
                archive.addfile(info)
            elif kind == "sparse":
                info.type = tarfile.REGTYPE
                info.size = 1
                info.pax_headers = {"GNU.sparse.map": "0,1"}
                archive.addfile(info, io.BytesIO(b"x"))
            else:
                assert body is not None
                info.size = len(body)
                archive.addfile(info, io.BytesIO(body))
    return output.getvalue()


def valid_entries(name: str = "demo") -> list[tuple[str, bytes | None, str]]:
    manifest = {
        "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
        "name": name,
        "version": "1.2.3",
        "description": "Pinned demo plugin",
        "author": {"name": "Example Author", "url": "https://github.com/example"},
        "repository": "https://github.com/example/plugins",
        "license": "Apache-2.0",
        "keywords": ["demo"],
    }
    mcp = {
        "$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
        "mcpServers": {"demo": {"type": "streamable-http", "url": "https://example.com/mcp"}},
    }
    base = f"packages/{name}"
    return [
        (base, None, "dir"),
        (f"{base}/plugin.json", json.dumps(manifest).encode(), "file"),
        (f"{base}/README.md", b"# Demo\n", "file"),
        (f"{base}/mcp.json", json.dumps(mcp).encode(), "file"),
    ]


def commit_response(revision: str = "a" * 40, **updates) -> bytes:
    value = {"sha": revision, "url": registry.commit_api_url("example/plugins", revision), "tree": {"sha": "b" * 40}, "parents": []}
    value.update(updates)
    return json.dumps(value).encode()


def external_opener(body: bytes, revision: str = "a" * 40) -> FakeOpener:
    return FakeOpener(
        FakeResponse(commit_response(revision), registry.commit_api_url("example/plugins", revision)),
        FakeResponse(body, registry.archive_url("example/plugins", revision)),
    )


class RegistryDescriptorTests(unittest.TestCase):
    def descriptor(self, root: Path, **updates) -> Path:
        value = {"schema_version": 1, "repository": "example/plugins", "revision": "a" * 40, "path": "packages/demo", "categories": ["developer-tools"]}
        value.update(updates)
        path = root / "demo.json"
        path.write_text(json.dumps(value))
        return path

    def test_valid_descriptor(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            value = registry.validate_descriptor(self.descriptor(Path(tmp)))
            self.assertEqual(value["name"], "demo")

    def test_rejects_mutable_ref_credentials_and_url_syntax(self) -> None:
        invalid = [
            {"revision": "main"},
            {"repository": "user:token@github.com/example/plugins"},
            {"repository": "https://github.com/example/plugins?x=1"},
            {"repository": "Example/plugins"},
            {"repository": "example/plugins.git"},
        ]
        for update in invalid:
            with self.subTest(update=update), tempfile.TemporaryDirectory() as tmp:
                with self.assertRaises(registry.RegistryError):
                    registry.validate_descriptor(self.descriptor(Path(tmp), **update))

    def test_rejects_traversal_absolute_ambiguous_and_unicode_paths(self) -> None:
        for value in ["../demo", "/demo", "packages//demo", "packages/./demo", "packages\\demo", "packages/%64emo", "packages/de\u0301mo"]:
            with self.subTest(path=value), tempfile.TemporaryDirectory() as tmp:
                with self.assertRaises(registry.RegistryError):
                    registry.validate_descriptor(self.descriptor(Path(tmp), path=value))

    def test_rejects_unsorted_duplicate_or_invalid_categories(self) -> None:
        for value in [["z", "a"], ["a", "a"], ["Not-Slug"], [], ["a"] * 9]:
            with self.subTest(categories=value), tempfile.TemporaryDirectory() as tmp:
                with self.assertRaises(registry.RegistryError):
                    registry.validate_descriptor(self.descriptor(Path(tmp), categories=value))

    def test_submitter_cannot_assign_claim_fields(self) -> None:
        for field in ["featured", "verified", "official", "tested", "downloads", "ranking", "name", "description"]:
            with self.subTest(field=field), tempfile.TemporaryDirectory() as tmp:
                with self.assertRaises(registry.RegistryError):
                    registry.validate_descriptor(self.descriptor(Path(tmp), **{field: True}))

    def test_rejects_duplicate_keys_and_nonstandard_json_numbers(self) -> None:
        documents = [
            '{"schema_version":1,"schema_version":1}',
            '{"repository":"example/plugins","Repository":"example/plugins"}',
            '{"café":1,"cafe\\u0301":2}',
            '{"schema_version":NaN}',
        ]
        for document in documents:
            with self.subTest(document=document), tempfile.TemporaryDirectory() as tmp:
                path = Path(tmp) / "demo.json"
                path.write_text(document)
                with self.assertRaises(registry.RegistryError):
                    registry.validate_descriptor(path)


class NetworkLimitTests(unittest.TestCase):
    def download(self, response: FakeResponse, destination: Path) -> None:
        registry.download_archive("example/plugins", "a" * 40, destination, FakeOpener(response))

    def test_uses_only_exact_approved_archive_url_and_timeout(self) -> None:
        url = registry.archive_url("example/plugins", "a" * 40)
        opener = FakeOpener(FakeResponse(b"abc", url))
        with tempfile.TemporaryDirectory() as tmp:
            registry.download_archive("example/plugins", "a" * 40, Path(tmp) / "a.tgz", opener)
        self.assertEqual(opener.request.full_url, url)
        self.assertEqual(opener.timeout, registry.CONNECT_TIMEOUT_SECONDS)

    def test_rejects_redirect_or_unapproved_final_host(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            response = FakeResponse(b"x", "https://evil.example/archive")
            with self.assertRaises(registry.RegistryError):
                self.download(response, Path(tmp) / "a.tgz")

    def test_rejects_bad_status_and_content_length(self) -> None:
        url = registry.archive_url("example/plugins", "a" * 40)
        responses = [FakeResponse(b"", url, status=404), FakeResponse(b"", url, length=str(registry.MAX_DOWNLOAD_BYTES + 1)), FakeResponse(b"", url, length="-1")]
        for response in responses:
            with self.subTest(status=response.status, headers=response.headers), tempfile.TemporaryDirectory() as tmp:
                with self.assertRaises(registry.RegistryError):
                    self.download(response, Path(tmp) / "a.tgz")

    def test_enforces_streamed_compressed_size(self) -> None:
        url = registry.archive_url("example/plugins", "a" * 40)
        with tempfile.TemporaryDirectory() as tmp, mock.patch.object(registry, "MAX_DOWNLOAD_BYTES", 4):
            with self.assertRaises(registry.RegistryError):
                self.download(FakeResponse(b"12345", url), Path(tmp) / "a.tgz")

    def test_enforces_total_elapsed_time(self) -> None:
        url = registry.archive_url("example/plugins", "a" * 40)
        with tempfile.TemporaryDirectory() as tmp, mock.patch.object(registry.time, "monotonic", side_effect=[0, registry.TOTAL_DOWNLOAD_SECONDS + 1]):
            with self.assertRaises(registry.RegistryError):
                self.download(FakeResponse(b"x", url), Path(tmp) / "a.tgz")


class CommitResolutionTests(unittest.TestCase):
    def test_resolves_exact_git_commit_before_archive_without_forwarding_token(self) -> None:
        revision = "a" * 40
        opener = external_opener(archive_bytes(valid_entries()), revision)
        descriptor = {"name": "demo", "repository": "example/plugins", "revision": revision, "path": "packages/demo", "categories": ["developer-tools"]}
        with mock.patch.dict(registry.os.environ, {"GITHUB_TOKEN": "secret-token"}, clear=False):
            registry.external_entry(descriptor, opener)
        self.assertEqual([request.full_url for request in opener.requests], [registry.commit_api_url("example/plugins", revision), registry.archive_url("example/plugins", revision)])
        self.assertEqual(opener.requests[0].get_header("Authorization"), "Bearer secret-token")
        self.assertIsNone(opener.requests[1].get_header("Authorization"))
        self.assertEqual(opener.timeouts, [registry.CONNECT_TIMEOUT_SECONDS, registry.CONNECT_TIMEOUT_SECONDS])

    def test_token_cannot_follow_api_redirect_to_another_host(self) -> None:
        request = registry.urllib.request.Request(
            registry.commit_api_url("example/plugins", "a" * 40),
            headers={"Authorization": "Bearer secret-token"},
        )
        with self.assertRaises(registry.RegistryError):
            registry.NoRedirect().redirect_request(request, None, 302, "Found", {}, "https://evil.example/steal")

    def test_fails_closed_on_status_url_type_sha_and_malformed_json(self) -> None:
        revision = "a" * 40
        url = registry.commit_api_url("example/plugins", revision)
        cases = [
            FakeResponse(commit_response(revision), url, status=404),
            FakeResponse(commit_response(revision), "https://evil.example/commit"),
            FakeResponse(b"[]", url),
            FakeResponse(commit_response("c" * 40), url),
            FakeResponse(b"{bad", url),
            FakeResponse(b'{"sha":"' + revision.encode() + b'","sha":"' + revision.encode() + b'"}', url),
            FakeResponse(commit_response(revision, url="https://evil.example/commit"), url),
            FakeResponse(json.dumps({"sha": revision, "tree": "not-an-object", "parents": []}).encode(), url),
            FakeResponse(commit_response(revision, parents=[{"sha": "not-a-sha"}]), url),
        ]
        for response in cases:
            with self.subTest(status=response.status, url=response.geturl(), body=response._body.getvalue()):
                with self.assertRaises(registry.RegistryError):
                    registry.resolve_commit("example/plugins", revision, FakeOpener(response))

    def test_bounds_commit_response_bytes_content_length_and_time(self) -> None:
        revision = "a" * 40
        url = registry.commit_api_url("example/plugins", revision)
        responses = [
            FakeResponse(b"{}", url, length=str(registry.MAX_API_RESPONSE_BYTES + 1)),
            FakeResponse(b"{}", url, length="-1"),
        ]
        for response in responses:
            with self.subTest(headers=response.headers), self.assertRaises(registry.RegistryError):
                registry.resolve_commit("example/plugins", revision, FakeOpener(response))
        with mock.patch.object(registry, "MAX_API_RESPONSE_BYTES", 4), self.assertRaises(registry.RegistryError):
            registry.resolve_commit("example/plugins", revision, FakeOpener(FakeResponse(b"12345", url)))
        with mock.patch.object(registry.time, "monotonic", side_effect=[0, registry.API_TOTAL_SECONDS + 1]), self.assertRaises(registry.RegistryError):
            registry.resolve_commit("example/plugins", revision, FakeOpener(FakeResponse(b"{}", url)))


class ArchiveLimitTests(unittest.TestCase):
    def extract(self, body: bytes, destination: Path) -> None:
        compressed, expanded = destination.parent / "a.tgz", destination.parent / "a.tar"
        compressed.write_bytes(body)
        registry.decompress_archive(compressed, expanded)
        destination.mkdir()
        registry.extract_package(expanded, "packages/demo", destination)

    def test_valid_package_extracts(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            destination = Path(tmp) / "package"
            self.extract(archive_bytes(valid_entries()), destination)
            self.assertTrue((destination / "plugin.json").is_file())

    def test_rejects_links_special_and_sparse_files(self) -> None:
        for kind in ["symlink", "fifo", "sparse"]:
            entries = valid_entries() + [(f"packages/demo/{kind}", None, kind)]
            with self.subTest(kind=kind), tempfile.TemporaryDirectory() as tmp:
                with self.assertRaises(registry.RegistryError):
                    self.extract(archive_bytes(entries), Path(tmp) / "package")

    def test_rejects_archive_traversal_absolute_ambiguous_and_unicode(self) -> None:
        names = ["../escape", "/absolute", "packages/demo/bad\\name", "packages/demo/de\u0301mo"]
        for name in names:
            with self.subTest(name=name), tempfile.TemporaryDirectory() as tmp:
                with self.assertRaises(registry.RegistryError):
                    self.extract(archive_bytes(valid_entries() + [(name, b"x", "file")]), Path(tmp) / "package")

    def test_enforces_file_count_per_file_total_and_depth(self) -> None:
        cases = [
            ("MAX_FILES", 1, valid_entries() + [("packages/demo/extra", b"x", "file")]),
            ("MAX_FILE_BYTES", 1, valid_entries() + [("packages/demo/extra", b"xx", "file")]),
            ("MAX_EXTRACTED_BYTES", 3, valid_entries()),
            ("MAX_PATH_DEPTH", 2, valid_entries() + [("packages/demo/a/b/c", b"x", "file")]),
        ]
        for constant, limit, entries in cases:
            with self.subTest(limit=constant), tempfile.TemporaryDirectory() as tmp, mock.patch.object(registry, constant, limit):
                with self.assertRaises(registry.RegistryError):
                    self.extract(archive_bytes(entries), Path(tmp) / "package")

    def test_enforces_expanded_archive_limit(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            compressed, expanded = Path(tmp) / "a.gz", Path(tmp) / "a"
            compressed.write_bytes(gzip.compress(b"12345"))
            with mock.patch.object(registry, "MAX_ARCHIVE_BYTES", 4), self.assertRaises(registry.RegistryError):
                registry.decompress_archive(compressed, expanded)

    def test_enforces_archive_processing_time(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            compressed, expanded = Path(tmp) / "a.gz", Path(tmp) / "a"
            compressed.write_bytes(gzip.compress(b"12345"))
            with mock.patch.object(registry.time, "monotonic", side_effect=[0, registry.ARCHIVE_PROCESS_SECONDS + 1]), self.assertRaises(registry.RegistryError):
                registry.decompress_archive(compressed, expanded)

    def test_rejects_case_collisions_and_multiple_roots(self) -> None:
        collisions = valid_entries() + [("packages/demo/README.MD", b"x", "file")]
        roots = archive_bytes(valid_entries())
        # A second tar root is constructed explicitly because archive_bytes prefixes all entries.
        output = io.BytesIO()
        with tarfile.open(fileobj=output, mode="w:gz") as archive:
            for name in ["root-a/packages/demo", "root-b/other"]:
                info = tarfile.TarInfo(name)
                info.type = tarfile.DIRTYPE
                archive.addfile(info)
        for body in [archive_bytes(collisions), output.getvalue()]:
            with tempfile.TemporaryDirectory() as tmp, self.assertRaises(registry.RegistryError):
                self.extract(body, Path(tmp) / "package")

    def test_rejects_duplicates_outside_selected_package_and_member_floods(self) -> None:
        duplicate = valid_entries() + [("other/file", b"one", "file"), ("OTHER/FILE", b"two", "file")]
        with tempfile.TemporaryDirectory() as tmp, self.assertRaises(registry.RegistryError):
            self.extract(archive_bytes(duplicate), Path(tmp) / "package")
        with tempfile.TemporaryDirectory() as tmp, mock.patch.object(registry, "MAX_MEMBERS", 1), self.assertRaises(registry.RegistryError):
            self.extract(archive_bytes(valid_entries()), Path(tmp) / "package")


class ExternalPackageTests(unittest.TestCase):
    def test_external_entry_is_pinned_schema_only_and_derived_from_package(self) -> None:
        body = archive_bytes(valid_entries() + [("packages/demo/icon.svg", b"<svg/>", "file")])
        descriptor = {"name": "demo", "repository": "example/plugins", "revision": "a" * 40, "path": "packages/demo", "categories": ["developer-tools"]}
        item = registry.external_entry(descriptor, external_opener(body))
        self.assertEqual(item["install_source"], f"example/plugins@{'a' * 40}//packages/demo")
        self.assertEqual(item["author"]["name"], "Example Author")
        self.assertEqual(item["license"], "Apache-2.0")
        self.assertEqual(item["validation"]["level"], "schema_only")
        self.assertEqual(item["client_support"], {"resolution": "install_time", "clients": list(registry.CLIENT_IDS)})
        self.assertRegex(item["source"]["tree_sha256"], r"^sha256:[0-9a-f]{64}$")
        self.assertNotIn("icon", item)
        self.assertNotIn("icon_sha256", item["source"])

    def test_rejects_duplicate_keys_in_component_json(self) -> None:
        entries = valid_entries()
        entries[3] = (entries[3][0], b'{"mcpServers":{},"mcpServers":{}}', "file")
        descriptor = {"name": "demo", "repository": "example/plugins", "revision": "a" * 40, "path": "packages/demo", "categories": ["developer-tools"]}
        with self.assertRaises(registry.RegistryError):
            registry.external_entry(descriptor, external_opener(archive_bytes(entries)))

    def test_allows_array_data_json_without_weakening_duplicate_checks(self) -> None:
        entries = valid_entries() + [("packages/demo/data.json", b'[1, 2, 3]', "file")]
        descriptor = {"name": "demo", "repository": "example/plugins", "revision": "a" * 40, "path": "packages/demo", "categories": ["developer-tools"]}
        item = registry.external_entry(descriptor, external_opener(archive_bytes(entries)))
        self.assertEqual(item["name"], "demo")

    def test_rejects_manifest_name_or_repository_mismatch_and_missing_license(self) -> None:
        mutations = [("name", "other"), ("repository", "https://github.com/other/plugins"), ("license", "")]
        for field, value in mutations:
            entries = valid_entries()
            manifest = json.loads(entries[1][1])
            manifest[field] = value
            entries[1] = (entries[1][0], json.dumps(manifest).encode(), "file")
            descriptor = {"name": "demo", "repository": "example/plugins", "revision": "a" * 40, "path": "packages/demo", "categories": ["developer-tools"]}
            with self.subTest(field=field), self.assertRaises(registry.RegistryError):
                registry.external_entry(descriptor, external_opener(archive_bytes(entries)))

    def test_existing_mcp_secret_checks_are_preserved(self) -> None:
        entries = valid_entries()
        mcp = json.loads(entries[3][1])
        mcp["mcpServers"]["demo"]["headers"] = {"Authorization": "token"}
        entries[3] = (entries[3][0], json.dumps(mcp).encode(), "file")
        descriptor = {"name": "demo", "repository": "example/plugins", "revision": "a" * 40, "path": "packages/demo", "categories": ["developer-tools"]}
        with self.assertRaises(registry.RegistryError):
            registry.external_entry(descriptor, external_opener(archive_bytes(entries)))

    def test_rejects_plugin_manifest_that_fails_vendored_schema(self) -> None:
        entries = valid_entries()
        manifest = json.loads(entries[1][1])
        manifest["homepage"] = 42
        entries[1] = (entries[1][0], json.dumps(manifest).encode(), "file")
        descriptor = {"name": "demo", "repository": "example/plugins", "revision": "a" * 40, "path": "packages/demo", "categories": ["developer-tools"]}
        with self.assertRaisesRegex(registry.RegistryError, r"plugin\.json: Agent Plugins 1\.0 schema error.*homepage"):
            registry.external_entry(descriptor, external_opener(archive_bytes(entries)))

    def test_rejects_mcp_configuration_that_fails_vendored_schema(self) -> None:
        entries = valid_entries()
        mcp = json.loads(entries[3][1])
        mcp["mcpServers"]["demo"]["url"] = 42
        entries[3] = (entries[3][0], json.dumps(mcp).encode(), "file")
        descriptor = {"name": "demo", "repository": "example/plugins", "revision": "a" * 40, "path": "packages/demo", "categories": ["developer-tools"]}
        with self.assertRaisesRegex(registry.RegistryError, r"mcp\.json: Agent Plugins 1\.0 schema error"):
            registry.external_entry(descriptor, external_opener(archive_bytes(entries)))


class GeneratedIndexTests(unittest.TestCase):
    def test_committed_legacy_index_is_byte_frozen_complete_and_sorted(self) -> None:
        registry.validate_legacy_catalog_freeze()
        first = registry.OUTPUT.read_bytes()
        index = json.loads(first)
        self.assertEqual(index["schema_version"], 1)
        self.assertGreaterEqual(len(index["plugins"]), 26)
        names = [item["name"] for item in index["plugins"]]
        self.assertEqual(names, sorted(names))
        required = {"name", "version", "description", "author", "license", "categories", "keywords", "source", "install_source", "built_in", "client_support", "validation", "components"}
        for item in index["plugins"]:
            self.assertTrue(required.issubset(item))
            if item["built_in"]:
                self.assertEqual(item["install_source"], item["name"])
                self.assertEqual(item["client_support"]["resolution"], "catalog")
            else:
                source = item["source"]
                self.assertEqual(
                    item["install_source"],
                    f"{source['repository']}@{source['revision']}//{source['path']}",
                )
                self.assertEqual(item["client_support"]["resolution"], "install_time")
            self.assertTrue(item["client_support"]["clients"])
        self.assertEqual(sum(item["built_in"] for item in index["plugins"]), 26)
        context7 = next(item for item in index["plugins"] if item["name"] == "context7")
        cloudflare_docs = next(item for item in index["plugins"] if item["name"] == "cloudflare-docs")
        self.assertNotIn("chatgpt", context7["client_support"]["clients"])
        self.assertIn("chatgpt", cloudflare_docs["client_support"]["clients"])

    def test_generated_index_accepts_26_builtins_plus_valid_external(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            entries = Path(tmp)
            descriptor = {
                "schema_version": 1,
                "repository": "example/plugins",
                "revision": "a" * 40,
                "path": "packages/demo",
                "categories": ["developer-tools"],
            }
            (entries / "demo.json").write_text(json.dumps(descriptor))
            opener = external_opener(archive_bytes(valid_entries()))
            frozen_builtins = json.loads(registry.OUTPUT.read_bytes())["plugins"]
            with mock.patch.object(registry, "ENTRIES", entries), mock.patch.object(registry, "builtin_entries", return_value=frozen_builtins):
                index = registry.build(opener)

        self.assertEqual(len(index["plugins"]), 27)
        self.assertEqual(sum(item["built_in"] for item in index["plugins"]), 26)
        external = next(item for item in index["plugins"] if not item["built_in"])
        self.assertEqual(
            external["install_source"],
            f"example/plugins@{'a' * 40}//packages/demo",
        )
        self.assertEqual(external["client_support"]["resolution"], "install_time")

    def test_builtin_name_cannot_be_claimed_by_descriptor(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            entries = Path(tmp)
            descriptor = {"schema_version": 1, "repository": "example/plugins", "revision": "a" * 40, "path": "packages/demo", "categories": ["developer-tools"]}
            (entries / "demo.json").write_text(json.dumps(descriptor))
            builtin = {"name": "demo"}
            with mock.patch.object(registry, "ENTRIES", entries), mock.patch.object(registry, "builtin_entries", return_value=[builtin]), self.assertRaises(registry.RegistryError):
                registry.build()


class DirectoryTreeDigestTests(unittest.TestCase):
    def fixture(self):
        return json.loads((Path(__file__).parent / "fixtures" / "directory" / "tree-digest-golden.json").read_text())

    def framed_entries(self, fixture):
        return [
            tuple(item[field].encode("utf-8") for field in ("path", "kind", "mode", "target", "content"))
            for item in fixture["entries"]
        ]

    def test_cross_language_go_golden_framing(self) -> None:
        fixture = self.fixture()
        self.assertEqual(fixture["algorithm"], registry.DIRECTORY_TREE_DIGEST_ALGORITHM)
        entries = self.framed_entries(fixture)
        self.assertEqual(registry._directory_tree_digest_entries(entries.copy()), fixture["expected_digest"])
        self.assertEqual(registry._directory_tree_digest_entries(entries.copy()), fixture["expected_digest"])

    def test_publishable_files_directories_and_modes_match_golden_subset(self) -> None:
        fixture = self.fixture()
        publishable = [item for item in fixture["entries"] if item["kind"] != "symlink"]
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            for item in publishable:
                path = root / item["path"]
                if item["kind"] == "directory":
                    path.mkdir(parents=True, exist_ok=True)
                    continue
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_bytes(item["content"].encode("utf-8"))
                path.chmod(0o755 if item["mode"] == "100755" else 0o644)
            (root / ".git").mkdir()
            (root / ".git" / "ignored").write_bytes(b"not package content")
            (root / ".plugin-kit-ai.lock").write_bytes(b"not package content")
            expected = registry._directory_tree_digest_entries(self.framed_entries({"entries": publishable}))
            self.assertEqual(registry.directory_tree_digest(root), expected)
            self.assertEqual(registry.directory_tree_digest(root), expected)

    def test_publishing_keeps_stricter_symlink_rejection(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "target").write_bytes(b"")
            (root / "link").symlink_to("target")
            with self.assertRaisesRegex(ValueError, "symlink"):
                registry.directory_tree_digest(root)


class DirectoryDomainTests(unittest.TestCase):
    def source(self):
        return registry.load_directory_source()

    def fixture(self):
        return json.loads((Path(__file__).parent / "fixtures" / "directory" / "domain-source.json").read_text())

    def external_product_source(self):
        source = copy.deepcopy(self.source())
        product = copy.deepcopy(source["products"][0])
        product.update({
            "id": "zz-community-product",
            "display_name": "Community Product",
            "description": "A valid external community product used only by this in-memory regression fixture.",
            "manifest_name": "zz-community-product",
            "aliases": ["zz-community-product"],
            "reserved_aliases": ["zz-community-product"],
            "default_distribution": "zz-community/zz-community-product",
            "distributions": ["zz-community/zz-community-product"],
        })
        distribution = copy.deepcopy(source["distributions"][0])
        distribution.update({
            "id": "zz-community/zz-community-product",
            "product_id": "zz-community-product",
            "packager": "zz-community",
        })
        distribution["releases"][0].update({
            "manifest_name": "zz-community-product",
            "package_source": {
                "repository": "zz-community/zz-community-product",
                "revision": "a" * 40,
                "path": "packages/zz-community-product",
            },
        })
        source["products"].append(product)
        source["distributions"].append(distribution)
        return source

    def local_external_release(self, root: Path):
        repository = root / "external"
        package = repository / "packages" / "zz-community-product"
        package.mkdir(parents=True)
        manifest = {
            "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
            "name": "zz-community-product",
            "version": "1.2.3",
            "description": "Local deterministic external fixture",
            "author": {"name": "Fixture Author"},
            "repository": "https://github.com/example/external",
            "license": "Apache-2.0",
            "keywords": ["fixture"],
        }
        (package / "plugin.json").write_text(json.dumps(manifest) + "\n")
        (package / "mcp.json").write_text(json.dumps({
            "$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
            "mcpServers": {"fixture": {"type": "streamable-http", "url": "https://example.test/mcp"}},
        }) + "\n")
        (package / "README.md").write_text("# Fixture\n")
        subprocess.run(["/usr/bin/git", "init", "-q", str(repository)], check=True)
        subprocess.run(["/usr/bin/git", "-C", str(repository), "config", "uploadpack.allowFilter", "true"], check=True)
        subprocess.run(["/usr/bin/git", "-C", str(repository), "add", "."], check=True)
        subprocess.run([
            "/usr/bin/git", "-C", str(repository), "-c", "user.name=Fixture",
            "-c", "user.email=fixture@example.test", "commit", "-qm", "valid",
        ], check=True)
        revision = subprocess.check_output(["/usr/bin/git", "-C", str(repository), "rev-parse", "HEAD"], text=True).strip()
        source = self.external_product_source()
        distribution = source["distributions"][-1]
        release = distribution["releases"][0]
        facts = registry.validated_package_facts(package)
        release.update({
            "package_version": facts["package_version"],
            "manifest_name": facts["manifest_name"],
            "agent_plugins_schema": facts["agent_plugins_schema"],
            "package_source": {
                "repository": "example/external", "revision": revision,
                "path": "packages/zz-community-product",
            },
            "tree_digest_algorithm": registry.DIRECTORY_TREE_DIGEST_ALGORITHM,
            "tree_digest": registry.directory_tree_digest(package),
            "manifest_digest": registry.digest_bytes((package / "plugin.json").read_bytes()),
            "components": facts["components"],
        })
        return source, repository, package, revision

    def isolate_external_product(self, source):
        product = source["products"][-1]
        distribution = source["distributions"][-1]
        source["products"] = [product]
        source["distributions"] = [distribution]
        source["evidence"] = []
        return distribution

    def commit_fixture_change(self, repository: Path, message: str) -> str:
        subprocess.run(["/usr/bin/git", "-C", str(repository), "add", "."], check=True)
        subprocess.run([
            "/usr/bin/git", "-C", str(repository), "-c", "user.name=Fixture",
            "-c", "user.email=fixture@example.test", "commit", "-qm", message,
        ], check=True)
        return subprocess.check_output(["/usr/bin/git", "-C", str(repository), "rev-parse", "HEAD"], text=True).strip()

    def test_missing_base_directory_is_treated_as_initial_migration(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repository = Path(tmp) / "repository"
            repository.mkdir()
            subprocess.run(["/usr/bin/git", "-C", str(repository), "init", "-q"], check=True)
            (repository / "README.md").write_text("pre-Directory base\n")
            subprocess.run(["/usr/bin/git", "-C", str(repository), "add", "README.md"], check=True)
            subprocess.run([
                "/usr/bin/git", "-C", str(repository), "-c", "user.name=Fixture",
                "-c", "user.email=fixture@example.test", "commit", "-qm", "base",
            ], check=True)
            revision = subprocess.check_output(
                ["/usr/bin/git", "-C", str(repository), "rev-parse", "HEAD"], text=True,
            ).strip()

            with mock.patch.object(registry, "ROOT", repository):
                self.assertIsNone(registry.load_directory_source_at_revision(revision))
                with self.assertRaisesRegex(registry.RegistryError, "cannot read Directory source"):
                    registry.load_directory_source_at_revision("f" * 40)

    def test_changed_external_release_reacquires_valid_local_git_source(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            source, repository, _package, _revision = self.local_external_release(Path(tmp))
            changed = registry.validate_changed_external_releases(
                source, self.source(), repository_overrides={"example/external": repository},
            )
            self.assertEqual(changed, [("zz-community/zz-community-product", 1)])

    def test_initial_migration_skips_revoked_release_without_acquisition(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            source, repository, _package, _revision = self.local_external_release(Path(tmp))
            distribution = self.isolate_external_product(source)
            distribution["releases"][0]["package_source"]["repository"] = "777genius/universal-agent-plugins"
            distribution["release_policies"][0]["status"] = "revoked"
            acquirer = mock.Mock(side_effect=AssertionError("revoked release was acquired"))

            self.assertEqual(
                registry.validate_changed_local_releases(
                    source, None, repository_root=repository, acquirer=acquirer,
                ),
                [],
            )
            acquirer.assert_not_called()

    def test_initial_migration_rejects_active_release_with_live_npx(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            source, repository, package, _revision = self.local_external_release(Path(tmp))
            self.isolate_external_product(source)
            (package / "mcp.json").write_text(json.dumps({
                "$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
                "mcpServers": {"fixture": {
                    "type": "stdio", "command": "npx", "args": ["fixture-runtime@1.2.3"],
                }},
            }) + "\n")
            revision = self.commit_fixture_change(repository, "live npx initial migration")
            release = source["distributions"][-1]["releases"][0]
            release["package_source"]["revision"] = revision
            release["tree_digest"] = registry.directory_tree_digest(package)
            release["components"] = registry.validated_package_facts(package)["components"]

            with self.assertRaisesRegex(
                registry.RegistryError,
                "live npx without a recognized content-addressed runtime closure contract",
            ):
                registry.validate_changed_external_releases(
                    source, None, repository_overrides={"example/external": repository},
                )

    def test_initial_migration_validates_non_revoked_suspended_and_candidate_releases(self) -> None:
        for status in ("suspended", "candidate"):
            with self.subTest(status=status), tempfile.TemporaryDirectory() as tmp:
                source, repository, _package, _revision = self.local_external_release(Path(tmp))
                self.isolate_external_product(source)["status"] = status

                self.assertEqual(
                    registry.validate_changed_external_releases(
                        source, None, repository_overrides={"example/external": repository},
                    ),
                    [("zz-community/zz-community-product", 1)],
                )

    def test_capability_relaxation_revalidates_external_release_but_display_edits_do_not(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            source, repository, _package, _revision = self.local_external_release(Path(tmp))
            product = next(item for item in source["products"] if item["id"] == "zz-community-product")
            product["minimum_capabilities"]["skills"] = "optional"
            previous = copy.deepcopy(source)
            old_product = next(item for item in previous["products"] if item["id"] == product["id"])
            old_product["minimum_capabilities"]["skills"] = "required"

            self.assertEqual(
                registry.validate_changed_external_releases(
                    source, previous,
                    repository_overrides={"example/external": repository},
                ),
                [("zz-community/zz-community-product", 1)],
            )
            with self.assertRaisesRegex(registry.RegistryError, "reacquisition failed closed"):
                registry.validate_changed_external_releases(
                    source, previous,
                    acquirer=mock.Mock(side_effect=OSError("offline")),
                )

            metadata_only = copy.deepcopy(source)
            next(item for item in metadata_only["products"] if item["id"] == product["id"])["description"] = "Updated display copy only."
            acquirer = mock.Mock(side_effect=AssertionError("display edit fetched package"))
            self.assertEqual(
                registry.validate_changed_external_releases(metadata_only, source, acquirer=acquirer),
                [],
            )
            acquirer.assert_not_called()

    def test_pr_validation_rejects_newly_eligible_historical_bridge_without_versioned_recipe(self) -> None:
        source = self.fixture()
        bridge = next(item for item in source["distributions"] if item["kind"] == "community_bridge")
        bridge["releases"][0]["package_source"]["repository"] = "777genius/universal-agent-plugins"
        older = copy.deepcopy(bridge["releases"][0])
        older["sequence"] = 2
        newer = copy.deepcopy(bridge["releases"][0])
        newer["sequence"] = 3
        bridge["releases"] = [older, newer]
        older_policy = copy.deepcopy(bridge["release_policies"][0])
        older_policy["release_sequence"] = 2
        newer_policy = copy.deepcopy(older_policy)
        newer_policy["release_sequence"] = 3
        bridge["release_policies"] = [older_policy, newer_policy]
        previous = copy.deepcopy(source)
        previous["products"][0]["minimum_capabilities"]["skills"] = "required"
        source["products"][0]["minimum_capabilities"]["skills"] = "optional"

        with self.assertRaisesRegex(
            registry.RegistryError,
            r"@2: active historical bridge requires reproduction.*versioned historical reproduction inputs are unavailable",
        ):
            registry.validate_historical_bridge_eligibility(source, previous)

        bridge["releases"] = [newer]
        bridge["release_policies"] = [newer_policy]
        previous["distributions"] = [
            item for item in previous["distributions"] if item["id"] != bridge["id"]
        ] + [copy.deepcopy(bridge)]
        previous["distributions"].sort(key=lambda item: item["id"])
        registry.validate_historical_bridge_eligibility(source, previous)

    def test_pr_validation_uses_pinned_local_release_not_newest_worktree_path(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            source, repository, package, _revision = self.local_external_release(Path(tmp))
            manifest = json.loads((package / "plugin.json").read_text())
            manifest["repository"] = "https://github.com/777genius/universal-agent-plugins"
            (package / "plugin.json").write_text(json.dumps(manifest) + "\n")
            revision = self.commit_fixture_change(repository, "canonical local identity")
            distribution = source["distributions"][-1]
            release = distribution["releases"][0]
            release["package_source"]["repository"] = "777genius/universal-agent-plugins"
            release["package_source"]["revision"] = revision
            release["tree_digest"] = registry.directory_tree_digest(package)
            release["manifest_digest"] = registry.digest_bytes((package / "plugin.json").read_bytes())
            product = source["products"][-1]
            product["minimum_capabilities"]["skills"] = "optional"
            previous = copy.deepcopy(source)
            previous["products"][-1]["minimum_capabilities"]["skills"] = "required"

            (package / "mcp.json").write_text(json.dumps({
                "$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
                "mcpServers": {"fixture": {
                    "type": "stdio", "command": "npx", "args": ["fixture@1.0.0"],
                }},
            }) + "\n")
            self.assertEqual(
                registry.validate_changed_local_releases(
                    source, previous, repository_root=repository,
                ),
                [("zz-community/zz-community-product", 1)],
            )
            with self.assertRaisesRegex(registry.RegistryError, "local reacquisition failed closed"):
                registry.validate_changed_local_releases(
                    source, previous, repository_root=repository,
                    acquirer=mock.Mock(side_effect=OSError("offline")),
                )

    def test_changed_external_release_rejects_live_npx_launcher_aliases(self) -> None:
        commands = ("npx.cmd", "NPX", r"C:\\tools\\npx.exe", "/usr/local/bin/npx")
        for command in commands:
            with self.subTest(command=command), tempfile.TemporaryDirectory() as tmp:
                source, repository, package, _revision = self.local_external_release(Path(tmp))
                (package / "mcp.json").write_text(json.dumps({
                    "$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
                    "mcpServers": {
                        "fixture": {
                            "type": "stdio",
                            "command": command,
                            "args": ["-y", "fixture-runtime@1.2.3"],
                        },
                    },
                }) + "\n")
                revision = self.commit_fixture_change(repository, "live npx alias")
                release = source["distributions"][-1]["releases"][0]
                release["package_source"]["revision"] = revision
                release["tree_digest"] = registry.directory_tree_digest(package)
                release["manifest_digest"] = registry.digest_bytes((package / "plugin.json").read_bytes())
                release["components"] = registry.validated_package_facts(package)["components"]
                with self.assertRaisesRegex(
                    registry.RegistryError,
                    "live npx without a recognized content-addressed runtime closure contract",
                ):
                    registry.validate_changed_external_releases(
                        source, self.source(),
                        repository_overrides={"example/external": repository},
                    )

    def test_centralized_external_validator_rejects_npx_alias_and_preserves_digest_checks(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            source, _repository, package, _revision = self.local_external_release(Path(tmp))
            release = source["distributions"][-1]["releases"][0]
            (package / "mcp.json").write_text(json.dumps({
                "$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
                "mcpServers": {"fixture": {
                    "type": "stdio", "command": "NPX.BAT",
                    "args": ["-y", "fixture-runtime@1.2.3"],
                }},
            }) + "\n")
            release["tree_digest"] = registry.directory_tree_digest(package)
            release["components"] = registry.validated_package_facts(package)["components"]
            with self.assertRaisesRegex(
                registry.RegistryError,
                "live npx without a recognized content-addressed runtime closure contract",
            ):
                registry.validate_external_release_package(package, release, label="signing boundary")

            (package / "mcp.json").write_text(json.dumps({
                "$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
                "mcpServers": {"fixture": {
                    "type": "stdio", "command": "npx-wrapper.cmd", "args": ["runtime@latest"],
                }},
            }) + "\n")
            release["tree_digest"] = registry.directory_tree_digest(package)
            release["components"] = registry.validated_package_facts(package)["components"]
            registry.validate_external_release_package(package, release, label="signing boundary")
            (package / "README.md").write_text("tampered after review\n")
            with self.assertRaisesRegex(registry.RegistryError, "tree digest differs"):
                registry.validate_external_release_package(package, release, label="signing boundary")

    def test_external_release_rejects_substituted_and_malformed_local_git_sources(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            source, repository, package, _revision = self.local_external_release(Path(tmp))
            (package / "README.md").write_text("substituted source bytes\n")
            substituted_revision = self.commit_fixture_change(repository, "substituted")
            source["distributions"][-1]["releases"][0]["package_source"]["revision"] = substituted_revision
            with self.assertRaisesRegex(registry.RegistryError, "tree digest differs"):
                registry.validate_changed_external_releases(
                    source, self.source(), repository_overrides={"example/external": repository},
                )

            (package / "plugin.json").write_text("{malformed\n")
            malformed_revision = self.commit_fixture_change(repository, "malformed")
            release = source["distributions"][-1]["releases"][0]
            release["package_source"]["revision"] = malformed_revision
            release["tree_digest"] = registry.directory_tree_digest(package)
            release["manifest_digest"] = registry.digest_bytes((package / "plugin.json").read_bytes())
            with self.assertRaisesRegex(registry.RegistryError, "invalid UTF-8 JSON"):
                registry.validate_changed_external_releases(
                    source, self.source(), repository_overrides={"example/external": repository},
                )

    def test_unchanged_external_release_is_not_reacquired_and_failures_fail_closed(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            source, repository, _package, _revision = self.local_external_release(Path(tmp))
            acquirer = mock.Mock(side_effect=AssertionError("unchanged release was fetched"))
            self.assertEqual(registry.validate_changed_external_releases(source, copy.deepcopy(source), acquirer=acquirer), [])
            acquirer.assert_not_called()

            release = source["distributions"][-1]["releases"][0]
            release["package_source"]["revision"] = "f" * 40
            with self.assertRaisesRegex(registry.RegistryError, "reacquisition failed closed"):
                registry.validate_changed_external_releases(
                    source, self.source(), repository_overrides={"example/external": repository},
                )

    def assert_canonical_products_present_once(self, source) -> None:
        product_ids = [item["id"] for item in source["products"]]
        self.assertEqual(
            {product_id: product_ids.count(product_id) for product_id in CANONICAL_PRODUCT_IDS},
            dict.fromkeys(CANONICAL_PRODUCT_IDS, 1),
        )

    def assert_one_card_per_product(self, source) -> None:
        source_ids = [item["id"] for item in source["products"]]
        preview = registry.directory_preview(source)
        search = registry.directory_search(source)
        preview_ids = [item["id"] for item in preview["products"]]
        search_ids = [item["product_id"] for item in search["entries"]]
        self.assertEqual(preview["product_count"], len(source_ids))
        self.assertEqual(preview_ids, source_ids)
        self.assertEqual(search_ids, source_ids)
        self.assertEqual(len(set(preview_ids)), len(source_ids))
        self.assertEqual(len(set(search_ids)), len(source_ids))

    def test_canonical_products_include_the_first_bridge_cohort_and_context7_alternative(self) -> None:
        source = self.source()
        registry.validate_directory(source)
        self.assert_canonical_products_present_once(source)
        self.assertEqual(CANONICAL_PRODUCT_IDS, {path.name for path in registry.ROOT.joinpath("plugins").iterdir() if path.is_dir()})
        alternatives = {
            "chrome-devtools": ["777genius/chrome-devtools", "777genius/chrome-devtools-bridge"],
            "cloudflare-docs": ["777genius/cloudflare-docs", "777genius/cloudflare-docs-bridge"],
            "context7": ["777genius/context7", "upstash/context7"],
            "github": ["777genius/github", "777genius/github-bridge"],
        }
        products = {item["id"]: item for item in source["products"]}
        expected_distribution_ids = set()
        for product_id in CANONICAL_PRODUCT_IDS:
            product = products[product_id]
            self.assertEqual(product["aliases"], [product["id"]])
            self.assertEqual(product["reserved_aliases"], [product["id"]])
            expected = alternatives.get(product["id"], [f"777genius/{product['id']}"])
            self.assertEqual(product["distributions"], expected)
            expected_distribution_ids.update(expected)
        distribution_ids = [item["id"] for item in source["distributions"]]
        self.assertEqual(
            {distribution_id: distribution_ids.count(distribution_id) for distribution_id in expected_distribution_ids},
            dict.fromkeys(expected_distribution_ids, 1),
        )
        for distribution in source["distributions"]:
            if distribution["id"] not in expected_distribution_ids:
                continue
            suspended_live_npx = {
                "777genius/chrome-devtools",
                "777genius/chrome-devtools-bridge",
                "777genius/context7",
                "777genius/firebase",
                "777genius/hubspot-developer",
            }
            expected_status = "suspended" if distribution["id"] in suspended_live_npx else "active"
            self.assertEqual(distribution["status"], expected_status)
            self.assertEqual([item["sequence"] for item in distribution["releases"]], [1])
            self.assertEqual(
                [item["tree_digest_algorithm"] for item in distribution["releases"]],
                ["agentplugins-tree-sha256-v1"],
            )
            self.assertEqual([item["release_sequence"] for item in distribution["release_policies"]], [1])

    def test_bridge_cohort_preserves_every_migrated_legacy_distribution(self) -> None:
        source = self.source()
        actual = {item["id"]: item for item in source["distributions"]}
        expected_digests = {
            "777genius/chrome-devtools": ("sha256:46e983660fc3fadfcd51465d66a7dfdf18149f907f54e4b190a3ec1b9ec4f9df", "sha256:e7d43a8e39b0e83f2c05777e297f6a3884002dc2601d70528b55a44132db8091"),
            "777genius/cloudflare-docs": ("sha256:afdb5ca9b565971bbb34dec9fe6e107fcff4af98e0cd51cbc718eb43938a282d", "sha256:6b575562e527194ccd31238fde8c5f764409ecc98e211e92dfb81455eef88bdd"),
            "777genius/github": ("sha256:2478a68a98982f115b5288c8c8931365635ce827581516ca54e11266e95d28a3", "sha256:091e37c5f33c0a92f77070cfcb1b03759b2637927281c04d989e6c8fef888cae"),
        }
        for distribution_id, digests in expected_digests.items():
            release = actual[distribution_id]["releases"][0]
            self.assertEqual(release["package_source"]["revision"], "2ddbb99dd190c1792b79904f9875e6322bccd243")
            self.assertEqual((release["tree_digest"], release["manifest_digest"]), digests)
            self.assertEqual(release["published_at"], "2026-08-09T22:28:33Z")

    def test_migration_preserves_exact_package_bytes_and_provenance(self) -> None:
        source = self.source()
        for distribution in source["distributions"]:
            for policy in distribution["release_policies"]:
                self.assertEqual(policy["minimum_installer_version"], registry.DIRECTORY_MINIMUM_INSTALLER_VERSION)
            release = distribution["releases"][0]
            if release["package_source"]["revision"] is not None:
                continue
            root = registry.ROOT / release["package_source"]["path"]
            self.assertEqual(release["tree_digest_algorithm"], "agentplugins-tree-sha256-v1")
            self.assertEqual(release["tree_digest"], registry.directory_tree_digest(root))
            self.assertEqual(release["manifest_digest"], registry.digest_bytes((root / "plugin.json").read_bytes()))

    def test_real_bridge_defaults_qualified_history_and_fail_closed_npx_resolution(self) -> None:
        source = self.source()
        expected_defaults = {
            "cloudflare-docs": "777genius/cloudflare-docs-bridge",
            "github": "777genius/github-bridge",
        }
        for product, bridge in expected_defaults.items():
            self.assertEqual(registry.resolve_directory(source, product, ["codex"])["distribution_id"], bridge)
            legacy = f"777genius/{product}"
            self.assertEqual(registry.resolve_directory(source, legacy, ["codex"])["distribution_id"], legacy)
        with self.assertRaisesRegex(registry.RegistryError, r"chrome-devtools: no distribution supports"):
            registry.resolve_directory(source, "chrome-devtools", ["codex"])
        for distribution_id in ("777genius/chrome-devtools", "777genius/chrome-devtools-bridge"):
            with self.assertRaisesRegex(registry.RegistryError, rf"{distribution_id}: distribution is suspended"):
                registry.resolve_directory(source, distribution_id, ["codex"])
        with self.assertRaisesRegex(registry.RegistryError, r"context7: no distribution supports"):
            registry.resolve_directory(source, "context7", ["codex"])
        with self.assertRaisesRegex(registry.RegistryError, r"upstash/context7: .* evidence .* for codex"):
            registry.resolve_directory(source, "upstash/context7", ["codex"])

        context7 = next(
            product for product in registry.directory_preview(source)["products"]
            if product["id"] == "context7"
        )
        self.assertTrue(all(
            not distribution["eligible_targets"]
            for distribution in context7["distributions"]
        ))

    def test_both_chrome_distributions_are_suspended_and_all_policies_revoked(self) -> None:
        source = self.source()
        chrome_ids = {"777genius/chrome-devtools", "777genius/chrome-devtools-bridge"}
        chrome = {
            item["id"]: item for item in source["distributions"]
            if item["id"] in chrome_ids
        }
        self.assertEqual(set(chrome), chrome_ids)
        for distribution in chrome.values():
            self.assertEqual(distribution["status"], "suspended")
            self.assertTrue(distribution["release_policies"])
            self.assertEqual(
                {policy["status"] for policy in distribution["release_policies"]},
                {"revoked"},
            )

    def test_active_non_bridge_live_npx_distribution_is_rejected(self) -> None:
        source = self.source()
        distribution = next(
            item for item in source["distributions"]
            if item["id"] == "777genius/chrome-devtools"
        )
        self.assertEqual(distribution["kind"], "community")
        distribution["status"] = "active"
        distribution["release_policies"][0]["status"] = "active"
        distribution["releases"][0]["package_source"]["revision"] = None
        with self.assertRaisesRegex(
            registry.RegistryError,
            "active in-repository release uses live npx without a recognized content-addressed runtime closure contract",
        ):
            registry.validate_active_local_runtime_closures(source)

    def test_bound_historical_release_is_not_checked_against_current_path(self) -> None:
        source = self.source()
        distribution = next(
            item for item in source["distributions"]
            if item["id"] == "777genius/chrome-devtools"
        )
        distribution["status"] = "active"
        distribution["release_policies"][0]["status"] = "active"
        self.assertIsNotNone(distribution["releases"][0]["package_source"]["revision"])
        registry.validate_active_local_runtime_closures(source)

    def test_real_bridge_and_upstream_context7_provenance_is_exact(self) -> None:
        source = self.source()
        distributions = {item["id"]: item for item in source["distributions"]}
        expected = {
            "777genius/chrome-devtools-bridge": ("ChromeDevTools/chrome-devtools-mcp", "774d78f5eef5e610407a0c92fa6ec5ed74b027e8"),
            "777genius/cloudflare-docs-bridge": ("cloudflare/mcp-server-cloudflare", "0c51a6fbcf9a2fae80120287e8238fb947cdc2df"),
            "777genius/github-bridge": ("github/github-mcp-server", "fcdd664099f957c4a7dc183d9381cef191e8c8a9"),
        }
        for distribution_id, provenance in expected.items():
            release = distributions[distribution_id]["releases"][0]
            self.assertEqual((release["build_provenance"]["upstream_repository"], release["build_provenance"]["upstream_revision"]), provenance)
            self.assertIsNone(release["package_source"]["revision"])
        context7 = distributions["upstash/context7"]["releases"][0]
        self.assertEqual(context7["package_source"], {"repository": "upstash/context7", "revision": "769c6cd22c3d95462d1f55d789e9532cabefa5a9", "path": "plugins/agent-plugins/context7"})
        self.assertEqual(context7["tree_digest"], "sha256:08eed3b67f2e71a11b68baa594380c2f69ec1bc97584d701deaf7942ac34c0d8")
        self.assertEqual(context7["manifest_digest"], "sha256:d01781acd899aefa9445a290cf43a481230321934d62f9c8a2aab06a89718236")

    def bridge_reports(self):
        import build_bridges

        reports = []
        for bridge_id in build_bridges.recipe_ids(registry.ROOT):
            _path, recipe = build_bridges.load_recipe(registry.ROOT, bridge_id)
            package = registry.ROOT / recipe["output"]
            inventory = build_bridges.validate_components(package, recipe)
            reports.append({
                "bridge_id": bridge_id,
                "product_id": recipe["product_id"],
                "distribution_id": recipe["distribution_id"],
                "package_path": recipe["output"],
                "overlay_path": f"bridges/{bridge_id}/{recipe['overlay']}",
                "upstream_repository": recipe["upstream"]["repository"],
                "upstream_revision": recipe["upstream"]["revision"],
                "manifest_digest": registry.digest_bytes((package / "plugin.json").read_bytes()),
                "tree_digest_algorithm": registry.DIRECTORY_TREE_DIGEST_ALGORITHM,
                "tree_digest": registry.directory_tree_digest(package),
                "components": inventory,
            })
        return reports

    def test_bridge_release_recipe_and_build_report_bind_every_security_field(self) -> None:
        source = self.source()
        reports = self.bridge_reports()
        registry.validate_bridge_bindings(source, build_reports=reports)
        bridges = [item for item in source["distributions"] if item["kind"] == "community_bridge"]
        first = bridges[0]
        mutations = [
            ("product identity", lambda distribution, release: distribution.__setitem__("product_id", "cloudflare-docs")),
            ("distribution identity", lambda distribution, release: distribution.__setitem__("id", "777genius/not-the-recipe")),
            ("package output", lambda distribution, release: release["package_source"].__setitem__("path", "plugins/cloudflare-docs")),
            ("upstream repository", lambda distribution, release: release["build_provenance"].__setitem__("upstream_repository", "example/wrong")),
            # Exact review failure probe: only the claimed revision changes;
            # recipe and committed package bytes remain untouched.
            ("upstream revision", lambda distribution, release: release["build_provenance"].__setitem__("upstream_revision", "0" * 40)),
            ("components", lambda distribution, release: release.__setitem__("components", ["mcp", "skills"])),
            ("manifest digest", lambda distribution, release: release.__setitem__("manifest_digest", "sha256:" + "0" * 64)),
            ("tree algorithm", lambda distribution, release: release.__setitem__("tree_digest_algorithm", "wrong")),
            ("tree digest", lambda distribution, release: release.__setitem__("tree_digest", "sha256:" + "0" * 64)),
        ]
        for label, mutate in mutations:
            changed = copy.deepcopy(source)
            distribution = next(item for item in changed["distributions"] if item["id"] == first["id"])
            mutate(distribution, distribution["releases"][0])
            with self.subTest(label=label), self.assertRaises(registry.RegistryError):
                registry.validate_bridge_bindings(changed, build_reports=reports)

        with self.assertRaisesRegex(registry.RegistryError, "reports.*one-for-one"):
            registry.validate_bridge_bindings(source, build_reports=reports[:-1])
        with self.assertRaisesRegex(registry.RegistryError, "duplicate bridge build report"):
            registry.validate_bridge_bindings(source, build_reports=[*reports, copy.deepcopy(reports[0])])
        for field in (
            "product_id", "package_path", "overlay_path", "upstream_repository", "upstream_revision",
            "manifest_digest", "tree_digest_algorithm", "tree_digest", "components",
        ):
            changed_reports = copy.deepcopy(reports)
            report = changed_reports[0]
            report[field] = [] if field == "components" else "wrong"
            with self.subTest(report_field=field), self.assertRaisesRegex(registry.RegistryError, f"report {field} mismatch"):
                registry.validate_bridge_bindings(source, build_reports=changed_reports)

    def test_bridge_binding_uses_newest_release_without_revalidating_historical_bytes(self) -> None:
        source = self.source()
        reports = self.bridge_reports()
        distribution = next(item for item in source["distributions"] if item["kind"] == "community_bridge")
        historical = copy.deepcopy(distribution["releases"][0])
        historical["package_source"]["revision"] = "1" * 40
        historical["manifest_digest"] = "sha256:" + "1" * 64
        historical["tree_digest"] = "sha256:" + "2" * 64
        historical["build_provenance"]["upstream_revision"] = "3" * 40
        current = copy.deepcopy(distribution["releases"][0])
        current["sequence"] = historical["sequence"] + 1
        distribution["releases"] = [historical, current]

        registry.validate_bridge_bindings(source, build_reports=reports)

        historical["package_source"]["revision"] = None
        with self.assertRaisesRegex(
            registry.RegistryError,
            "only the newest bridge release may await revision binding",
        ):
            registry.validate_bridge_bindings(source, build_reports=reports)

    def test_bridge_recipe_set_rejects_missing_orphan_and_duplicate(self) -> None:
        source = self.source()
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            shutil.copytree(registry.ROOT / "bridges", root / "bridges")
            shutil.rmtree(root / "bridges" / "github")
            with self.assertRaisesRegex(registry.RegistryError, "one-for-one"):
                registry.validate_bridge_bindings(source, repository_root=root)

        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            shutil.copytree(registry.ROOT / "bridges", root / "bridges")
            recipe = yaml.safe_load((root / "bridges" / "github" / "bridge.yaml").read_text())
            recipe["product_id"] = "orphan"
            recipe["distribution_id"] = "777genius/orphan-bridge"
            recipe["output"] = "plugins/orphan"
            orphan = root / "bridges" / "orphan"
            orphan.mkdir()
            (orphan / "bridge.yaml").write_text(yaml.safe_dump(recipe, sort_keys=False))
            with self.assertRaisesRegex(registry.RegistryError, "one-for-one"):
                registry.validate_bridge_bindings(source, repository_root=root)

            recipe["distribution_id"] = "777genius/github-bridge"
            (orphan / "bridge.yaml").write_text(yaml.safe_dump(recipe, sort_keys=False))
            with self.assertRaisesRegex(registry.RegistryError, "duplicate canonical bridge recipe"):
                registry.validate_bridge_bindings(source, repository_root=root)

    def test_distribution_kind_and_evidence_contract_fixture(self) -> None:
        fixture = self.fixture()
        registry.validate_directory(fixture, verify_packages=False)
        self.assertEqual({item["kind"] for item in fixture["distributions"]}, {"upstream", "community_bridge", "community"})
        bridge = next(item for item in fixture["distributions"] if item["kind"] == "community_bridge")
        self.assertEqual(bridge["releases"][0]["package_source"]["revision"], "b" * 40)
        self.assertIn("build_provenance", bridge["releases"][0])
        self.assertEqual(bridge["release_policies"][0]["current_evidence"], ["evidence/demo-bridge-runtime"])

    def test_target_authentication_is_required_and_enum_validated(self) -> None:
        fixture = self.fixture()
        target = fixture["distributions"][0]["release_policies"][0]["targets"][0]
        del target["authentication"]
        with self.assertRaisesRegex(registry.RegistryError, r"'authentication' is a required property"):
            registry.validate_directory(fixture, verify_packages=False)

        fixture = self.fixture()
        target = fixture["distributions"][0]["release_policies"][0]["targets"][0]
        target["authentication"] = "sometimes"
        with self.assertRaisesRegex(registry.RegistryError, r"'sometimes' is not one of"):
            registry.validate_directory(fixture, verify_packages=False)

    def test_unknown_authentication_is_honest_and_retained_in_preview(self) -> None:
        fixture = self.fixture()
        registry.validate_directory(fixture, verify_packages=False)
        preview = registry.directory_preview(fixture)
        community = next(
            item
            for item in preview["products"][0]["distributions"]
            if item["id"] == "community/demo"
        )
        self.assertEqual(
            community["eligible_targets"],
            [{"client": "codex", "authentication": "unknown"}],
        )

    def test_evidence_level_and_outcome_do_not_become_authentication(self) -> None:
        fixture = self.fixture()
        observation = fixture["evidence"][0]
        observation["level"] = "oauth"
        observation["outcome"] = "inconclusive"
        registry.validate_directory(fixture, verify_packages=False)
        preview = registry.directory_preview(fixture)
        bridge = next(
            item
            for item in preview["products"][0]["distributions"]
            if item["id"] == "packager/demo-bridge"
        )
        self.assertEqual(
            bridge["eligible_targets"],
            [
                {"client": "codex", "authentication": "required"},
                {"client": "cursor", "authentication": "required"},
            ],
        )

    def test_declared_default_and_candidate_do_not_implicitly_promote(self) -> None:
        fixture = self.fixture()
        result = registry.resolve_directory(fixture, "demo", ["codex", "cursor"])
        self.assertEqual(result["distribution_id"], "packager/demo-bridge")
        self.assertIsNone(result["fallback_reason"])
        qualified = registry.resolve_directory(fixture, "packager/demo-bridge", ["cursor"])
        self.assertEqual(qualified["release_sequence"], 3)
        with self.assertRaisesRegex(registry.RegistryError, "candidate"):
            registry.resolve_directory(fixture, "upstream/demo", ["cursor"])

    def promote_upstream(self, fixture, evidence_targets=(), *, point_to_evidence=True, set_default=True):
        product = fixture["products"][0]
        if set_default:
            product["default_distribution"] = "upstream/demo"
        upstream = next(item for item in fixture["distributions"] if item["id"] == "upstream/demo")
        upstream["status"] = "active"
        release = upstream["releases"][0]
        evidence_ids = []
        for target in evidence_targets:
            evidence_id = f"evidence/upstream-demo-materialization-{target}"
            evidence_ids.append(evidence_id)
            fixture["evidence"].append({
                "schema_version": 1,
                "id": evidence_id,
                "product_id": upstream["product_id"],
                "distribution_id": upstream["id"],
                "release_sequence": release["sequence"],
                "package_tree_digest": release["tree_digest"],
                "manifest_digest": release["manifest_digest"],
                "source_repository": release["package_source"]["repository"],
                "source_revision": release["package_source"]["revision"],
                "source_path": release["package_source"]["path"],
                "level": "materialization",
                "outcome": "passed",
                "client": target,
                "client_version": "1.0.0",
                "installer_version": "0.1.6",
                "os": "linux",
                "architecture": "amd64",
                "observed_at": "2026-08-20T00:00:00Z",
                "artifact": {
                    "repository": "upstream/evidence",
                    "revision": "8" * 40,
                    "path": f"evidence/{target}.json",
                    "digest": "sha256:" + "8" * 64,
                },
            })
        fixture["evidence"].sort(key=lambda item: item["id"])
        if point_to_evidence:
            upstream["release_policies"][0]["current_evidence"] = sorted(evidence_ids)
        return upstream

    def test_upstream_default_rejects_empty_and_stale_positive_evidence(self) -> None:
        fixture = self.fixture()
        self.promote_upstream(fixture)
        with self.assertRaisesRegex(
            registry.RegistryError,
            r"upstream default upstream/demo@1 lacks current positive package compatibility evidence .* codex,cursor",
        ):
            registry.validate_directory(fixture, verify_packages=False)

        fixture = self.fixture()
        self.promote_upstream(fixture, ("codex", "cursor"), point_to_evidence=False)
        with self.assertRaisesRegex(registry.RegistryError, r"evidence .* codex,cursor"):
            registry.validate_directory(fixture, verify_packages=False)

    def test_upstream_default_rejects_wrong_target_evidence(self) -> None:
        fixture = self.fixture()
        self.promote_upstream(fixture, ("cursor",))
        with self.assertRaisesRegex(registry.RegistryError, r"evidence .* targets: codex$"):
            registry.validate_directory(fixture, verify_packages=False)

    def test_upstream_default_accepts_exact_current_static_compatibility_evidence(self) -> None:
        fixture = self.fixture()
        self.promote_upstream(fixture, ("codex", "cursor"))
        registry.validate_directory(fixture, verify_packages=False)
        self.assertEqual(registry.resolve_directory(fixture, "demo", ["codex", "cursor"])["distribution_id"], "upstream/demo")

    def test_unqualified_fallback_skips_upstream_without_selected_target_evidence(self) -> None:
        fixture = self.fixture()
        fixture["products"][0]["default_distribution"] = "community/demo"
        self.promote_upstream(fixture, set_default=False)
        result = registry.resolve_directory(fixture, "demo", ["codex", "cursor"])
        self.assertEqual(result["distribution_id"], "packager/demo-bridge")
        self.assertEqual(
            result["fallback_reason"],
            "declared default community/demo was ineligible: release 1 does not support cursor",
        )

    def test_unqualified_fallback_selects_upstream_with_exact_target_evidence(self) -> None:
        fixture = self.fixture()
        fixture["products"][0]["default_distribution"] = "community/demo"
        self.promote_upstream(fixture, ("codex", "cursor"), set_default=False)
        result = registry.resolve_directory(fixture, "demo", ["codex", "cursor"])
        self.assertEqual((result["distribution_id"], result["release_sequence"]), ("upstream/demo", 1))
        self.assertEqual(
            result["fallback_reason"],
            "declared default community/demo was ineligible: release 1 does not support cursor",
        )

    def test_qualified_upstream_selection_requires_evidence_for_selected_targets_only(self) -> None:
        fixture = self.fixture()
        self.promote_upstream(fixture, ("cursor",), set_default=False)
        selected = registry.resolve_directory(fixture, "upstream/demo", ["cursor"])
        self.assertEqual((selected["distribution_id"], selected["release_sequence"]), ("upstream/demo", 1))
        with self.assertRaisesRegex(
            registry.RegistryError,
            r"upstream/demo: release 1 lacks current positive package compatibility evidence .* for codex$",
        ):
            registry.resolve_directory(fixture, "upstream/demo", ["codex"])

    def test_upstream_selection_rejects_stale_release_tuple_evidence(self) -> None:
        fixture = self.fixture()
        upstream = self.promote_upstream(fixture, ("cursor",), set_default=False)
        evidence_id = upstream["release_policies"][0]["current_evidence"][0]
        observation = next(item for item in fixture["evidence"] if item["id"] == evidence_id)
        observation["release_sequence"] = 2
        with self.assertRaisesRegex(
            registry.RegistryError,
            r"upstream/demo: release 1 lacks current positive package compatibility evidence .* for cursor$",
        ):
            registry.resolve_directory(fixture, "upstream/demo", ["cursor"])

        observation["release_sequence"] = 1
        observation["package_tree_digest"] = "sha256:" + "0" * 64
        with self.assertRaisesRegex(registry.RegistryError, r"evidence .* for cursor$"):
            registry.resolve_directory(fixture, "upstream/demo", ["cursor"])

        for field, value in (("distribution_id", "other/demo"), ("outcome", "failed"), ("client", "codex")):
            fixture = self.fixture()
            upstream = self.promote_upstream(fixture, ("cursor",), set_default=False)
            evidence_id = upstream["release_policies"][0]["current_evidence"][0]
            observation = next(item for item in fixture["evidence"] if item["id"] == evidence_id)
            observation[field] = value
            with self.subTest(field=field), self.assertRaisesRegex(registry.RegistryError, r"cursor$"):
                registry.resolve_directory(fixture, "upstream/demo", ["cursor"])

    def test_contribution_guide_keeps_alias_and_provenance_contract(self) -> None:
        root = MODULE_PATH.parents[1]
        registry_guide = (root / "registry" / "README.md").read_text()
        contributing = (root / "CONTRIBUTING.md").read_text()
        self.assertEqual(registry_guide.count("## Submit an external package"), 1)
        self.assertNotIn("registry/entries", registry_guide + contributing)
        self.assertIn("registry/directory.json", registry_guide)
        self.assertIn("registry/directory.json", contributing)
        self.assertNotIn("external package never", registry_guide.lower())
        for contract in (
            "Every accepted product",
            "at least one globally unique short-name",
            "An alias identifies the product",
            "kind: upstream",
            "plugin.json` physically exists",
            "Directory acceptance does not make either one official upstream software",
            "Active and historical product aliases stay reserved",
            "A proposal that collides",
            "reassigns an alias",
            "Accepting an external distribution does not make it the default",
        ):
            with self.subTest(contract=contract):
                self.assertIn(contract, registry_guide)

    def test_fallback_uses_one_distribution_for_the_complete_target_set(self) -> None:
        fixture = self.fixture()
        product = fixture["products"][0]
        product["default_distribution"] = "community/demo"
        result = registry.resolve_directory(fixture, "demo", ["codex", "cursor"])
        self.assertEqual(result["distribution_id"], "packager/demo-bridge")
        self.assertIn("does not support cursor", result["fallback_reason"])
        bridge = next(item for item in fixture["distributions"] if item["id"] == "packager/demo-bridge")
        bridge["release_policies"][0]["targets"] = bridge["release_policies"][0]["targets"][:1]
        with self.assertRaisesRegex(registry.RegistryError, "complete target set"):
            registry.resolve_directory(fixture, "demo", ["codex", "cursor"])

    def test_policy_is_mutable_without_changing_release_identity_or_bytes(self) -> None:
        fixture = self.fixture()
        bridge = next(item for item in fixture["distributions"] if item["id"] == "packager/demo-bridge")
        release_before = copy.deepcopy(bridge["releases"])
        bridge["release_policies"][0]["minimum_installer_version"] = "0.2.0"
        bridge["release_policies"][0]["current_evidence"] = []
        registry.validate_directory(fixture, verify_packages=False)
        self.assertEqual(bridge["releases"], release_before)
        self.assertEqual((bridge["id"], bridge["releases"][0]["sequence"]), ("packager/demo-bridge", 3))

    def test_current_trusted_failure_blocks_only_its_applicable_client(self) -> None:
        fixture = self.fixture()
        fixture["evidence"][0]["outcome"] = "failed"
        self.assertEqual(registry.resolve_directory(fixture, "demo", ["codex"])["distribution_id"], "packager/demo-bridge")
        with self.assertRaisesRegex(registry.RegistryError, "blocking trusted failure for cursor"):
            registry.resolve_directory(fixture, "demo", ["cursor"])

    def test_preview_records_complete_multi_target_release_eligibility(self) -> None:
        fixture = self.fixture()
        distribution = next(
            item for item in fixture["distributions"]
            if item["id"] == "packager/demo-bridge"
        )
        newest = copy.deepcopy(distribution["releases"][0])
        newest["sequence"] = 4
        distribution["releases"].append(newest)
        newest_policy = copy.deepcopy(distribution["release_policies"][0])
        newest_policy["release_sequence"] = 4
        newest_policy["targets"][0]["authentication"] = "not_required"
        newest_policy["targets"] = newest_policy["targets"][:1]
        newest_policy["current_evidence"] = []
        distribution["release_policies"].append(newest_policy)

        registry.validate_directory(fixture, verify_packages=False)
        resolved = registry.resolve_directory(fixture, "packager/demo-bridge", ["codex", "cursor"])
        self.assertEqual(resolved["release_sequence"], 3)
        preview = registry.directory_preview(fixture)
        registry._validate_document(
            preview, "directory-preview.schema.json", "fixture review preview",
        )
        older = next(
            item for item in preview["products"][0]["distributions"]
            if item["id"] == "packager/demo-bridge" and item["release_sequence"] == 3
        )
        newer = next(
            item for item in preview["products"][0]["distributions"]
            if item["id"] == "packager/demo-bridge" and item["release_sequence"] == 4
        )
        self.assertEqual(
            older["eligible_targets"],
            [
                {"client": "codex", "authentication": "required"},
                {"client": "cursor", "authentication": "required"},
            ],
        )
        self.assertEqual(
            newer["eligible_targets"],
            [{"client": "codex", "authentication": "not_required"}],
        )
        self.assertEqual(
            {(item["id"], item["release_sequence"]) for item in (older, newer)},
            {("packager/demo-bridge", 3), ("packager/demo-bridge", 4)},
        )
        resolutions = preview["products"][0]["target_resolutions"]
        codex = next(item for item in resolutions if item["targets"] == [
            {"client": "codex", "authentication": "not_required"},
        ])
        codex_cursor = next(item for item in resolutions if item["targets"] == [
            {"client": "codex", "authentication": "required"},
            {"client": "cursor", "authentication": "required"},
        ])
        self.assertEqual(
            (codex["distribution_id"], codex["release_sequence"]),
            ("packager/demo-bridge", 4),
        )
        self.assertNotIn("fallback_reason", codex)
        self.assertEqual(
            (codex_cursor["distribution_id"], codex_cursor["release_sequence"]),
            ("packager/demo-bridge", 3),
        )

    def test_preview_records_exact_fallback_target_set_resolution(self) -> None:
        fixture = self.fixture()
        fixture["products"][0]["default_distribution"] = "community/demo"

        preview = registry.directory_preview(fixture)
        resolution = next(
            item for item in preview["products"][0]["target_resolutions"]
            if item["targets"] == [
                {"client": "codex", "authentication": "required"},
                {"client": "cursor", "authentication": "required"},
            ]
        )
        self.assertEqual(
            (resolution["distribution_id"], resolution["release_sequence"]),
            ("packager/demo-bridge", 3),
        )
        self.assertEqual(
            resolution["fallback_reason"],
            "declared default community/demo was ineligible: release 1 does not support cursor",
        )

    def test_release_sequences_and_alias_reservations_are_enforced(self) -> None:
        fixture = self.fixture()
        duplicate = copy.deepcopy(fixture["distributions"][0]["releases"][0])
        fixture["distributions"][0]["releases"].append(duplicate)
        fixture["distributions"][0]["release_policies"].append(copy.deepcopy(fixture["distributions"][0]["release_policies"][0]))
        with self.assertRaisesRegex(registry.RegistryError, "release sequences"):
            registry.validate_directory(fixture, verify_packages=False)
        fixture = self.fixture()
        fixture["products"][0]["aliases"] = ["unreserved"]
        with self.assertRaisesRegex(registry.RegistryError, "remain reserved"):
            registry.validate_directory(fixture, verify_packages=False)

    def test_preview_and_search_are_deterministic_and_have_one_product_card(self) -> None:
        source = self.source()
        preview = registry.encoded(registry.directory_preview(source))
        search = registry.encoded(registry.directory_search(source))
        self.assertEqual(preview, registry.REVIEW_PREVIEW.read_bytes())
        self.assertEqual(search, registry.REVIEW_SEARCH.read_bytes())
        document = json.loads(preview)
        self.assert_one_card_per_product(source)
        self.assertNotIn("snapshot_sequence", document)
        self.assertNotIn("expires_at", document)

    def test_valid_external_product_preserves_count_and_unique_card_contract(self) -> None:
        committed_source = self.source()
        source = self.external_product_source()
        self.assertEqual(len(source["products"]), len(committed_source["products"]) + 1)
        self.assertEqual(len(source["distributions"]), len(committed_source["distributions"]) + 1)
        registry.validate_directory(source)
        self.assert_canonical_products_present_once(source)
        self.assert_one_card_per_product(source)

    def test_removing_or_duplicating_a_canonical_product_breaks_the_contract(self) -> None:
        for mutation in ("remove", "duplicate"):
            with self.subTest(mutation=mutation):
                source = copy.deepcopy(self.source())
                canonical = next(item for item in source["products"] if item["id"] == "context7")
                if mutation == "remove":
                    source["products"].remove(canonical)
                else:
                    source["products"].append(copy.deepcopy(canonical))
                with self.assertRaises(AssertionError):
                    self.assert_canonical_products_present_once(source)

    def test_current_source_covers_authentication_for_every_target(self) -> None:
        source = self.source()
        expected_not_required = {
            "agent-code-navigator",
            "chrome-devtools",
            "cloudflare-docs",
            "context7",
            "docker-hub",
        }
        targets = [
            (distribution, target)
            for distribution in source["distributions"]
            if distribution["product_id"] in CANONICAL_PRODUCT_IDS
            for policy in distribution["release_policies"]
            for target in policy["targets"]
        ]
        self.assertTrue(targets)
        for distribution, target in targets:
            expected = "not_required" if distribution["product_id"] in expected_not_required else "required"
            self.assertEqual(target["authentication"], expected, (distribution["id"], target["client"]))

        notion = [target for distribution, target in targets if distribution["product_id"] == "notion"]
        self.assertTrue(notion)
        self.assertEqual({target["authentication"] for target in notion}, {"required"})
        chatgpt_docs = [
            target
            for distribution, target in targets
            if distribution["product_id"] == "cloudflare-docs" and target["client"] == "chatgpt"
        ]
        self.assertTrue(chatgpt_docs)
        self.assertEqual({target["authentication"] for target in chatgpt_docs}, {"not_required"})

    def test_direct_sources_are_recognized_without_directory_resolution(self) -> None:
        sha = "a" * 40
        self.assertTrue(registry.is_direct_source("./plugin"))
        self.assertTrue(registry.is_direct_source(f"owner/repo@{sha}//plugin"))
        self.assertFalse(registry.is_direct_source("context7"))
        self.assertFalse(registry.is_direct_source("owner/repo@main//plugin"))

    def test_legacy_catalogs_and_package_readme_blocks_are_frozen_or_valid(self) -> None:
        registry.validate_legacy_catalog_freeze()
        registry.validate_readme_blocks(self.source())

    def test_public_outputs_never_use_registry_v3_branding(self) -> None:
        for path in [registry.DIRECTORY_SOURCE, registry.REVIEW_PREVIEW, registry.REVIEW_SEARCH]:
            prohibited = "registry " + "v3"
            self.assertNotIn(prohibited, path.read_text(encoding="utf-8").casefold())


if __name__ == "__main__":
    unittest.main()
