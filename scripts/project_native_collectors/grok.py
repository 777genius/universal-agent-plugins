"""Measured Grok 1.0.70 acquisition and fixed native help research only.

The shared runner owns genuine hosted admission, immutable acquisition,
resource limits and capture. The explicit observe phase adds only read-only
opaque catalogs around text-only skill fixtures; no trust, doctor, provider or
MCP connection is invoked and no native discovery qualification is inferred.
"""
import hashlib
import json
import math
import os
from pathlib import Path
import re
import stat

import native_client_project_artifacts as artifacts

CLIENT = "grok"
VERSION = "1.0.70"
AVAILABLE_PHASES = ("acquire", "help", "observe")
ACQUISITION = {
    "url": "https://x.ai/cli/grok-1.0.70-linux-x86_64.gz",
    # Genuine run 40235269281/attempt 2, artifact 12615677952; reviewed freeze
    # b157c54c086a3eb17f285c6013749de59e26ed2f. No 1.0.67 identity is reused.
    "integrity": "sha256:11d27a30adfb75c4b9b180072ab27412a1916f054ff5497ab70fb9a77d304bcc",
    "expected_bytes": 76_760_282,
    "binary_sha256": "22d281c5964c5444147328096ba2a221f3f0874c6858e27c825656a80f6e58e9",
    "max_bytes": 96 << 20,
    "max_expanded_bytes": 256 << 20,
    "format": "gzip",
    # A gzip stream has no archive member: this is the shared runner's fixed
    # synthetic output name, never an authority derived from its gzip header.
    "entrypoint": "grok",
    "runtime": "native",
}

# This periodically synced September snapshot guides candidate commands;
# it is not shipping-source provenance for the measured executable.
COMMAND_SOURCE = "https://github.com/xai-org/grok-build/tree/2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8"
PROBES = (
    ("grok-version", ("--version",), None),
    ("grok-help", ("--help",), ""),
    ("grok-inspect-help", ("inspect", "--help"), "inspect"),
    ("grok-mcp-help", ("mcp", "--help"), "mcp"),
    ("grok-mcp-list-help", ("mcp", "list", "--help"), "mcp list"),
    ("grok-mcp-doctor-help", ("mcp", "doctor", "--help"), "mcp doctor"),
    ("grok-plugin-help", ("plugin", "--help"), "plugin"),
    ("grok-plugin-list-help", ("plugin", "list", "--help"), "plugin list"),
    ("grok-doctor-help", ("doctor", "--help"), "doctor"),
)


def _streams(record, stdout, stderr):
    if (not isinstance(record, dict) or type(record.get("exit")) is not int
            or record["exit"] != 0 or "error" not in record
            or record["error"] is not None):
        raise ValueError("Grok fixed help probe failed its bounded capture")
    if not isinstance(stdout, bytes) or not isinstance(stderr, bytes) or stderr:
        raise ValueError("Grok fixed help probe has unexpected stream data")
    try:
        return stdout.decode("utf-8", "strict")
    except UnicodeError:
        raise ValueError("Grok fixed help probe is not UTF-8") from None


def _version(stdout):
    match = re.fullmatch(rb"grok 1\.0\.70 \(([0-9a-f]{7,40})\)( \[stable\])?\n", stdout)
    if not match:
        raise ValueError("Grok version differs from the frozen 1.0.70 grammar")
    return {"version": VERSION, "compiled_commit_observed": match[1].decode("ascii"),
            "channel_observed": "stable" if match[2] else None,
            "shipping_source_equivalence_claimed": False}


def _help(text, command):
    name = "grok" + (" " + command if command else "")
    # Complete Clap metavariables only, including a variadic suffix. A parent
    # <COMMAND> is legitimate; different commands or unclosed delimiters are not.
    metavar = r"(?:\[[A-Za-z0-9_.:|= /-]+\]|<[A-Za-z0-9_.:|= /-]+>)(?:\.\.\.)?"
    usage = re.findall(r"^Usage: ([^\r\n]+)$", text, re.MULTILINE)
    if (len(usage) != 1 or not re.fullmatch(re.escape(name)
            + r"(?:[ \t]+" + metavar + r")*[ \t]*", usage[0])):
        raise ValueError("Grok help Usage does not identify the requested command")
    if not re.search(r"^[ \t]+(?:-h,[ \t]+)?--help(?:[ \t]+[^\r\n]*)?$", text, re.MULTILINE):
        raise ValueError("Grok fixed command help is missing its help option")
    if not command:
        if not re.search(r"^Commands:$", text, re.MULTILINE) or any(
                not re.search(r"^[ \t]{2,}" + word + r"(?:[ \t]+[^\r\n]*)?$", text, re.MULTILINE)
                for word in ("inspect", "mcp", "plugin")):
            raise ValueError("Grok main help is missing the selected command surface")



UPDATER_ENV = {"GROK_DISABLE_AUTOUPDATER": "1"}
CATALOGS = (("inspect", ("inspect", "--json")),
            ("plugins", ("plugin", "list", "--json")),
            ("mcp", ("mcp", "list", "--json")))
NAMES = {scope: "uap-grok-research-" + scope for scope in ("user", "a", "b")}


def _json(body):
    if not isinstance(body, bytes) or not 0 < len(body) <= 1 << 20:
        raise ValueError("Grok opaque JSON size/type refused")

    def unique(pairs):
        result = {}
        for key, value in pairs:
            if key in result:
                raise ValueError("Grok duplicate JSON key")
            result[key] = value
        return result

    def constant(value):
        raise ValueError("Grok nonfinite JSON number: " + value)

    try:
        result = json.loads(body.decode("utf-8", "strict"), object_pairs_hook=unique, parse_constant=constant)
        if not isinstance(result, (dict, list)):
            raise ValueError("Grok opaque catalog must be a JSON object or list")
        pending, count = [(result, 0)], 0
        while pending:
            value, depth = pending.pop()
            count += 1
            if count > 8192 or depth > 32:
                raise ValueError("Grok opaque JSON structure limit")
            if isinstance(value, dict):
                pending.extend((entry, depth + 1) for pair in value.items() for entry in pair)
            elif isinstance(value, list):
                pending.extend((entry, depth + 1) for entry in value)
            elif isinstance(value, float) and not math.isfinite(value):
                raise ValueError("Grok nonfinite JSON number")
        return result
    except (UnicodeError, RecursionError) as error:
        raise ValueError("Grok opaque catalog is malformed or too deep") from error


def _directory(root, parts, create=False):
    # Every component below the fixed fresh root is opened descriptor-relative;
    # no authored path is followed through an alias, including during creation.
    descriptor = os.open(root, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        if not artifacts.same_opened(root.lstat(), os.fstat(descriptor)):
            raise ValueError("Grok fixture root identity differs")
        for part in parts:
            if create:
                try:
                    os.mkdir(part, 0o700, dir_fd=descriptor)
                except FileExistsError:
                    pass
            child = os.open(part, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=descriptor)
            os.close(descriptor)
            descriptor = child
            if os.fstat(descriptor).st_uid != os.getuid():
                raise ValueError("Grok fixture directory owner differs")
        return descriptor
    except BaseException:
        os.close(descriptor)
        raise


def _read_owned(root, relative, expected, original=None):
    parent = _directory(root, relative.parts[:-1])
    try:
        before = os.stat(relative.name, dir_fd=parent, follow_symlinks=False)
        if (not stat.S_ISREG(before.st_mode) or before.st_nlink != 1
                or before.st_uid != os.getuid() or stat.S_IMODE(before.st_mode) != 0o600
                or before.st_size != len(expected) or before.st_size > 8192
                or (original is not None and artifacts.identity(before) != original)):
            raise ValueError("Grok authored fixture type/size/mode differs")
        descriptor = os.open(relative.name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=parent)
        try:
            held = os.fstat(descriptor)
            if not artifacts.same_opened(before, held):
                raise ValueError("Grok authored fixture opened identity differs")
            body = os.read(descriptor, len(expected) + 1)
            if body != expected or artifacts.identity(os.fstat(descriptor)) != artifacts.identity(held):
                raise ValueError("Grok authored fixture bytes/held identity differs")
        finally:
            os.close(descriptor)
        if artifacts.identity(os.stat(relative.name, dir_fd=parent, follow_symlinks=False)) != artifacts.identity(before):
            raise ValueError("Grok authored fixture path identity differs")
        # Reopen the namespace after the held read to reject replaced parents.
        current = _directory(root, relative.parts[:-1])
        try:
            left, right = os.fstat(parent), os.fstat(current)
            if (left.st_dev, left.st_ino) != (right.st_dev, right.st_ino):
                raise ValueError("Grok authored fixture parent replaced")
        finally:
            os.close(current)
        return artifacts.identity(before)
    finally:
        os.close(parent)


def _fixture(root, scope):
    name = NAMES[scope]
    plugin = Path(".grok/plugins") / name
    manifest = {"name": name, "version": "1.0.0", "description": "Inert native scope research", "skills": "./skills/"}
    files = {plugin / ".grok-plugin/plugin.json": (json.dumps(manifest, sort_keys=True) + "\n").encode(),
             plugin / "skills" / name / "SKILL.md": (
                 "---\nname: " + name + "\ndescription: Inert native scope research marker\n---\n\nText-only marker for " + scope + ".\n").encode()}
    owned = []
    for relative, body in files.items():
        parent = _directory(root, relative.parts[:-1], create=True)
        try:
            descriptor = os.open(relative.name, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW,
                                 0o600, dir_fd=parent)
            try:
                if os.write(descriptor, body) != len(body):
                    raise ValueError("Grok authored fixture short write")
            finally:
                os.close(descriptor)
        finally:
            os.close(parent)
        original = _read_owned(root, relative, body)
        owned.append((root, relative, body, original))
    return owned


def _observe(ctx):
    root, home = Path(ctx.root), Path(ctx.home)
    projects = {key: Path(ctx.projects[key]) for key in ("a", "b", "user")}
    roots = [home, *projects.values()]
    if (not root.is_absolute() or root.resolve() != root or len(set(roots)) != 4
            or any(path.resolve() != path or not path.is_relative_to(root) or path == root for path in roots)
            or any(left.is_relative_to(right) for left in roots for right in roots if left != right)):
        raise ValueError("Grok requires separate real projects and original home")
    owned, rows = [], []

    def catalog(stage, project, kind, arguments):
        for folder, relative, body, original in owned:
            _read_owned(folder, relative, body, original)
        record, stdout, stderr = ctx.probe("grok-" + stage + "-" + project + "-" + kind,
                                           list(arguments), project=project, extra_env=UPDATER_ENV, timeout=30)
        text = _streams(record, stdout, stderr)
        value = _json(stdout)
        for folder, relative, body, original in owned:
            _read_owned(folder, relative, body, original)
        rows.append({"stage": stage, "cwd_selection": project, "catalog": kind,
                     "root_kind": "object" if isinstance(value, dict) else "list",
                     "stdout_sha256": hashlib.sha256(stdout).hexdigest(),
                     "marker_tokens": [name for name in NAMES.values() if name in text],
                     "interpretation": "opaque native catalog; no source schema or loading qualification"})

    for project in ("a", "user"):
        for kind, arguments in CATALOGS:
            catalog("empty", project, kind, arguments)
    for scope, directory in (("user", home), ("a", projects["a"]), ("b", projects["b"])):
        owned.extend(_fixture(directory, scope))
    for project in ("a", "b", "user"):
        for kind, arguments in CATALOGS:
            catalog("populated", project, kind, arguments)
    return {"catalogs": rows, "protected_files": [
                {"root": str(folder), "path": str(relative), "sha256": hashlib.sha256(body).hexdigest()}
                for folder, relative, body, _original in owned],
            "scope_discovery_qualified": False, "uap_lifecycle_qualified": False,
            "mcp_initialization_qualified": False, "mcp_tools_list_qualified": False,
            "mcp_tools_call_qualified": False, "authentication_qualified": False, "real_model_qualified": False,
            "remaining": "Freeze observed native schemas before any source-selection, trust or lifecycle claim"}


def collect(ctx):
    """Run fixed help and only the explicitly selected opaque research phase."""
    if ctx.phase not in ("help", "observe"):
        raise ValueError("Grok collector requires the explicit help or observe phase")
    observations = {}
    for label, arguments, command in PROBES:
        record, stdout, stderr = ctx.probe(label, list(arguments), project="a", extra_env=UPDATER_ENV, timeout=30)
        text = _streams(record, stdout, stderr)
        row = {"surface": "version" if command is None else "help", "arguments": list(arguments)}
        if command is None:
            row.update(_version(stdout))
        else:
            _help(text, command)
        observations[label] = row
    cases = [{"id": "grok.help", "status": "passed",
              "reason": "Measured pinned Grok 1.0.70 fixed version/help table observed; help alone makes no discovery or runtime claim"}]
    unobserved = ["grok.scope-observation", "grok.project-lifecycle",
                  "grok.native-restart-removal", "grok.mcp-initialize",
                  "grok.mcp-tools-list", "grok.mcp-tools-call"]
    if ctx.phase == "observe":
        observations["opaque-scope-catalogs"] = _observe(ctx)
        cases.append({"id": "grok.scope-observation", "status": "passed",
                      "reason": "All fifteen read-only opaque JSON catalogs completed with exact text-only fixture preservation; source schema and every product qualification remain unqualified"})
        unobserved.remove("grok.scope-observation")
    return {"observations": observations, "cases": cases, "unobserved": unobserved}
