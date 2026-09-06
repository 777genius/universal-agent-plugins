#!/usr/bin/env python3
"""Fail closed on missing native proof; consume Go JSON, never execute a product.

An offline E2E test under internal/authoring must log, after successful execution:
AUTHORING_NATIVE_E2E {"entrypoint":"agentplugins","sha256":"<executed binary hash>"}
(and likewise plugin-kit-ai). Use AUTHORING_NATIVE_BIN_DIR for the built binaries.
"""
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import sys


def check(root):
    evidence = root / "evidence"
    evidence.mkdir(parents=True, exist_ok=True)
    errors, skips, proofs, support = [], [], [], []
    summary = {"status": "unproven", "errors": errors, "skips": skips, "e2e": proofs,
               "support_packages": support}

    def require(condition, message):
        if not condition:
            errors.append(message)

    def digest(path):
        with path.open("rb") as stream:
            value = hashlib.sha256()
            for block in iter(lambda: stream.read(1024 * 1024), b""):
                value.update(block)
            return value.hexdigest()

    try:
        head = (evidence / "head.txt").read_text(encoding="utf-8").strip()
        summary["head"] = head
        require(bool(re.fullmatch(r"[0-9a-f]{40}", head)) and head == os.environ["EXPECTED_HEAD"], "head mismatch")
        env = json.loads((evidence / "go-env.json").read_text(encoding="utf-8"))
        summary["go"] = env
        native_os = {"Linux": "linux", "Darwin": "darwin", "Windows": "windows"}.get(platform.system())
        native_arch = {"x86_64": "amd64", "amd64": "amd64", "arm64": "arm64", "aarch64": "arm64"}.get(platform.machine().lower())
        summary["host"] = {"os": native_os, "arch": native_arch, "release": platform.release()}
        require(native_os == env["GOHOSTOS"] == env["GOOS"] == os.environ["EXPECTED_OS"], "native OS mismatch")
        require(native_arch == env["GOHOSTARCH"] == env["GOARCH"] == os.environ["EXPECTED_ARCH"], "native architecture mismatch")
        require(env["GOVERSION"] == "go1.25.13", "Go pin mismatch")
        suffix = ".exe" if native_os == "windows" else ""
        summary["go_sha256"] = digest(Path(env["GOROOT"]) / "bin" / ("go" + suffix))
        binaries = {name: digest(root / "bin" / (name + suffix)) for name in ("agentplugins", "plugin-kit-ai")}
        summary["binary_sha256"] = binaries
        packages = (evidence / "packages.txt").read_text(encoding="utf-8").splitlines()
        require(len(packages) >= 6, "missing required package discovery")
        required = ("adapters/packageview", "adapters/packagedigest", "conformance", "adapters/loader", "internal/authoringcli", "internal/authoring/scaffold")
        for tail in required:
            require(any(p.endswith("/" + tail) for p in packages), "missing package: " + tail)
        terminals, passed, output, markers, tested = {}, set(), {}, [], set()
        # Explicitly reviewed pure support package; never exempt a tested package.
        report = "github.com/777genius/plugin-kit-ai/cli/internal/authoring/report"
        for line in (evidence / "go-test.json").read_text(encoding="utf-8").splitlines():
            event = json.loads(line)
            package, test = event.get("Package", ""), event.get("Test", "")
            key = (package, test)
            action = event["Action"]
            if test:
                tested.add(package)
            if action == "output":
                text = event.get("Output", "")
                output[key] = output.get(key, "") + text
                if re.search(r"platform_unavailable|not[ _-]?available|gate.*(?:incomplete|unproven)", text, re.I):
                    errors.append(f"unavailable native evidence: {package}/{test}: {text.strip()}")
                if "AUTHORING_NATIVE_E2E " in text:
                    markers.append((key, json.loads(text.split("AUTHORING_NATIVE_E2E ", 1)[1])))
            elif action in ("pass", "fail", "skip"):
                if not test:
                    terminals[package] = action
                elif action == "pass":
                    passed.add(key)
                if action == "fail":
                    errors.append(f"failed: {package}/{test}")
                if action == "skip":
                    if not test and package == report and "[no test files]" in output.get(key, ""):
                        support.append({"package": package, "output": output[key], "classification": "unproven"})
                        continue
                    # Only the existing optional Unix socket fixture is exempt.
                    # Device, reparse, symlink and permission skips need explicit proof.
                    reason = output.get(key, "")
                    optional = (package.endswith("/adapters/packagedigest") and
                                test == "TestSnapshotRejectsPortablePathAndContentHazards/special_file" and
                                "Unix sockets unavailable:" in reason)
                    skips.append({"package": package, "test": test, "output": reason,
                                  "classification": "optional-unproven" if optional else "required-unproven"})
                    require(optional, f"required fixture skipped; separate explicit proof needed: {package}/{test}")
        summary["packages"] = terminals
        for package in packages:
            if package == report and any(p["package"] == package for p in support):
                require(package not in tested and terminals.get(package) == "skip", "support package contained tests or inconsistent outcome: " + package)
                continue
            require(terminals.get(package) == "pass", "package did not pass: " + package)
            require(any(p == package for p, _ in passed), "no executed passing tests: " + package)
        for key, marker in markers:
            name = marker.get("entrypoint")
            require(key in passed and key[0] in packages and key[0].endswith("/internal/authoring/commands"), "E2E marker must belong to passing authoring commands test")
            require(name in binaries and marker.get("sha256") == binaries.get(name), "E2E executed binary hash mismatch")
            proofs.append({"package": key[0], "test": key[1], **marker})
        require({p.get("entrypoint") for p in proofs} == set(binaries), "missing compiled offline E2E proof for both entrypoints")
        require((evidence / "test-exit.txt").read_text(encoding="utf-8").strip() == "0", "go test exited unsuccessfully")
    except (OSError, ValueError, KeyError, TypeError, AttributeError) as exc:
        errors.append(f"missing or malformed evidence: {exc}")
    summary["status"] = "unproven" if errors else "proven"
    if not errors:
        for package in support:
            package["classification"] = "transitively-covered-by-authoring-e2e"
    (evidence / "summary.json").write_text(json.dumps(summary, indent=2) + "\n", encoding="utf-8")
    print(json.dumps(summary, indent=2))
    return bool(errors)


if __name__ == "__main__":
    sys.exit(check(Path(sys.argv[1])))
