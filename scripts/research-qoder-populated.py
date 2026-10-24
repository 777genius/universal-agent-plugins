#!/usr/bin/env python3
"""Finite pinned Qoder scope observations, solely on genuine hosted Linux.

This collector never qualifies ownership, loaded skills, trust, models or MCP.
Acquisition precedes one network-isolated, capability-free nonroot collection.
"""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import stat
import socket
import sys
import time

from native_client_research import bounded_read, run_probe


def load_sibling(name, filename):
    spec = importlib.util.spec_from_file_location(name, Path(__file__).with_name(filename))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


phase1 = load_sibling("qoder_scope_phase1", "research-qoder-native.py")
grok = load_sibling("qoder_scope_bounds", "research-grok-native.py")
VERSION = phase1.VERSION
PLUGIN_NAME = "uap-scope-research"
MAX_ENTRIES, MAX_DEPTH, MAX_FILE, MAX_TOTAL = 4096, 32, 1 << 20, 64 << 20
MAX_METADATA = 4096
BINARY_BYTES = 182982112
COLLECTOR_SECONDS = 8 * 60
STAGES = ("empty", "user-installed", "A-installed", "B-installed")
LOCATIONS = ("user-cwd", "project-A", "project-B")
HELP = (("version", ("--version",)),
        ("install-help", ("plugins", "install", "--help")),
        ("list-help", ("plugins", "list", "--help")),
        ("uninstall-help", ("plugins", "uninstall", "--help")))
HOST_KEYS = ("GITHUB_ACTIONS", "RUNNER_ENVIRONMENT", "AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED", "EXPECTED_COMMIT")
FALSE_CLAIMS = {name: False for name in ("project_ownership_qualified", "native_activation_qualified",
               "loaded_skills_qualified", "mcp_qualified", "trust_qualified", "model_qualified",
               "authentication_qualified", "native_session_restart_qualified")}


class IsolationError(ValueError):
    def __init__(self, message, facts):
        super().__init__(message)
        self.facts = facts


class InventoryError(ValueError):
    def __init__(self, message, partial):
        super().__init__(message)
        self.partial = dict(partial)


def strict_json(body):
    def unique(pairs):
        value = {}
        for key, item in pairs:
            if key in value:
                raise ValueError("duplicate JSON object key")
            value[key] = item
        return value
    return json.loads(body.decode("utf-8"), object_pairs_hook=unique,
                      parse_constant=lambda _: (_ for _ in ()).throw(ValueError("nonfinite JSON value")))


def identity(info):
    return (info.st_dev, info.st_ino, info.st_mode, info.st_nlink,
            info.st_size, info.st_mtime_ns, info.st_ctime_ns)


def inventory(roots, budget):
    """Descriptor-relative bounded reads; aliases and incomplete scans fail closed."""
    result, count, total = {}, 0, 0

    def walk(fd, prefix, depth):
        nonlocal count, total
        budget.check()
        if depth > MAX_DEPTH:
            raise ValueError("inventory depth limit exceeded")
        names = []
        with os.scandir(fd) as entries:
            for item in entries:
                budget.check()
                count += 1
                if count > MAX_ENTRIES or len(os.fsencode(item.name)) > MAX_METADATA:
                    raise ValueError("inventory entry/name bound exceeded")
                names.append(item.name)
        for name in sorted(names):
            budget.check()
            path = prefix + "/" + name
            if len(os.fsencode(path)) > MAX_METADATA:
                raise ValueError("inventory path bound exceeded")
            before = os.stat(name, dir_fd=fd, follow_symlinks=False)
            record = {"mode": oct(stat.S_IMODE(before.st_mode)), "size": before.st_size}
            result[path] = record
            if stat.S_ISDIR(before.st_mode):
                record["kind"] = "directory"
                child = os.open(name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=fd)
                try:
                    opened = os.fstat(child)
                    if identity(opened) != identity(before):
                        raise ValueError("directory changed before inventory admission")
                    walk(child, path, depth + 1)
                    if identity(os.fstat(child)) != identity(opened):
                        raise ValueError("directory changed during inventory")
                finally:
                    os.close(child)
            elif stat.S_ISREG(before.st_mode):
                record["kind"] = "file"
                if before.st_nlink != 1 or before.st_size > MAX_FILE or before.st_size > MAX_TOTAL - total:
                    raise ValueError("inventory file size/alias/aggregate bound exceeded")
                child = os.open(name, os.O_RDONLY | os.O_NONBLOCK | os.O_NOFOLLOW, dir_fd=fd)
                with os.fdopen(child, "rb") as source:
                    if identity(os.fstat(source.fileno())) != identity(before):
                        raise ValueError("file changed before inventory admission")
                    body = bounded_read(grok.BudgetReader(source, budget), min(MAX_FILE, MAX_TOTAL - total))
                    if len(body) != before.st_size or identity(os.fstat(source.fileno())) != identity(before):
                        raise ValueError("file changed during inventory")
                total += len(body)
                record["sha256"] = hashlib.sha256(body).hexdigest()
            else:
                record["kind"] = "refused"
                record["type"] = oct(stat.S_IFMT(before.st_mode))
                raise ValueError("inventory symlink or special file refused")
            if identity(os.stat(name, dir_fd=fd, follow_symlinks=False)) != identity(before):
                raise ValueError("inventory path changed after read")

    try:
        for label, root in roots.items():
            budget.check()
            before = os.stat(root, follow_symlinks=False)
            result[label] = {"kind": "directory", "mode": oct(stat.S_IMODE(before.st_mode)), "size": before.st_size}
            fd = os.open(root, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
            try:
                if identity(os.fstat(fd)) != identity(before):
                    raise ValueError("inventory root changed before admission")
                walk(fd, label, 0)
                if identity(os.fstat(fd)) != identity(before) or identity(os.stat(root, follow_symlinks=False)) != identity(before):
                    raise ValueError("inventory root changed during observation")
            finally:
                os.close(fd)
    except (ValueError, OSError) as exc:
        raise InventoryError(str(exc), result) from exc
    return result


def file_digest(path, budget, limit):
    before = os.stat(path, follow_symlinks=False)
    if not stat.S_ISREG(before.st_mode) or before.st_nlink != 1 or before.st_size > limit:
        raise ValueError("binary/fixture must be bounded regular unaliased file")
    fd = os.open(path, os.O_RDONLY | os.O_NONBLOCK | os.O_NOFOLLOW)
    digest, length = hashlib.sha256(), 0
    with os.fdopen(fd, "rb") as stream:
        if identity(os.fstat(stream.fileno())) != identity(before):
            raise ValueError("binary/fixture changed before reading")
        while True:
            budget.check()
            body = stream.read(min(1 << 20, limit + 1 - length))
            if not body:
                break
            digest.update(body)
            length += len(body)
            if length > limit:
                raise ValueError("binary/fixture byte bound exceeded")
        if identity(os.fstat(stream.fileno())) != identity(before):
            raise ValueError("binary/fixture changed during reading")
    if identity(os.stat(path, follow_symlinks=False)) != identity(before):
        raise ValueError("binary/fixture path changed after reading")
    return length, digest.hexdigest()


def read_regular(path, budget, limit):
    before = os.stat(path, follow_symlinks=False)
    if not stat.S_ISREG(before.st_mode) or before.st_nlink != 1 or before.st_size > limit:
        raise ValueError("bounded collector record must be regular and unaliased")
    descriptor = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    with os.fdopen(descriptor, "rb") as stream:
        if identity(os.fstat(stream.fileno())) != identity(before):
            raise ValueError("collector record changed before read")
        body = bounded_read(grok.BudgetReader(stream, budget), limit)
        if len(body) != before.st_size or identity(os.fstat(stream.fileno())) != identity(before):
            raise ValueError("collector record changed during read")
    if identity(os.stat(path, follow_symlinks=False)) != identity(before):
        raise ValueError("collector record changed after read")
    return body


def fixtures(root):
    sources = {}
    for label, version, marker in (("user", "0.9.0", "USER"), ("A", "1.0.0", "A"), ("B", "1.1.0", "B")):
        source = root / "sources" / label
        manifest = source / ".qoder-plugin/plugin.json"
        skill = source / "skills/marker/SKILL.md"
        manifest.parent.mkdir(parents=True)
        skill.parent.mkdir(parents=True)
        manifest.write_text(json.dumps({"name": PLUGIN_NAME, "version": version, "description": "Inert scoped research fixture"}) + "\n")
        skill.write_text(f"---\nname: marker\ndescription: Inert {marker} research marker\n---\n{marker}\n")
        sources[label] = source
    return sources


def fixture_pins(sources, budget):
    return {str(path): file_digest(path, budget, MAX_FILE)[1]
            for source in sources.values() for path in (source / ".qoder-plugin/plugin.json", source / "skills/marker/SKILL.md")}


def check_pins(request, budget):
    size, digest = file_digest(Path(request["binary"]), budget, phase1.MAX_BINARY)
    if size != BINARY_BYTES or digest != phase1.BINARY_SHA256:
        raise ValueError("pinned binary changed before native execution")
    sources = {"source-" + key: Path(value) for key, value in request["sources"].items()}
    if inventory(sources, budget) != request["source_baseline"]:
        raise ValueError("complete inert source fixture tree changed")
    for path, expected in request["fixture_pins"].items():
        if file_digest(Path(path), budget, MAX_FILE)[1] != expected:
            raise ValueError("fixed source fixture changed")


def isolation_facts(parent_namespace, uid, gid):
    status = {}
    with open("/proc/self/status", encoding="ascii") as stream:
        body = stream.read(65537)
    if len(body) > 65536:
        raise ValueError("process status exceeds bound")
    for line in body.splitlines():
        key, _, value = line.partition(":")
        status[key] = value.strip()
    namespace = os.readlink("/proc/self/ns/net")
    interfaces = sorted(name for _, name in socket.if_nameindex())
    facts = {"uid": os.getuid(), "gid": os.getgid(), "euid": os.geteuid(), "egid": os.getegid(), "groups": os.getgroups(), "network_namespace": namespace,
             "parent_network_namespace": parent_namespace, "interfaces": interfaces,
             "no_new_privs": status.get("NoNewPrivs"),
             "capabilities": {key: status.get(key) for key in ("CapInh", "CapPrm", "CapEff", "CapBnd", "CapAmb")}}
    if uid == 0 or gid == 0 or facts["uid"] != uid or facts["gid"] != gid or facts["euid"] != uid or facts["egid"] != gid or facts["groups"]:
        raise IsolationError("collector must retain original nonroot uid/gid and no supplementary groups", facts)
    if namespace == parent_namespace or interfaces != ["lo"] or facts["no_new_privs"] != "1":
        raise IsolationError("network namespace or no-new-privileges isolation not established", facts)
    if any(value is None or not re.fullmatch("0+", value) for value in facts["capabilities"].values()):
        raise IsolationError("collector retained capabilities", facts)
    return facts


def validate_output(record, stdout, stderr, kind, stage):
    if record["error"] or record["exit"] != 0 or stderr:
        raise ValueError("native command failed, exceeded bounds, or emitted stderr")
    if kind == "version":
        if not re.search(r"(?<![A-Za-z0-9_.+-])v?" + re.escape(VERSION) + r"(?![A-Za-z0-9_.+-])", stdout.decode("utf-8")):
            raise ValueError("native version differs from pin")
        return None
    if kind == "help":
        if not stdout.strip():
            raise ValueError("native help output empty")
        return None
    parsed = strict_json(stdout)
    if kind == "list":
        if not isinstance(parsed, list) or (stage == "empty" and parsed != []) or (stage != "empty" and not parsed):
            raise ValueError("empty/populated list boundary not established")
    elif not isinstance(parsed, (dict, list)) or not parsed:
        raise ValueError("successful install JSON has unknown scalar/empty shape")
    return parsed


def validate_request(request):
    root = Path(request["root"])
    if not root.is_absolute() or root.resolve() != root or root.parent == root:
        raise ValueError("collector root must be exact existing canonical directory")
    expected_sources = {key: str(root / "sources" / key) for key in ("user", "A", "B")}
    if request["sources"] != expected_sources or request["binary"] != str(root / "binary/qoder"):
        raise ValueError("collector source/binary paths differ from fixed layout")
    expected_pins = {str(Path(source) / relative) for source in expected_sources.values()
                     for relative in (".qoder-plugin/plugin.json", "skills/marker/SKILL.md")}
    if set(request["fixture_pins"]) != expected_pins or any(not re.fullmatch(r"[0-9a-f]{64}", pin) for pin in request["fixture_pins"].values()):
        raise ValueError("collector fixture pins differ from exact six source files")
    env = request["native_environment"]
    home = root / "home"
    expected = {"HOME": str(home), "USERPROFILE": str(home), "PATH": "/usr/bin:/bin",
                "TMPDIR": str(root / "tmp"), "LANG": "C.UTF-8", "GIT_CONFIG_NOSYSTEM": "1", "GIT_TERMINAL_PROMPT": "0"}
    for key, relative in (("XDG_CONFIG_HOME", "config"), ("XDG_CACHE_HOME", "cache"),
                          ("XDG_STATE_HOME", "state"), ("XDG_DATA_HOME", "data"),
                          ("NPM_CONFIG_CACHE", "npm-cache"), ("NPM_CONFIG_USERCONFIG", "npm-user-config"),
                          ("NPM_CONFIG_GLOBALCONFIG", "npm-global-config"), ("GIT_CONFIG_GLOBAL", "git-config")):
        expected[key] = str(home / relative)
    if env != expected:
        raise ValueError("native environment differs from exact fresh authority allowlist")
    output = Path(request["output"])
    if not output.is_absolute() or output.resolve() != output or output == root or output.is_relative_to(root):
        raise ValueError("output must be exact external test-owned directory")


def collect(request, summary, budget):
    validate_request(request)
    root, output = Path(request["root"]), Path(request["output"])
    summary["isolation"] = isolation_facts(request["parent_namespace"], request["uid"], request["gid"])
    sources = {key: Path(value) for key, value in request["sources"].items()}
    roots = {name: root / name for name in ("home", *LOCATIONS)}
    roots.update({"source-" + key: value for key, value in sources.items()})
    env = request["native_environment"]
    summary["native_environment"] = env
    summary["fixture_pins"] = request["fixture_pins"]
    summary["source_baseline"] = request["source_baseline"]
    summary["fixed_fixture_bodies"] = {path: read_regular(Path(path), budget, MAX_FILE).decode("utf-8") for path in request["fixture_pins"]}
    binary = request["binary"]

    def probe(label, arguments, cwd, kind, stage):
        check_pins(request, budget)
        record, stdout, stderr = run_probe([binary, *arguments], cwd, env, output, label,
                                          timeout=min(30, budget.check()))
        record["accepted"] = False
        summary["probes"].append(record)
        check_pins(request, budget)
        parsed = validate_output(record, stdout, stderr, kind, stage)
        record["accepted"] = True
        if parsed is not None:
            record["observed_json"] = parsed
        return parsed

    for label, arguments in HELP:
        probe(label, arguments, root / "user-cwd", "version" if label == "version" else "help", "empty")
    for stage in STAGES:
        summary["stage"] = stage
        summary["inventories"][stage + "-before"] = inventory(roots, budget)
        if stage != "empty":
            key = {"user-installed": "user", "A-installed": "A", "B-installed": "B"}[stage]
            location = {"user": "user-cwd", "A": "project-A", "B": "project-B"}[key]
            scope = "user" if key == "user" else "project"
            probe(stage + "-install", ["plugins", "install", str(sources[key]), "--scope", scope, "--json"], root / location, "install", stage)
        for location in LOCATIONS:
            probe(stage + "-" + location + "-list", ["plugins", "list", "--json"], root / location, "list", stage)
        check_pins(request, budget)
        summary["inventories"][stage + "-after"] = inventory(roots, budget)
    summary["success"] = len(summary["probes"]) == 19


def wrapper_command(request_path, request):
    carried = {key: os.environ[key] for key in HOST_KEYS if key in os.environ}
    return ["/usr/bin/sudo", "-n", "/usr/bin/unshare", "--net", "--", "/usr/bin/setpriv",
            "--reuid=" + str(request["uid"]), "--regid=" + str(request["gid"]), "--clear-groups",
            "--inh-caps=-all", "--ambient-caps=-all", "--bounding-set=-all", "--no-new-privs",
            "/usr/bin/env", "-i", "PATH=/usr/bin:/bin", "LANG=C.UTF-8", "PYTHONDONTWRITEBYTECODE=1",
            *[key + "=" + value for key, value in carried.items()], sys.executable, str(Path(__file__).resolve()),
            "--collect", str(request_path)]


def acquire(directory, budget):
    with phase1.urllib.request.urlopen(phase1.URL, timeout=min(90, budget.check())) as response:
        body = bounded_read(grok.BudgetReader(response, budget), phase1.MAX_ARCHIVE)
    phase1.native.verify_digest(body, phase1.ARCHIVE_PIN)
    binary = phase1.archive_binary(body)
    if len(binary) != BINARY_BYTES or hashlib.sha256(binary).hexdigest() != phase1.BINARY_SHA256:
        raise ValueError("downloaded binary differs from exact pin")
    path = directory / "qoder"
    path.write_bytes(binary)
    path.chmod(0o700)
    return path, {"url": phase1.URL, "version": VERSION, "archive_integrity": phase1.ARCHIVE_PIN,
                  "archive_bytes": len(body), "binary_sha256": phase1.BINARY_SHA256, "binary_bytes": len(binary)}


def summary_record():
    return {"evidence_kind": "qoder-populated-scope-research", "version": VERSION,
            "source_commit": os.environ.get("EXPECTED_COMMIT", ""), "success": False,
            "probes": [], "inventories": {}, **FALSE_CLAIMS}


def write_summary(output, summary):
    (output / "qoder-populated-research.json").write_text(json.dumps(summary, indent=2) + "\n")


def child_main(request_path):
    phase1.native.require_hosted("linux-amd64")
    request = strict_json(read_regular(request_path, grok.Budget(5), 1 << 20))
    output = Path(request["output"])
    summary = summary_record()
    try:
        with grok.Budget(max(0.001, request["deadline"] - time.monotonic())) as budget:
            collect(request, summary, budget)
    except Exception as exc:
        summary["error"] = f"{type(exc).__name__}: {str(exc)[:512]}"
        if isinstance(exc, InventoryError):
            summary["partial_inventory"] = exc.partial
        if isinstance(exc, IsolationError):
            summary["isolation"] = exc.facts
    write_summary(output, summary)
    return 0 if summary["success"] else 1


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path)
    parser.add_argument("--collect", type=Path, help=argparse.SUPPRESS)
    args = parser.parse_args(argv)
    try:
        phase1.native.require_hosted("linux-amd64")
    except RuntimeError as exc:
        print(str(exc)[:512], file=sys.stderr)
        return 2
    if args.collect:
        return child_main(args.collect)
    if args.output is None:
        parser.error("--output is required")
    output = args.output.absolute()
    output.mkdir(parents=True, exist_ok=False)
    summary = summary_record()
    try:
        if os.getuid() == 0 or os.getgid() == 0:
            raise ValueError("original hosted collector must be nonroot")
        with grok.Budget(COLLECTOR_SECONDS) as budget, phase1.tempfile.TemporaryDirectory(prefix="qoder-populated-") as temporary:
            root = Path(temporary)
            for name in ("binary", "home", "tmp", *LOCATIONS):
                (root / name).mkdir()
            binary, summary["acquisition"] = acquire(root / "binary", budget)
            sources = fixtures(root)
            request = {"root": str(root), "output": str(output), "binary": str(binary),
                       "sources": {key: str(path) for key, path in sources.items()},
                       "fixture_pins": fixture_pins(sources, budget),
                       "source_baseline": inventory({"source-" + key: value for key, value in sources.items()}, budget),
                       "native_environment": phase1.runtime_environment(root / "home", root / "tmp"),
                       "uid": os.getuid(), "gid": os.getgid(), "parent_namespace": os.readlink("/proc/self/ns/net"),
                       "deadline": budget.deadline}
            request_path = root / "request.json"
            request_path.write_text(json.dumps(request))
            record, _, _ = run_probe(wrapper_command(request_path, request), root, {}, output,
                                     "isolated-collector", timeout=budget.check())
            summary["wrapper"] = record
            child_path = output / "qoder-populated-research.json"
            if child_path.exists():
                child = strict_json(read_regular(child_path, budget, 64 << 20))
                child["acquisition"], child["wrapper"] = summary["acquisition"], record
                summary = child
            if record["error"] or record["exit"] != 0:
                summary["success"] = False
                summary.setdefault("error", "isolated collector failed")
    except Exception as exc:
        summary["success"] = False
        summary["error"] = f"{type(exc).__name__}: {str(exc)[:512]}"
    write_summary(output, summary)
    return 0 if summary["success"] else 1


if __name__ == "__main__":
    sys.exit(main())
