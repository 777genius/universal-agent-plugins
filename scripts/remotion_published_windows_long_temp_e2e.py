"""Real pinned Remotion Skill lifecycle on short and default-long Windows TEMP.

Reuses the existing published/native case auditor with a published npm CLI pinned to release 0.1.102.
No alternate success interpretation, fixture edit or native trust bypass.
"""

import hashlib
import importlib.util
import json
import os
from pathlib import Path
from runpy import run_path
import shutil
import subprocess
import tempfile

FIXTURE_SHA256 = "e22cb3b5b23e74a9e03b55e24a684ab93038f89cf18e28fd5d9755175754c0d9"
REMOTION_SHA = "c7cb2fdcacd134fb2ccb9b7a44f9ffa44e467271"
_TEMP_ROOTS = run_path(str(Path(__file__).with_name("remotion_windows_long_temp_e2e.py")))
temp_root_authority = _TEMP_ROOTS["temp_root_authority"]
physical_temp_root = _TEMP_ROOTS["physical_temp_root"]


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
    evidence = {
        "source_checkout_sha": subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=repo, text=True).strip(),
        "fixture_sha256": FIXTURE_SHA256,
        "fixture_worktree_eol": ("unchanged" if checkout_bytes == fixture_bytes else "crlf-only"),
        "remotion_commit": REMOTION_SHA,
        "cli_path_kind": "published-npm-windows",
        "published_version": version.stdout.strip(),
        "published_tag": "agentplugins-v0.1.102",
        "results": {},
        "temp_roots": {},
    }
    failures = []
    created_roots, physical_roots = {}, {}
    try:
        for label, options in (
                ("runner-temp", {"prefix": "uap-remotion-short-", "dir": os.environ["RUNNER_TEMP"]}),
                ("default-windows-temp", {"prefix": "uap-live-30x30-"})):
            created = Path(tempfile.mkdtemp(**options))
            created_roots[label] = (created, None)
            evidence["temp_roots"][label] = {
                "created_path": str(created), "created_root_chars": len(str(created / "skill-24")),
                "cleanup": "retained-unverified",
            }
            authority = temp_root_authority(created)
            created_roots[label] = (created, authority)
            physical_roots[label] = physical_temp_root(created, authority)
            evidence["temp_roots"][label]["root_chars"] = len(str(physical_roots[label] / "skill-24"))
        short, long = physical_roots["runner-temp"], physical_roots["default-windows-temp"]
        created_lengths = {label: row["created_root_chars"]
                           for label, row in evidence["temp_roots"].items()}
        short_length = len(str(short / "skill-24"))
        long_length = len(str(long / "skill-24"))
        for shorter, longer in ((created_lengths["runner-temp"], created_lengths["default-windows-temp"]),
                                (short_length, long_length)):
            if longer < 65 or longer <= shorter:
                raise RuntimeError("default Windows TEMP is not longer than runner TEMP")
        for label, root in (("runner-temp", short), ("default-windows-temp", long)):
            try:
                row = audit.do_case(str(cli), "codex", "skill", item, 24, root,
                                    fixtures["plugin_declared_names"], fixtures["signed_release_pins"])
                evidence["results"][label] = {"root_chars": len(str(root / "skill-24")),
                                              "created_root_chars": created_lengths[label], "case": row}
                if row.get("status") != "passed":
                    failures.append(f"{label}: native E2E case did not PASS")
            except Exception as error:
                failures.append(f"{label}: {type(error).__name__}")
                evidence["results"][label] = {"root_chars": len(str(root / "skill-24")),
                                              "created_root_chars": created_lengths[label],
                                              "error_type": type(error).__name__,
                                              "error_traceback": audit.bounded_error_traceback(error)}
    except Exception as error:
        failures.append(f"TEMP setup: {type(error).__name__}")
        evidence["setup_error"] = {"error_type": type(error).__name__,
                                   "error_traceback": audit.bounded_error_traceback(error)}
    finally:
        # Keep both created objects for evidence; a later pathname deletion
        # cannot be authorized by an earlier same-object identity check.
        # The unchanged native auditor still requires product uninstall.
        for label, (created, authority) in created_roots.items():
            try:
                if authority is None or temp_root_authority(created) != authority:
                    raise ValueError("created TEMP root authority is unverified or changed")
                evidence["temp_roots"][label]["cleanup"] = "retained-for-evidence"
            except Exception as error:
                failures.append(f"{label}: TEMP ownership {type(error).__name__}")
                evidence["temp_roots"][label].update({
                    "cleanup": "retained-unverified", "cleanup_error_type": type(error).__name__,
                    "cleanup_error_traceback": audit.bounded_error_traceback(error),
                })
        report.write_text(json.dumps(evidence, indent=2) + "\n", encoding="utf-8")
    if failures:
        raise SystemExit("; ".join(failures))
    print("Published npm 0.1.102 Codex Remotion lifecycle PASS in both Windows TEMP profiles")


if __name__ == "__main__":
    main()
