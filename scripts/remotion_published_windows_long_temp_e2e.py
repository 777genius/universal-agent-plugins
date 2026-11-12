"""Real pinned Remotion Skill lifecycle on short and default-long Windows TEMP.

Reuses the existing published/native case auditor with a published npm CLI pinned to release 0.1.102.
No alternate success interpretation, fixture edit or native trust bypass.
"""

import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile

FIXTURE_SHA256 = "7399eaed15b3eb0981a6be12082dd8825529e896a59ea125b9500a666c21112e"
REMOTION_SHA = "c7cb2fdcacd134fb2ccb9b7a44f9ffa44e467271"


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
                failures.append(f"{label}: {type(error).__name__}")
                evidence["results"][label] = {"root_chars": len(str(root / "skill-24")),
                                              "error_type": type(error).__name__}
    finally:
        report.write_text(json.dumps(evidence, indent=2) + "\n", encoding="utf-8")
        for root in (short, long):
            shutil.rmtree(root, ignore_errors=True)
    if failures:
        raise SystemExit("; ".join(failures))
    print("Published npm 0.1.102 Codex Remotion lifecycle PASS in both Windows TEMP profiles")


if __name__ == "__main__":
    main()
