#!/usr/bin/env python3
"""Collect pinned Grok project-source observations on disposable hosted Linux.

Text-only candidate fixtures and read-only native inventory cannot by themselves
qualify exact project ownership, installation, model use or MCP initialization.
"""
import argparse
import gzip
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import re
import signal
import stat
import sys
import tempfile
import time
import urllib.request

from native_client_research import bounded_read, run_probe

SPEC = importlib.util.spec_from_file_location("grok_research_matrix", Path(__file__).with_name("run-native-client-matrix.py"))
native = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(native)
VERSION = "1.0.67"
URL = "https://x.ai/cli/grok-1.0.67-linux-x86_64.gz"
ARCHIVE_PIN = "sha256:6bbea96cde807c13e4c66d9583d8acdd155e446d93131c2820999cf5523ed5a5"
BINARY_SHA256 = "ce33b1a29e37901f56fe59916cda7a1149e6fd377118f918e90d89f9abb52a51"
MAX_ARCHIVE, MAX_BINARY = 96 << 20, 256 << 20
MAX_ENTRIES, MAX_DEPTH, MAX_FILE, MAX_TOTAL = 4096, 32, 1 << 20, 64 << 20
MAX_METADATA = 4096
COLLECTOR_SECONDS = 8 * 60
PLUGIN_NAME = "uap-project-native-research"
STAGES = ("empty", "user", "project-a", "project-b")
PROJECTS = ("project-a", "project-b")
HELP_PROBES = (("version", ("--version",)), ("plugin-list-help", ("plugin", "list", "--help")),
               ("inspect-help", ("inspect", "--help")))
JSON_PROBES = (("list", ("plugin", "list", "--json")), ("inspect", ("inspect", "--json")))
EXPECTED_PROBES = ([name for name, _ in HELP_PROBES]
                   + [f"{stage}-{project}-{kind}" for stage in STAGES for project in PROJECTS for kind, _ in JSON_PROBES])


class CollectorDeadline(Exception):
    pass


class Budget:
    def __init__(self, seconds=COLLECTOR_SECONDS):
        self.deadline = time.monotonic() + seconds

    def check(self):
        remaining = self.deadline - time.monotonic()
        if remaining <= 0:
            raise CollectorDeadline("total collector deadline exceeded")
        return remaining

    def alarm(self, _signum, _frame):
        raise CollectorDeadline("total collector deadline exceeded")

    def __enter__(self):
        remaining = self.check()
        self.previous_handler = signal.signal(signal.SIGALRM, self.alarm)
        self.previous_timer = signal.setitimer(signal.ITIMER_REAL, remaining)
        return self

    def __exit__(self, *_args):
        signal.setitimer(signal.ITIMER_REAL, 0)
        signal.signal(signal.SIGALRM, self.previous_handler)
        if self.previous_timer[0]:
            signal.setitimer(signal.ITIMER_REAL, *self.previous_timer)


class BudgetReader:
    def __init__(self, source, budget):
        self.source, self.budget = source, budget

    def read(self, count):
        self.budget.check()
        return self.source.read(count)


def acquire_binary(directory, budget):
    with urllib.request.urlopen(URL, timeout=min(90, budget.check())) as response:
        body = bounded_read(BudgetReader(response, budget), MAX_ARCHIVE)
    native.verify_digest(body, ARCHIVE_PIN)
    with gzip.GzipFile(fileobj=io.BytesIO(body)) as compressed:
        binary = bounded_read(BudgetReader(compressed, budget), MAX_BINARY)
    actual = hashlib.sha256(binary).hexdigest()
    if not binary or actual != BINARY_SHA256:
        raise ValueError("Grok binary integrity mismatch")
    path = directory / "grok"
    path.write_bytes(binary)
    path.chmod(0o700)
    return path, {"url": URL, "version": VERSION, "archive_integrity": ARCHIVE_PIN,
                  "integrity_source": "measured official HTTPS versioned bytes; no separate vendor SHA256 published",
                  "archive_bytes": len(body), "binary_sha256": actual, "binary_bytes": len(binary),
                  "installer_executed": False}


def runtime_environment(home, temporary):
    env = native.profile_environment(home, "linux-amd64")
    env.update({"PATH": "/usr/bin:/bin", "TMPDIR": str(temporary), "LANG": "C.UTF-8"})
    return env


def regular_identity(info):
    return (info.st_dev, info.st_ino, info.st_mode, info.st_nlink, info.st_size,
            info.st_mtime_ns, info.st_ctime_ns)


def inventory(roots, budget):
    """Traverse descriptor-relative, no-follow paths; every read/count is bounded."""
    result, totals = {}, {"entries": 0, "bytes": 0}

    def walk(directory_fd, prefix, depth):
        budget.check()
        if depth > MAX_DEPTH:
            raise ValueError("inventory depth limit exceeded")
        names = []
        with os.scandir(directory_fd) as entries:
            for entry in entries:
                budget.check()
                totals["entries"] += 1
                if totals["entries"] > MAX_ENTRIES or len(os.fsencode(entry.name)) > MAX_METADATA:
                    raise ValueError("inventory entry/name limit exceeded")
                names.append(entry.name)
        for name in sorted(names):
            budget.check()
            path = prefix + "/" + name
            if len(os.fsencode(path)) > MAX_METADATA:
                raise ValueError("inventory path metadata limit exceeded")
            before = os.stat(name, dir_fd=directory_fd, follow_symlinks=False)
            mode = stat.S_IMODE(before.st_mode)
            record = {"mode": oct(mode), "size": before.st_size}
            if stat.S_ISDIR(before.st_mode):
                record["kind"] = "directory"
                child_fd = os.open(name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=directory_fd)
                try:
                    opened = os.fstat(child_fd)
                    if regular_identity(opened) != regular_identity(before):
                        raise ValueError("inventory directory changed before admission")
                    walk(child_fd, path, depth + 1)
                    if regular_identity(os.fstat(child_fd)) != regular_identity(opened):
                        raise ValueError("inventory directory changed during traversal")
                finally:
                    os.close(child_fd)
            elif stat.S_ISREG(before.st_mode):
                if before.st_nlink != 1 or before.st_size > MAX_FILE:
                    raise ValueError("inventory regular file alias/size refused")
                descriptor = os.open(name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=directory_fd)
                with os.fdopen(descriptor, "rb") as source:
                    opened = os.fstat(source.fileno())
                    if not stat.S_ISREG(opened.st_mode) or regular_identity(opened) != regular_identity(before):
                        raise ValueError("inventory file changed before admission")
                    body = bounded_read(BudgetReader(source, budget), min(MAX_FILE, MAX_TOTAL - totals["bytes"]))
                    totals["bytes"] += len(body)
                    if len(body) != before.st_size or regular_identity(os.fstat(source.fileno())) != regular_identity(before):
                        raise ValueError("inventory file changed during read")
                record.update({"kind": "file", "sha256": hashlib.sha256(body).hexdigest()})
            elif stat.S_ISLNK(before.st_mode):
                target = os.readlink(name, dir_fd=directory_fd)
                if len(os.fsencode(target)) > MAX_METADATA:
                    raise ValueError("inventory link metadata limit exceeded")
                record.update({"kind": "symlink", "target": target})
            else:
                record["kind"] = "special"
                record["type"] = oct(stat.S_IFMT(before.st_mode))
            after = os.stat(name, dir_fd=directory_fd, follow_symlinks=False)
            if regular_identity(after) != regular_identity(before):
                raise ValueError("inventory entry changed during observation")
            result[path] = record

    for label, root in roots.items():
        budget.check()
        descriptor = os.open(root, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
        try:
            walk(descriptor, label, 0)
        finally:
            os.close(descriptor)
    return result


def require_baseline(snapshot, baseline):
    for name, expected in baseline.items():
        if snapshot.get(name) != expected:
            raise ValueError("read-only observation changed fixture baseline: " + name[:MAX_METADATA])


def fixture(root, marker, version):
    """Only test-owned plain manifest/skill text; never adopt an existing package."""
    parent = root
    for name in (".grok", "plugins"):
        parent = parent / name
        try:
            parent.mkdir(mode=0o755)
        except FileExistsError:
            if not stat.S_ISDIR(parent.lstat().st_mode):
                raise ValueError("fixture parent is not a real directory")
    plugin = parent / PLUGIN_NAME
    plugin.mkdir(mode=0o755)
    manifest_dir, skill = plugin / ".grok-plugin", plugin / "skills" / marker
    manifest_dir.mkdir(mode=0o755)
    skill.mkdir(parents=True, mode=0o755)
    manifest = {"name": PLUGIN_NAME, "version": version, "description": "Inert text-only native research", "skills": "./skills/"}
    bodies = {manifest_dir / "plugin.json": json.dumps(manifest, indent=2) + "\n",
              skill / "SKILL.md": f"---\nname: {marker}\ndescription: Inert native discovery research marker\n---\n\nText only. No commands, hooks, MCP servers or executable payload.\n"}
    for path, text in bodies.items():
        with path.open("x", encoding="utf-8") as output:
            output.write(text)
        path.chmod(0o644)
    return tuple(bodies)


def unique_object(pairs):
    value = {}
    for key, entry in pairs:
        if key in value:
            raise ValueError("duplicate JSON object key")
        value[key] = entry
    return value


def invalid_constant(_value):
    raise ValueError("non-JSON numeric constant")


def validate_probe(record, stdout, stderr, kind):
    if record["error"] or record["exit"] != 0:
        return False
    try:
        if kind == "version":
            return re.search(r"(?<![A-Za-z0-9_.+-])v?" + re.escape(VERSION) + r"(?![A-Za-z0-9_.+-])",
                             stdout.decode("utf-8")) is not None
        if kind == "json":
            decoded = json.loads(stdout.decode("utf-8"), object_pairs_hook=unique_object, parse_constant=invalid_constant)
            return not stderr.strip() and isinstance(decoded, (list, dict))
        return bool(stdout.strip() or stderr.strip())
    except (UnicodeError, ValueError, RecursionError):
        return False


def collect(binary, root, env, output, summary, budget):
    roots = {name: root / name for name in ("home", *PROJECTS)}
    for project in PROJECTS:
        record, stdout, stderr = run_probe(["/usr/bin/git", "init", "--quiet", "--template="],
                                           roots[project], env, output, "git-init-" + project,
                                           timeout=min(30, budget.check()))
        record["accepted"] = record["exit"] == 0 and not record["error"]
        summary["fixture_preparation"].append(record)
        if not record["accepted"]:
            raise ValueError("fresh project Git initialization failed")
    baseline = {name: entry for name, entry in inventory(roots, budget).items() if entry["kind"] == "file"}

    def observe(label, arguments, project, kind):
        budget.check()
        record, stdout, stderr = run_probe([str(binary), *arguments], roots[project], env, output, label,
                                           timeout=min(30, budget.check()))
        record["accepted"] = validate_probe(record, stdout, stderr, kind)
        summary["probes"].append(record)

    for label, arguments in HELP_PROBES:
        observe(label, arguments, "project-a", "version" if label == "version" else "help")
    for stage in STAGES:
        budget.check()
        if stage != "empty":
            owner, marker, version = {"user": ("home", "uap-research-user", "0.9.0"),
                                      "project-a": ("project-a", "uap-research-a", "1.0.0"),
                                      "project-b": ("project-b", "uap-research-b", "2.0.0")}[stage]
            files = fixture(roots[owner], marker, version)
            snapshot = inventory(roots, budget)
            require_baseline(snapshot, baseline)
            for path in files:
                key = owner + "/" + path.relative_to(roots[owner]).as_posix()
                baseline[key] = snapshot[key]
        before = inventory(roots, budget)
        require_baseline(before, baseline)
        for project in PROJECTS:
            for kind, arguments in JSON_PROBES:
                observe(f"{stage}-{project}-{kind}", arguments, project, "json")
        after = inventory(roots, budget)
        require_baseline(after, baseline)
        summary["inventories"].append({"stage": stage, "before": before, "after": after,
                                       "native_created": {name: entry for name, entry in after.items() if name not in before}})


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
    summary = {"evidence_kind": "grok-precode-project-source-observations", "version": VERSION,
               "target": "linux-amd64", "source_commit": os.environ.get("EXPECTED_COMMIT", ""),
               "expected_probes": EXPECTED_PROBES, "probes": [], "fixture_preparation": [], "inventories": [],
               "collection_complete": False, "project_source_qualified": False,
               "interpretation": "Human review of actual source/version/marker attribution and A/B/user precedence required",
               "unobserved": ["UAP project installation/ownership", "disabled/alias/ancestor and Claude-compatible conflicts",
                              "skill execution", "MCP initialization/tool result", "model/account/trust", "native restart"],
               "runtime_network_isolation_claimed": False}
    try:
        with Budget() as budget, tempfile.TemporaryDirectory(prefix="grok-native-research-") as temporary:
            root = Path(temporary)
            for name in ("binary", "home", "tmp", *PROJECTS):
                (root / name).mkdir()
            binary, summary["acquisition"] = acquire_binary(root / "binary", budget)
            env = runtime_environment(root / "home", root / "tmp")
            summary["native_environment"] = env
            collect(binary, root, env, output, summary, budget)
            summary["collection_complete"] = ([record["probe"] for record in summary["probes"]] == EXPECTED_PROBES
                                                and all(record["accepted"] for record in summary["probes"]))
    except Exception as exc:
        summary["error"] = f"{type(exc).__name__}: {str(exc)[:512]}"
    (output / "grok-precode-research.json").write_text(json.dumps(summary, indent=2) + "\n", encoding="utf-8")
    return 0 if summary["collection_complete"] else 1


if __name__ == "__main__":
    sys.exit(main())
