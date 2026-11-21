#!/usr/bin/env python3
"""Replay bounded plan research in a disposable worktree; no product patches."""

import argparse
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile

BASE = "5f63d48bf4461600e6bb916a6e4997f9830545e1"
HERE = Path(__file__).resolve().parent
SPIKES = ("plan_state_spike_test.go.txt", "plan_ux_spike_test.go.txt")
REQUIRED_PASSES = {
    "TestPlanStateSpikeRetainedInstallation", "TestPlanStateSpikeRootDependenciesNoSave",
    "TestPlanStateSpikeUnrelatedUnmanagedSync", "TestPlanUXRemediationParserSpike",
}
CLI_CASES = (
    "TestPlanStateSpike.*|TestPlanUXRemediationParserSpike|"
    "TestSavedAddAutomaticTargetsDeferUnqualifiedHostWithoutProbing|"
    "TestSavedAddDeferredHostCancelOrDeselectNeverProbes|"
    "TestSavedAddDefaultsOnlySafelyDetectedTargets|"
    "TestSavedAddExplicitTargetKeepsInstallerAdmission|"
    "TestSkippedReasonsSurviveZeroSingleAndMultipleChoices"
)
SUITES = {
    "cli": ["./cli/internal/agentpluginscli", "-run", "^(" + CLI_CASES + ")$"],
    "reliability": [
        "./install/integrationctl/agentplugins/environment",
        "./install/integrationctl/agentplugins/environmentapp",
        "./install/integrationctl/agentplugins/transaction",
        "-run", "Test(Apply|SyncError|Environment|RecoveryAuthority|Observe|LifecycleFacts)",
    ],
}


def checked(*argv, cwd=None, env=None):
    return subprocess.check_output(argv, cwd=cwd, env=env, text=True).strip()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--go", default="go", help="Existing Go toolchain; never downloaded")
    parser.add_argument("--output", required=True, type=Path, help="New directory for logs")
    args = parser.parse_args()
    go = shutil.which(args.go)
    if not go:
        parser.error("Go executable not found; pass --go /path/to/go")
    go = str(Path(go).resolve())
    for filename in SPIKES:
        if not (HERE / filename).is_file():
            parser.error("Missing research source: " + filename)
    repo = checked("git", "rev-parse", "--show-toplevel", cwd=HERE)
    checked("git", "cat-file", "-e", BASE + "^{commit}", cwd=repo)
    env = dict(os.environ, GOTOOLCHAIN="local", GOPROXY="off", GOMAXPROCS="2")
    version = checked(go, "version", env=env)
    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=False)
    report = ["PLAN RESEARCH — filtered Go JSON events", "Base: " + BASE,
              version, "Offline: GOTOOLCHAIN=local GOPROXY=off GOMAXPROCS=2", ""]
    report.append("Optional GC settings: " + " ".join(key + "=" + env.get(key, "default")
                                                     for key in ("GOGC", "GOMEMLIMIT")))
    summaries = {}
    with tempfile.TemporaryDirectory(prefix="uap-plan-") as temporary:
        worktree = Path(temporary) / "checkout"
        checked("git", "worktree", "add", "--detach", str(worktree), BASE, cwd=repo)
        try:
            for filename in SPIKES:
                source = HERE / filename
                target = worktree / "cli/internal/agentpluginscli" / source.name.removesuffix(".txt")
                with target.open("xb") as stream:
                    stream.write(source.read_bytes())
            for name, selection in SUITES.items():
                command = [go, "test", "-json", "-count=1", "-p", "2", "-mod=readonly", *selection]
                with (output / (name + ".jsonl")).open("wb") as stdout, (output / (name + ".stderr")).open("wb") as stderr:
                    result = subprocess.run(command, cwd=worktree, env=env, stdout=stdout, stderr=stderr)
                events = [json.loads(line) for line in (output / (name + ".jsonl")).read_text().splitlines()]
                counts = {action: sum(e.get("Action") == action and "Test" in e for e in events)
                          for action in ("run", "pass", "fail", "skip")}
                packages = [e["Package"] for e in events if e.get("Action") == "pass" and "Test" not in e]
                top_level = sum(e.get("Action") == "pass" and "Test" in e and "/" not in e["Test"] for e in events)
                missing = sorted(REQUIRED_PASSES - {e.get("Test") for e in events if e.get("Action") == "pass"}) if name == "cli" else []
                summaries[name] = dict(exit_code=result.returncode, **counts,
                                       top_level_pass=top_level, packages=packages, missing_spikes=missing)
                report.extend(["SUITE " + name, "ARGV " + json.dumps(["go", *command[1:]]),
                               json.dumps(summaries[name], sort_keys=True)])
                for event in events:
                    action = event.get("Action")
                    if action in ("pass", "fail", "skip") and "Test" in event:
                        report.append(action.upper() + " " + event["Test"])
                    message = event.get("Output", "").strip()
                    if any(marker in message for marker in (
                        "ACTUAL SAVED CANDIDATE", "ASSERT unchanged",
                        "actual Cobra remove argv", "ERROR source", "ERROR installation",
                    )):
                        report.append("EVIDENCE " + message)
                report.append("")
        finally:
            # Only the private temporary worktree created above is removed.
            checked("git", "worktree", "remove", "--force", str(worktree), cwd=repo)
    (output / "results.txt").write_text("\n".join(report).rstrip() + "\n")
    (output / "summary.json").write_text(json.dumps(summaries, indent=2) + "\n")
    print(json.dumps(summaries, indent=2))
    # Empty selections and skipped cases cannot qualify this evidence run.
    return 0 if all(s["exit_code"] == 0 and s["run"] == s["pass"] > 0
                    and s["fail"] == s["skip"] == 0 and not s["missing_spikes"]
                    for s in summaries.values()) else 1


if __name__ == "__main__":
    raise SystemExit(main())
