#!/usr/bin/env python3
"""Pinned read-only Cline config research on disposable hosted Linux only.

No prompt/session, installer/updater, trust, account, MCP connection or payload
is requested. Collected JSON still requires independent evidence interpretation.
"""
import argparse
import gzip
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path, PurePosixPath
import re
import sys
import tarfile
import tempfile
import urllib.request

from native_client_research import bounded_read, run_probe


def adjacent_module(name, filename):
    spec = importlib.util.spec_from_file_location(name, Path(__file__).with_name(filename))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


native = adjacent_module("cline_research_matrix", "run-native-client-matrix.py")
grok = adjacent_module("cline_research_bounds", "research-grok-native.py")
VERSION = "3.0.73"
URL = "https://registry.npmjs.org/@cline/cli-linux-x64/-/cli-linux-x64-3.0.73.tgz"
ARCHIVE_PIN = "sha512-K+T94s0kPHLoMSANOJJ7KHb/etzYXZGQUbarKDKUPoZsvy4+oD1MAqYnZlfbKFM8tpnRQWouFngD9TB3mFqqeA=="
BINARY_SHA256 = "1ff8841d17d4d83763134c193d414e4f7fcf2ef56af759e5ebbe96727fc2c8f7"
BINARY_BYTES = 132_511_200
MAX_ARCHIVE, MAX_EXPANDED, MAX_BINARY, MAX_MEMBERS = 64 << 20, 384 << 20, 256 << 20, 256
PROJECTS = ("project-a", "project-b")
HELP_PROBES = (("version", ("--version",)), ("help", ("--help",)),
               ("config-help", ("config", "--help")), ("mcp-help", ("mcp", "--help")))
MCP_PROBES = (("default-a", "project-a", None), ("default-b", "project-b", None),
              ("override-a", "project-a", "project-a"), ("override-b", "project-b", "project-b"),
              ("cross-a-from-b", "project-b", "project-a"), ("default-a-again", "project-a", None))
EXPECTED_PROBES = ([name for name, _ in HELP_PROBES]
                   + [name for name, _, _ in MCP_PROBES] + ["skills-a", "skills-b"])


def archive_binary(body, budget):
    with gzip.GzipFile(fileobj=io.BytesIO(body)) as compressed:
        expanded = bounded_read(grok.BudgetReader(compressed, budget), MAX_EXPANDED)
    binary, seen = None, set()
    with tarfile.open(fileobj=io.BytesIO(expanded), mode="r:") as archive:
        for count, member in enumerate(archive, 1):
            budget.check()
            name = member.name
            parts = PurePosixPath(name).parts
            if (count > MAX_MEMBERS or not parts or parts[0] != "package"
                    or len(os.fsencode(name)) > 4096 or name.rstrip("/") != "/".join(parts)
                    or name.casefold() in seen or "\\" in name
                    or any(not re.fullmatch(r"[A-Za-z0-9._-]+", part) or part in (".", "..") for part in parts)):
                raise ValueError("unsafe, duplicate or excessive archive member")
            seen.add(name.casefold())
            if member.isdir() and member.size == 0:
                continue
            if member.type not in (tarfile.REGTYPE, tarfile.AREGTYPE) or not 0 <= member.size <= MAX_BINARY:
                raise ValueError("archive member must be an ordinary bounded regular file")
            if name == "package/bin/cline":
                if binary is not None or member.size != BINARY_BYTES:
                    raise ValueError("unexpected Cline binary member size or duplicate")
                with archive.extractfile(member) as source:
                    binary = bounded_read(grok.BudgetReader(source, budget), MAX_BINARY)
                if len(binary) != member.size:
                    raise ValueError("truncated Cline binary")
    if binary is None:
        raise ValueError("missing exact package/bin/cline member")
    return binary


def acquire_binary(directory, budget):
    with urllib.request.urlopen(URL, timeout=min(90, budget.check())) as response:
        body = bounded_read(grok.BudgetReader(response, budget), MAX_ARCHIVE)
    native.verify_digest(body, ARCHIVE_PIN)
    binary = archive_binary(body, budget)
    actual = hashlib.sha256(binary).hexdigest()
    if actual != BINARY_SHA256:
        raise ValueError("Cline binary integrity mismatch")
    path = directory / "cline"
    path.write_bytes(binary)
    path.chmod(0o700)
    return path, {"url": URL, "version": VERSION, "archive_integrity": ARCHIVE_PIN,
                  "archive_bytes": len(body), "binary_sha256": actual, "binary_bytes": len(binary),
                  "installer_executed": False, "archived_assets_materialized": False}


def runtime_environment(home, temporary):
    env = native.profile_environment(home, "linux-amd64")
    env.update({"PATH": "/usr/bin:/bin", "TMPDIR": str(temporary), "LANG": "C.UTF-8",
                "CLINE_NO_AUTO_UPDATE": "1"})
    return env


def write_fixture(path, text):
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("x", encoding="utf-8") as output:
        output.write(text)
    path.chmod(0o600)


def prepare_fixtures(root):
    paths = {}
    for owner, name, transport in (("home", "uap-cline-user", "streamableHttp"),
                                    ("project-a", "uap-cline-a", "streamableHttp"),
                                    ("project-b", "uap-cline-b", "sse")):
        selected = root / owner
        settings = (selected / ".cline/data/settings/cline_mcp_settings.json" if owner == "home"
                    else selected / ".uap/clients/cline/mcp.json")
        document = {"mcpServers": {name: {"transport": {"type": transport,
                    "url": "http://127.0.0.1:9/" + name}, "disabled": True}}}
        write_fixture(settings, json.dumps(document, indent=2) + "\n")
        write_fixture(selected / ".cline/skills" / name / "SKILL.md",
                      f"---\nname: {name}\ndescription: Inert Cline source research marker\n---\n\nText only. No executable, hook, plugin, MCP command or task.\n")
        paths[owner] = settings
    return paths


def child_environment(base, paths, override):
    if base.get("CLINE_NO_AUTO_UPDATE") != "1" or "CLINE_MCP_SETTINGS_PATH" in base:
        raise ValueError("Cline requires explicit no-update and a clean base override environment")
    selected = dict(base)
    if override is not None:
        if override not in PROJECTS:
            raise ValueError("MCP override must select a known disposable project")
        selected["CLINE_MCP_SETTINGS_PATH"] = str(paths[override])
    return selected


def validate_probe(record, stdout, stderr, kind):
    if record["error"] or record["exit"] != 0:
        return False
    try:
        if kind == "version":
            return re.search(r"(?<![A-Za-z0-9_.+-])v?" + re.escape(VERSION) + r"(?![A-Za-z0-9_.+-])",
                             stdout.decode("utf-8")) is not None
        if kind == "json":
            value = json.loads(stdout.decode("utf-8"), object_pairs_hook=grok.unique_object,
                               parse_constant=grok.invalid_constant)
            return not stderr.strip() and isinstance(value, list) and all(isinstance(row, dict) for row in value)
        return bool(stdout.strip() or stderr.strip())
    except (UnicodeError, ValueError, RecursionError):
        return False


def collect(binary, root, base, output, summary, budget):
    roots = {name: root / name for name in ("home", *PROJECTS)}
    for project in PROJECTS:
        record, _, _ = run_probe(["/usr/bin/git", "init", "--quiet", "--template="],
                                 roots[project], base, output, "git-init-" + project,
                                 timeout=min(30, budget.check()))
        record["accepted"] = record["exit"] == 0 and not record["error"]
        summary["fixture_preparation"].append(record)
        if not record["accepted"]:
            raise ValueError("fresh project Git initialization failed")
    paths = prepare_fixtures(root)
    before = grok.inventory(roots, budget)
    baseline = {name: entry for name, entry in before.items() if entry["kind"] == "file"}

    def observe(label, arguments, project, kind, override=None):
        budget.check()
        env = child_environment(base, paths, override)
        argv = [str(binary), "--auto-approve", "false", "--cwd", str(roots[project]), *arguments]
        record, stdout, stderr = run_probe(argv, roots[project], env, output, label,
                                           timeout=min(30, budget.check()))
        record["accepted"] = validate_probe(record, stdout, stderr, kind)
        record["mcp_settings_override"] = env.get("CLINE_MCP_SETTINGS_PATH")
        record["automatic_update_disabled"] = env["CLINE_NO_AUTO_UPDATE"] == "1"
        summary["probes"].append(record)
        grok.require_baseline(grok.inventory(roots, budget), baseline)

    for label, arguments in HELP_PROBES:
        observe(label, arguments, "project-a", "version" if label == "version" else "help")
    for label, project, override in MCP_PROBES:
        observe(label, ("config", "mcp", "--json"), project, "json", override)
    for project, label in zip(PROJECTS, ("skills-a", "skills-b")):
        observe(label, ("config", "skills", "--json"), project, "json")
    after = grok.inventory(roots, budget)
    grok.require_baseline(after, baseline)
    summary["inventories"] = {"before": before, "after": after,
                               "native_created": {name: entry for name, entry in after.items() if name not in before}}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    try:
        native.require_hosted("linux-amd64")
    except RuntimeError as exc:
        print(str(exc)[:512], file=sys.stderr)
        return 2
    output = args.output.absolute()
    output.mkdir(parents=True, exist_ok=False)
    summary = {"evidence_kind": "cline-precode-read-only-config-observations", "version": VERSION,
               "target": "linux-amd64", "source_commit": os.environ.get("EXPECTED_COMMIT", ""),
               "expected_probes": EXPECTED_PROBES, "probes": [], "fixture_preparation": [], "inventories": {},
               "collection_complete": False, "project_source_qualified": False, "project_runtime_qualified": False,
               "interpretation": "Human review of populated native marker/path/override attribution required",
               "unobserved": ["UAP project installation/ownership", "ordinary project MCP startup",
                              "MCP initialize/tool result", "enabled skill execution", "IDE discovery",
                              "trust/model/account", "native restart"], "runtime_network_isolation_claimed": False}
    try:
        with grok.Budget() as budget, tempfile.TemporaryDirectory(prefix="cline-native-research-") as temporary:
            root = Path(temporary)
            for name in ("binary", "home", "tmp", *PROJECTS):
                (root / name).mkdir()
            binary, summary["acquisition"] = acquire_binary(root / "binary", budget)
            env = runtime_environment(root / "home", root / "tmp")
            summary["native_environment"] = env
            collect(binary, root, env, output, summary, budget)
            summary["collection_complete"] = ([row["probe"] for row in summary["probes"]] == EXPECTED_PROBES
                                                and all(row["accepted"] for row in summary["probes"]))
    except Exception as exc:
        summary["error"] = f"{type(exc).__name__}: {str(exc)[:512]}"
    (output / "cline-precode-research.json").write_text(json.dumps(summary, indent=2) + "\n", encoding="utf-8")
    return 0 if summary["collection_complete"] else 1


if __name__ == "__main__":
    sys.exit(main())
