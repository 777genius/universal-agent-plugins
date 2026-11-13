#!/usr/bin/env python3
"""Finite Devin metadata capture; no product or MCP runtime qualification."""
import argparse
from contextlib import ExitStack
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import shutil
import stat
import sys

import project_native_research as shared
from pi_project_components_native_e2e import PiHTTPMCPProbe

VERSION = "3000.12.57"
ARCHIVE = "0946784ef886d716e82a8026f76933bf0a6e4f0a804a23722bb62047e8c07d4a"
BINARY = "3f3fb99f472d92f7852be3775ac2b66b5d60baf26f3441b02d54c56b160bc30d"
SPEC = {"url": "https://static.devin.ai/cli/3000.12.57/devin-3000.12.57-x86_64-unknown-linux.tar.gz",
        "integrity": "sha256:" + ARCHIVE, "expected_bytes": 72925933,
        "max_bytes": 96 << 20, "max_expanded_bytes": 256 << 20,
        "format": "tar.gz", "entrypoint": "bin/devin", "runtime": "native", "binary_sha256": BINARY}
HELP = (("version", ["--version"]), ("help", ["--help"]),
        ("mcp-help", ["mcp", "--help"]), ("mcp-list-help", ["mcp", "list", "--help"]),
        ("skills-list-help", ["skills", "list", "--help"]))


class AdmissionContext(shared.Context):
    """A nonreturning inherited runner leaves cleanup authority unproved."""
    process_cleanup_proved = True
    def command(self, *args, **kwargs):
        self.process_cleanup_proved = False
        self.summary["in_flight_command"] = args[0] if args else "unknown"
        result = super().command(*args, **kwargs)
        self.process_cleanup_proved = True
        self.summary.pop("in_flight_command", None)
        return result


def metadata_inventory(root, budget):
    """Finite descriptor-relative metadata; large runtime bytes use verify_files."""
    held = root_identity(root)
    result = {}
    for current, dirs, files, descriptor in os.fwalk(root, follow_symlinks=False):
        budget.check()
        relative = Path(current).relative_to(root)
        require(len(relative.parts) <= 32 and len(result) + len(dirs) + len(files) <= 4096, "outer metadata inventory bound")
        linked = Path(current).lstat()
        opened = os.fstat(descriptor)
        require((linked.st_dev, linked.st_ino, linked.st_mode) == (opened.st_dev, opened.st_ino, opened.st_mode)
                and stat.S_ISDIR(opened.st_mode), "outer directory identity differs")
        result[relative.as_posix()] = (opened.st_dev, opened.st_ino, opened.st_mode, opened.st_uid, opened.st_gid)
        for name in dirs + files:
            info = os.stat(name, dir_fd=descriptor, follow_symlinks=False)
            require(stat.S_ISDIR(info.st_mode) or stat.S_ISREG(info.st_mode), "outer alias/special member refused")
            require(stat.S_ISDIR(info.st_mode) or (info.st_nlink == 1 and info.st_size <= 512 << 20), "outer file alias/size refused")
            result[(relative / name).as_posix()] = (info.st_dev, info.st_ino, info.st_mode, info.st_nlink,
                info.st_uid, info.st_gid, info.st_size, info.st_mtime_ns, info.st_ctime_ns)
    require(root_identity(root) == held, "outer metadata root replaced")
    return result


def require(value, message):
    if not value:
        raise ValueError(message)


def strict_json(body):
    require(isinstance(body, bytes) and 0 < len(body) <= 1 << 20, "bounded JSON required")
    def unique(pairs):
        result = {}
        for key, value in pairs:
            require(key not in result, "duplicate native JSON key")
            result[key] = value
        return result
    value = json.loads(body.decode("utf-8", "strict"), object_pairs_hook=unique,
                       parse_constant=lambda _value: (_ for _ in ()).throw(ValueError("nonfinite JSON")))
    def visit(item, depth=0):
        require(depth <= 16, "native JSON depth")
        if isinstance(item, (dict, list)):
            require(len(item) <= 512, "native JSON cardinality")
            for child in item.values() if isinstance(item, dict) else item:
                visit(child, depth + 1)
    visit(value)
    require(isinstance(value, (dict, list)), "native JSON inventory container required")
    return value


def admitted_archive(receipt):
    members = receipt["members"]
    require(receipt["pin_verified"] and receipt["sha256"] == ARCHIVE and receipt["bytes"] == SPEC["expected_bytes"], "archive pin differs")
    require(len(members) == 250 and sum(row["kind"] == "file" for row in members) == 234
            and all(row["kind"] in ("file", "directory") for row in members), "archive closed member inventory differs")
    entry = [row for row in members if row["name"] == "bin/devin"]
    require(len(entry) == 1 and entry[0]["sha256"] == BINARY and entry[0]["bytes"] == 213734376
            and entry[0]["executable"] is True, "selected binary identity differs")
    require([row["name"] for row in members if row["kind"] == "file" and row["executable"]] == ["bin/devin"], "unreviewed executable companion")
    require(sum(row["bytes"] for row in members) == 214280841, "expanded runtime bytes differ")


def snapshot(ctx):
    roots = {"home": ctx.home, "temporary": ctx.root / "tmp", **ctx.projects}
    result = shared.legacy.inventory(roots, ctx.budget)
    require(all(row["kind"] in ("file", "directory") for row in result.values()), "fixture special/alias member")
    return result


def root_identity(path):
    require(path.is_absolute() and path.resolve() == path and not path.is_symlink(), "fixture root alias")
    info = path.lstat()
    require(stat.S_ISDIR(info.st_mode), "fixture root is not a directory")
    return (info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid)


def fixture(ctx, urls, nonce):
    original = Path(ctx.env["XDG_CONFIG_HOME"]) / "devin"
    roots = {"user": original, **{name: ctx.projects[name] / ".devin" for name in ("a", "b")}}
    startup = ctx.root / "stdio-started"
    marker = ctx.root / "stdio-effect.py"
    marker.write_text("from pathlib import Path\nPath(" + repr(str(startup)) + ").write_text('unexpected native start')\n")
    expected = {}
    for name, root in roots.items():
        root.mkdir(mode=0o700, parents=True, exist_ok=True)
        entries = {"same-name": {"url": urls[name], "disabled": False, "disabledTools": ["never-auto-approve"]},
                   "foreign-" + name: {"url": urls[name], "disabled": True},
                   "stdio-" + name: {"command": sys.executable, "args": [str(marker)], "disabled": False}}
        body = json.dumps({"mcpServers": entries}, sort_keys=True).encode() + b"\n"
        (root / "mcp_config.json").write_bytes(body)
        (root / "config.json").write_text('{"read_config_from":{"cloud":false}}\n')
        skill = root / "skills" / "same-skill" / "SKILL.md"
        skill.parent.mkdir(parents=True)
        skill.write_text("---\nname: same-skill\ndescription: Inert " + name + " " + nonce + "\n---\nPreserved source " + name + "\n")
        expected[name] = {"root": str(root), "mcp": entries,
                          "mcp_config_sha256": hashlib.sha256(body).hexdigest(), "skill": str(skill)}
    legacy = ctx.home / ".codeium" / "windsurf"
    legacy.mkdir(parents=True)
    (legacy / "mcp_config.json").write_text('{"mcpServers":{"legacy-foreign":{"url":"' + urls["legacy"] + '","disabled":true}},"sentinel":"' + nonce + '"}\n')
    (legacy / "config.json").write_text(json.dumps({"inert_legacy_sentinel": nonce}) + "\n")
    legacy_skill = legacy / "skills" / "legacy-foreign" / "SKILL.md"
    legacy_skill.parent.mkdir(parents=True)
    legacy_skill.write_text("---\nname: legacy-foreign\ndescription: Preserve legacy Windsurf skill.\n---\n" + nonce + "\n")
    local = ctx.projects["a"] / ".devin" / "mcp_config.local.json"
    body = json.dumps({"mcpServers": {"same-name": {"url": urls["local"], "disabled": True}}}).encode()
    local.write_bytes(body)
    expected["local"] = {"path": str(local), "sha256": hashlib.sha256(body).hexdigest(), "url": urls["local"]}
    return expected, startup, marker


def zero_effects(probes, startup):
    require(not startup.exists() and not startup.is_symlink(), "native metadata launched stdio")
    for probe in probes.values():
        calls, invalid = probe.call_records()
        require(not probe.methods() and not calls and invalid == 0 and not probe.accepted_connections, "native metadata contacted MCP endpoint")


def collect(ctx, probes):
    nonce = secrets.token_hex(32)
    expected, startup, marker = fixture(ctx, {key: value.url for key, value in probes.items()}, nonce)
    roots = {"home": ctx.home, "temporary": ctx.root / "tmp", **ctx.projects}
    identities = {name: root_identity(path) for name, path in roots.items()}
    baseline = snapshot(ctx)
    outer_before = metadata_inventory(ctx.root, ctx.budget)
    ctx.identities[marker] = hashlib.sha256(marker.read_bytes()).hexdigest()
    ctx.summary["stdio_effect_guard"] = {"path": str(marker), "sha256": ctx.identities[marker], "python": sys.executable}
    ctx.summary.update(fixture=expected, fixture_before=baseline, outer_before=outer_before, command_effects=[],
                       exact_source_admission="unreviewed-native-schema", admission_qualified=False)

    def command(label, arguments, scope="a"):
        before = snapshot(ctx)
        require(before == baseline, "unmeasured fixture change")
        require(metadata_inventory(ctx.root, ctx.budget) == outer_before, "unmeasured outer/runtime change")
        try:
            record, out, err = ctx.probe(label, arguments, project=scope)
            require(record["exit"] == 0, "native metadata command refused")
            return out, err
        finally:
            after = snapshot(ctx)
            delta = {key: {"before": before.get(key), "after": after.get(key)}
                     for key in before.keys() | after.keys() if before.get(key) != after.get(key)}
            outer_after = metadata_inventory(ctx.root, ctx.budget)
            outer_delta = {key: {"before": outer_before.get(key), "after": outer_after.get(key)}
                           for key in outer_before.keys() | outer_after.keys() if outer_before.get(key) != outer_after.get(key)}
            ctx.summary["command_effects"].append({"label": label, "changes": delta, "outer_changes": outer_delta})
            require({name: root_identity(path) for name, path in roots.items()} == identities, "fixture root replaced")
            require(after == baseline, "native inventory changed original authority or fixture")
            require(outer_after == outer_before, "native metadata changed outer/runtime inventory")
            zero_effects(probes, startup)

    for label, arguments in HELP:
        out, err = command(label, arguments)
        require(not err, "native help/version diagnostic")
        text = out.decode("utf-8", "strict")
        if label == "version":
            require(re.fullmatch(r"devin 3000\.12\.57 \([0-9a-f]{7,40}\)\n", text), "native version/build grammar differs")
        if label.endswith("list-help"):
            require("--json" in text, "native help omits JSON inventory")
    for label, scope in (("a", "a"), ("b", "b"), ("user", "user"), ("a-repeat", "a")):
        for kind in ("mcp", "skills"):
            out, _err = command(kind + "-" + label, [kind, "list", "--json"], scope)
            strict_json(out)
    ctx.summary.update(fixture_after=snapshot(ctx), metadata_capture_complete=True,
                       native_json_schema="requires-independent-original-review",
                       probe_methods={name: list(probe.methods()) for name, probe in probes.items()},
                       accepted_connections={name: len(probe.accepted_connections) for name, probe in probes.items()})


def observe_connections(probe):
    """Observe the existing fixture's accepted connections, including non-MCP HTTP."""
    probe.accepted_connections = []
    original = probe.server.get_request
    def get_request():
        connection, address = original()
        connection.settimeout(2)
        probe.accepted_connections.append(address)
        return connection, address
    probe.server.get_request = get_request
    return probe


def run(output):
    shared.native.require_hosted("linux-amd64")
    require(output.is_absolute() and output.resolve() == output and not output.exists()
            and shared.SOURCE not in output.parents, "fresh output outside source required")
    output.mkdir(mode=0o700, parents=True)
    identity = root_identity(output)
    summary = {"client": "windsurf", "phase": "observe", "candidate": "devin-structured-inventory-" + VERSION,
               "collection_complete": False, "metadata_capture_complete": False, "admission_qualified": False,
               "qualification": {name: False for name in shared.QUALIFICATION_CASES}, "commands": [],
               "host": {key: os.environ.get(key) for key in ("RUNNER_OS", "RUNNER_ARCH", "ImageOS", "ImageVersion", "GITHUB_RUN_ID", "GITHUB_RUN_ATTEMPT")}}
    ctx, held_cleanup, probes_settled = None, {}, True
    try:
        with shared.Budget(360) as budget:
            ctx = AdmissionContext(output, summary, budget)
            held_cleanup = {path: root_identity(path) for path in (ctx.home, ctx.private, ctx.output, output / "tmp", *ctx.projects.values())}
            ctx.source_and_projects()
            receipt, contents = shared.acquire(SPEC, budget)
            admitted_archive(receipt)
            summary["acquisition"] = receipt
            binary, ctx.identities = shared.materialize(output / "client", SPEC, receipt, contents)
            held_cleanup[output / "client"] = root_identity(output / "client")
            ctx.argv = [str(binary)]
            summary["runtime_files"] = [{"path": str(path), "sha256": digest} for path, digest in ctx.identities.items()]
            probes_settled = False
            probes = {}
            try:
                with ExitStack() as stack:
                    for key in ("a", "b", "user", "local", "legacy"):
                        probes[key] = observe_connections(stack.enter_context(PiHTTPMCPProbe(nonce=secrets.token_hex(32))))
                    try:
                        collect(ctx, probes)
                    finally:
                        summary.update(probe_methods={name: list(probe.methods()) for name, probe in probes.items()},
                                       accepted_connections={name: len(probe.accepted_connections) for name, probe in probes.items()})
            finally:
                probes_settled = len(probes) == 5 and all(not probe.thread.is_alive() for probe in probes.values())
            zero_effects(probes, output / "stdio-started")
            shared.verify_files(ctx.identities, budget)
            record, out, err = ctx.command("source-final-clean", ["/usr/bin/git", "status", "--porcelain", "--untracked-files=normal"], shared.SOURCE, ctx.env, 10)
            require(record["exit"] == 0 and not out and not err, "source changed")
            summary["collection_complete"] = True
    except Exception as error:
        summary["error"] = type(error).__name__ + ": " + str(error)[:300]
    finally:
        require(root_identity(output) == identity, "outer root replaced")
        try:
            require((ctx is None or ctx.process_cleanup_proved) and probes_settled, "owned process/probe cleanup unproved")
            for path, held in held_cleanup.items():
                require(root_identity(path) == held, "cleanup directory identity changed")
                shutil.rmtree(path)
            summary["cleanup_complete"] = True
        except Exception as error:
            summary.update(collection_complete=False, cleanup_complete=False,
                           cleanup_error=type(error).__name__)
        # Existing publisher admits only held bounded streams and closed JSON ZIP.
        # Unproved cleanup retains a fixed failure report, never raw captures.
        if not summary.get("cleanup_complete"):
            safe = {"collection_complete": False, "admission_qualified": False,
                    "qualification": summary["qualification"], "commands": [], "cleanup_complete": False}
            shared.publish(safe, None, output)
        else:
            try:
                bodies = shared.evidence(summary, ctx.captures if ctx else {}, [])
                require(sum(map(len, bodies.values())) <= 16 << 20, "admission evidence aggregate exceeds16MiB")
                shared.publish(summary, ctx, output)
            except (ValueError, KeyError, TypeError, OSError):
                summary["collection_complete"] = False
                safe = {"collection_complete": False, "admission_qualified": False, "commands": [],
                        "qualification": summary["qualification"], "artifact_admission_failed": True}
                shared.publish(safe, None, output)
    return 0 if summary["collection_complete"] else 1


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True, type=Path)
    raise SystemExit(run(parser.parse_args().output))
