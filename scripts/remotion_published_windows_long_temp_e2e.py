"""Real pinned Remotion Skill lifecycle on short and default-long Windows TEMP.

Reuses the existing published/native case auditor with a published npm CLI pinned to release 0.1.102.
No alternate success interpretation, fixture edit or native trust bypass.
"""

import hashlib
import importlib.util
import json
import os
import re
import stat
import traceback
from pathlib import Path
import shutil
import subprocess
import tempfile

FIXTURE_SHA256 = "e22cb3b5b23e74a9e03b55e24a684ab93038f89cf18e28fd5d9755175754c0d9"
REMOTION_SHA = "c7cb2fdcacd134fb2ccb9b7a44f9ffa44e467271"


def sanitized_failure_site(error, repo):
    """Report only source basename, line and function; never exception text."""
    scripts = (repo / "scripts").resolve()
    for frame in reversed(traceback.extract_tb(error.__traceback__)):
        filename = Path(frame.filename)
        if (filename.parent.resolve() == scripts
                and re.fullmatch(r"[A-Za-z0-9_-]{1,80}\.py", filename.name)
                and re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]{0,79}", frame.name)
                and type(frame.lineno) is int and 0 < frame.lineno < 100000):
            return {"file": filename.name, "line": frame.lineno, "function": frame.name}
    return {"file": "unqualified", "line": 0, "function": "unqualified"}


DIAGNOSTIC_LIMIT = 16


def diagnostic_path(path, root):
    """Lexical labels only: never resolve a failed authority to label it."""
    if not isinstance(path, Path):
        return {"role": "unqualified"}
    case = root / "skill-24"
    known = {
        case: "case", case / "home": "home", case / "project": "project",
        case / "uap-data": "uap-data", case / "tmp": "temporary",
        case / "home/.codex": "codex-profile",
        case / "home/.local": "local",
        case / "home/.local/state": "local-state",
        case / "home/.local/state/codex-sqlite": "sqlite-runtime",
    }
    try:
        relative = path.relative_to(root).as_posix()
    except ValueError:
        return {"role": "outside-disposable-root"}
    if len(relative) > 4096:
        return {"role": "overlong-disposable-key"}
    return {"role": known.get(path, "other-disposable-entry"),
            "key_sha256": hashlib.sha256(relative.encode("utf-8", "surrogatepass")).hexdigest()}


def snapshot_failure_facts(local, root):
    state, roots = local.get("state"), local.get("roots")
    if type(state) is not dict or type(roots) is not tuple:
        return {"category": "snapshot-context-unavailable"}
    entries, count, overlong = [], 0, 0
    for key, value in state.items():
        if type(key) is not str or type(value) is not str or value != "unreadable":
            continue
        count += 1
        if len(key) > 4096:
            overlong += 1
            continue
        if len(entries) >= DIAGNOSTIC_LIMIT:
            continue
        roles = []
        for index, protected in enumerate(roots[:DIAGNOSTIC_LIMIT]):
            if isinstance(protected, Path) and key.startswith(protected.name + "/"):
                roles.append({"root_index": index, **diagnostic_path(protected, root)})
        entries.append({"key_sha256": hashlib.sha256(key.encode("utf-8", "surrogatepass")).hexdigest(),
                        "root_candidates": roles})
    return {"category": "protected-snapshot-unreadable", "unreadable_count": count,
            "entries": entries, "entries_omitted": max(0, count - len(entries)),
            "overlong_keys": overlong}


def directory_failure_facts(local, root):
    path, info = local.get("path"), local.get("info")
    if not isinstance(path, Path) or not isinstance(info, os.stat_result):
        return {"category": "directory-context-unavailable"}
    attributes = getattr(info, "st_file_attributes", None)
    return {"category": "directory-guard-refused", **diagnostic_path(path, root),
            "held_directory": stat.S_ISDIR(info.st_mode),
            "held_symlink": stat.S_ISLNK(info.st_mode),
            "held_reparse": bool(attributes & 0x400) if type(attributes) is int else None,
            "canonical_relation": "not-retained-by-guard"}


def sanitized_failure_diagnostics(error, repo, root):
    """Project held guard facts; no native bytes, paths, or error messages."""
    scripts = (repo / "scripts").resolve()
    chain, guards = [], []
    cursor = error.__traceback__
    while cursor is not None:
        frame = cursor.tb_frame
        filename = Path(frame.f_code.co_filename)
        function = frame.f_code.co_name
        if (filename.parent.resolve() == scripts
                and re.fullmatch(r"[A-Za-z0-9_-]{1,80}\.py", filename.name)
                and re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]{0,79}", function)
                and 0 < cursor.tb_lineno < 100000):
            chain.append({"file": filename.name, "line": cursor.tb_lineno, "function": function})
            guard = None
            if (filename.name, function) == ("live-install-audit.py", "scoped_snapshot"):
                guard = snapshot_failure_facts(frame.f_locals, root)
            elif (filename.name, function) == ("codex_runtime_audit.py", "_real_directory"):
                guard = directory_failure_facts(frame.f_locals, root)
            if guard is not None:
                guards.append(guard)
        cursor = cursor.tb_next
    return {"source_chain": chain[-DIAGNOSTIC_LIMIT:],
            "source_chain_omitted": max(0, len(chain) - DIAGNOSTIC_LIMIT),
            "guards": guards[-DIAGNOSTIC_LIMIT:],
            "guards_omitted": max(0, len(guards) - DIAGNOSTIC_LIMIT)}


def main():
    if os.name != "nt":
        raise SystemExit("genuine Windows runner required")
    repo = Path(__file__).resolve().parent.parent
    fixture_path = repo / "tests/live-install/fixtures.json"
    report = Path(os.environ["RUNNER_TEMP"]) / "remotion-windows-published-0.1.102.json"
    # Windows checkout may convert LF to CRLF. Bind the immutable fixture to
    # Git's committed object, then accept only that exact platform conversion
    # in the disposable worktree; never invent or rewrite fixture bytes.
    fixture_bytes = subprocess.check_output(
        ["git", "show", "HEAD:tests/live-install/fixtures.json"], cwd=repo)
    checkout_bytes = fixture_path.read_bytes()
    if hashlib.sha256(fixture_bytes).hexdigest() != FIXTURE_SHA256:
        report.write_text(json.dumps({"stage": "fixture", "reason": "git-blob-digest"}) + "\n")
        raise SystemExit("immutable Git fixture SHA-256 mismatch")
    if checkout_bytes not in (fixture_bytes, fixture_bytes.replace(b"\n", b"\r\n")):
        report.write_text(json.dumps({"stage": "fixture", "reason": "unexpected-worktree-bytes"}) + "\n")
        raise SystemExit("checkout modified immutable fixture beyond Windows EOL conversion")
    fixtures = json.loads(fixture_bytes)
    item = fixtures["skills"][24]
    if item["name"] != "remotion-best-practices" or REMOTION_SHA not in item["source"]:
        raise SystemExit("wrong immutable Remotion skill source")

    module_path = repo / "scripts/live-install-audit.py"
    spec = importlib.util.spec_from_file_location("uap_remotion_audit", module_path)
    audit = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(audit)
    audit.validate_fixtures(fixtures)

    shim = shutil.which("agentplugins")
    if not shim:
        raise SystemExit("published npm CLI shim missing")
    cli = Path(shim).resolve(strict=True)
    if not cli.is_file() or cli.suffix.lower() not in (".cmd", ".exe"):
        raise SystemExit("published Windows CLI shim invalid")
    version = subprocess.run([str(cli), "version"], capture_output=True, text=True,
                             errors="replace", timeout=30)
    if version.returncode != 0 or version.stdout.strip() != "agentplugins 0.1.102":
        raise SystemExit("published npm CLI version does not match 0.1.102")
    short = Path(tempfile.mkdtemp(prefix="uap-remotion-short-", dir=os.environ["RUNNER_TEMP"]))
    long = Path(tempfile.mkdtemp(prefix="uap-live-30x30-"))
    evidence = {
        "source_checkout_sha": subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=repo, text=True).strip(),
        "fixture_sha256": FIXTURE_SHA256,
        "fixture_worktree_eol": ("unchanged" if checkout_bytes == fixture_bytes else "crlf-only"),
        "remotion_commit": REMOTION_SHA,
        "cli_path_kind": "published-npm-windows",
        "published_version": version.stdout.strip(),
        "published_tag": "agentplugins-v0.1.102",
        "results": {},
    }
    failures = []
    try:
        short_length = len(str(short / "skill-24"))
        long_length = len(str(long / "skill-24"))
        if long_length < 65 or long_length <= short_length:
            raise RuntimeError("default Windows TEMP is not longer than runner TEMP")
        for label, root in (("runner-temp", short), ("default-windows-temp", long)):
            try:
                row = audit.do_case(str(cli), "codex", "skill", item, 24, root,
                                    fixtures["plugin_declared_names"], fixtures["signed_release_pins"])
                evidence["results"][label] = {"root_chars": len(str(root / "skill-24")), "case": row}
                if row.get("status") != "passed":
                    failures.append(f"{label}: native E2E case did not PASS")
            except Exception as error:
                site = sanitized_failure_site(error, repo)
                try:
                    diagnostic = sanitized_failure_diagnostics(error, repo, root)
                except Exception:
                    diagnostic = {"category": "diagnostic-unavailable"}
                # The workflow log and artifact identify only an allowed source
                # coordinate, never exception text, paths or native profile data.
                failures.append(f"{label}: {type(error).__name__} at "
                                f"{site['file']}:{site['line']}:{site['function']}")
                evidence["results"][label] = {"root_chars": len(str(root / "skill-24")),
                                              "error_type": type(error).__name__,
                                              "source_site": site,
                                              "failure_diagnostics": diagnostic}
    finally:
        report.write_text(json.dumps(evidence, indent=2) + "\n", encoding="utf-8")
        for root in (short, long):
            shutil.rmtree(root, ignore_errors=True)
    if failures:
        raise SystemExit("; ".join(failures))
    print("Published npm 0.1.102 Codex Remotion lifecycle PASS in both Windows TEMP profiles")


if __name__ == "__main__":
    main()
