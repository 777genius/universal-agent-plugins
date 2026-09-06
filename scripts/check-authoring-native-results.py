#!/usr/bin/env python3
"""Fail closed on missing native proof; consume Go JSON, never execute a product.

An offline E2E test under internal/authoring must log, after successful execution:
AUTHORING_NATIVE_E2E {"entrypoint":"agentplugins","sha256":"<executed binary hash>"}
(and likewise plugin-kit-ai), with revision, read_profile, templates and SDK lock
evidence. Only the passing compiled vertical slice and its five template journeys
qualify. Use AUTHORING_NATIVE_BIN_DIR for the built binaries.
"""
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import sys


def unavailable_diagnostic(text, test):
    """Ignore only complete Go status lines naming this event's exact test."""
    name = re.escape(test)
    structural = (rf"[ \t]*(?:=== (?:RUN|PAUSE|CONT) +{name}|"
                  rf"--- (?:PASS|FAIL|SKIP): {name} \([0-9]+\.[0-9]+s\))")
    for line in text.splitlines():
        if test and re.fullmatch(structural, line):
            continue
        if re.search(r"platform_unavailable|not[ _-]?available|gate.*(?:incomplete|unproven)", line, re.I):
            return True
    return False


def check(root):
    evidence = root / "evidence"
    evidence.mkdir(parents=True, exist_ok=True)
    errors, skips, proofs, support = [], [], [], []
    summary = {"status": "unproven", "errors": errors, "skips": skips, "e2e": proofs,
               "support_packages": support, "scope": "linux-windows-offline-authoring-checkpoint",
               "release_gates": {"macos_writable": "unproven"}}

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
        require(native_os in ("linux", "windows"), "checkpoint supports Linux and Windows only; macOS writable release gate UNPROVEN")
        require(native_os == env["GOHOSTOS"] == env["GOOS"] == os.environ["EXPECTED_OS"], "native OS mismatch")
        require(native_arch == env["GOHOSTARCH"] == env["GOARCH"] == os.environ["EXPECTED_ARCH"], "native architecture mismatch")
        require(env["GOVERSION"] == "go1.25.13", "Go pin mismatch")
        suffix = ".exe" if native_os == "windows" else ""
        summary["go_sha256"] = digest(Path(env["GOROOT"]) / "bin" / ("go" + suffix))
        binaries = {name: digest(root / "bin" / (name + suffix)) for name in ("agentplugins", "plugin-kit-ai")}
        summary["binary_sha256"] = binaries
        if native_os == "windows":
            import ctypes
            volume = Path(root).anchor
            filesystem = ctypes.create_unicode_buffer(32)
            require(ctypes.windll.kernel32.GetDriveTypeW(volume) == 3, "Windows fixture volume is not fixed-drive")
            require(bool(ctypes.windll.kernel32.GetVolumeInformationW(volume, None, 0, None, None, None, filesystem, len(filesystem))), "cannot inspect Windows fixture filesystem")
            require(filesystem.value == "NTFS", "Windows native profile requires NTFS")
            summary["filesystem"] = {"type": filesystem.value, "profile": "local-fixed-drive-NTFS"}
        elif native_os == "linux":
            # Capture the actual fixture mount; no mount/provisioning operation.
            mounts = []
            for line in Path("/proc/self/mountinfo").read_text().splitlines():
                left, right = line.split(" - ", 1)
                mount = re.sub(r"\\([0-7]{3})", lambda m: chr(int(m[1], 8)), left.split()[4])
                if Path(root).resolve().is_relative_to(mount):
                    mounts.append((len(mount), right.split()[0], left.split()[5]))
            _, filesystem, options = max(mounts)
            summary["filesystem"] = {"type": filesystem, "mount_options": options, "profile": "packageview-local-linux-v1"}
        packages = (evidence / "packages.txt").read_text(encoding="utf-8").splitlines()
        require(len(packages) >= 6, "missing required package discovery")
        required = ("adapters/packageview", "adapters/packagedigest", "conformance", "adapters/loader", "internal/authoringcli", "internal/authoring/scaffold", "internal/authoring/project", "internal/authoring/commands")
        for tail in required:
            require(any(p.endswith("/" + tail) for p in packages), "missing package: " + tail)
        terminals, passed, output, markers, tested = {}, set(), {}, [], set()
        discovered = set()
        discovery_packages = set()
        for line in (evidence / "test-discovery.json").read_text(encoding="utf-8").splitlines():
            event = json.loads(line)
            require(event["Action"] != "fail", "test discovery failed")
            if event["Action"] == "output" and re.fullmatch(r"Test\w+\n", event.get("Output", "")):
                discovered.add((event["Package"], event["Output"].strip()))
            if not event.get("Test") and event["Action"] in ("pass", "skip"):
                discovery_packages.add(event["Package"])
        require(discovery_packages == set(packages), "incomplete test discovery")
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
                if unavailable_diagnostic(text, test):
                    errors.append(f"unavailable native evidence: {package}/{test}: {text.strip()}")
                if "AUTHORING_NATIVE_E2E " in text:
                    markers.append((key, json.loads(text.split("AUTHORING_NATIVE_E2E ", 1)[1])))
            elif action in ("pass", "fail", "skip"):
                if not test:
                    require(package not in terminals, "duplicate package terminal: " + package)
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
        require(set(terminals) == set(packages), "unexpected/missing executed package")
        for key in sorted(discovered):
            require(key in passed, "discovered test did not execute and pass: " + "/".join(key))
        require(bool(discovered), "empty test discovery")
        for package in packages:
            if package == report and any(p["package"] == package for p in support):
                require(package not in tested and terminals.get(package) == "skip", "support package contained tests or inconsistent outcome: " + package)
                continue
            require(terminals.get(package) == "pass", "package did not pass: " + package)
            require(any(p == package for p, _ in passed), "no executed passing tests: " + package)
        for key, marker in markers:
            name = marker.get("entrypoint")
            require(key in passed and key[0] in packages and key[0].endswith("/internal/authoring/commands") and key[1] == "TestNativeBinaryVerticalSlice", "E2E marker must belong to passing compiled vertical slice test")
            require(name in binaries and marker.get("sha256") == binaries.get(name), "E2E executed binary hash mismatch")
            require(marker.get("revision") == head, "executed E2E revision mismatch")
            require(marker.get("read_profile") == f"packageview-local-{native_os}-v1", "executed E2E read profile mismatch")
            templates = {"skill", "mcp-remote", "mcp-stdio", "hybrid", "hybrid-remote"}
            require(set(marker.get("templates", [])) == templates, "incomplete template journeys")
            for template in sorted(templates):
                require((key[0], key[1] + "/" + template) in passed, "missing passing native template: " + template)
            sdk = marker.get("sdk", {})
            require(bool(sdk.get("@modelcontextprotocol/sdk")) and bool(re.fullmatch(r"[0-9a-f]{64}", sdk.get("package-lock-sha256", ""))), "missing generated SDK lock evidence")
            proofs.append({"package": key[0], "test": key[1], **marker})
        require(len(proofs) == 2, "expected exactly two compiled E2E markers")
        require({p.get("entrypoint") for p in proofs} == set(binaries), "missing compiled offline E2E proof for both entrypoints")
        native_package = "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packageview"
        critical = {
            "linux": ["TestDeviceMetadataOnly", "TestReplacementRacesNeverFollowSpecialOrOutside/before-name-check/device", "TestReplacementRacesNeverFollowSpecialOrOutside/after-name-check/device"],
            "windows": ["TestNativeCaptureAndCleanup", "TestNativeTraversalOrderAndBoundedLinks", "TestNativeLegacyMetadataAndHardlinks", "TestNativeLegacySymlinkAlias", "TestNativeRootLinkRejected",
                        "TestWindowsReplacementBeforeAndAfterNameCheck", "TestWindowsReplacementWithPipeNamespaceLink", "TestWindowsSameObjectReopenAfterNameReplacement", "TestWindowsMetadataProbeReplacement", "TestWindowsExistingWriterAndReparseSetterDenied", "TestWindowsAttributesOnlyHandleCannotSetReparse", "TestWindowsDirectoryAncestryHeld", "TestWindowsInventoryMutationRejected", "TestWindowsJunctionsAndNamespaceRoots", "TestWindowsInertFIFOReparseRejected", "TestWindowsHandleLifetimeAndFailureCleanup", "TestWindowsOfflineFileHasNoDataOpen",
                        "TestWindowsScratchPhysicalAliases", "TestWindowsScratchIdentityUnavailableFailsClosed", "TestWindowsScratchAncestryHeld", "TestWindowsScratchDistinctNTFSVolumes"],
        }
        for test in critical.get(native_os, []):
            require((native_package, test) in passed, "missing mandatory native contract: " + test)
        command_package = "github.com/777genius/plugin-kit-ai/cli/internal/authoring/commands"
        path_contracts = ["ordinary-relative-roots", "traversal-order", "reparse-before-dot-dot"]
        if native_os == "windows":
            path_contracts += ["namespace-rejection", "same-volume-overlapping-alias", "two-physical-ntfs-volumes"]
        for contract in path_contracts:
            test = "TestNativePathFixBothBinaries/" + contract
            require((command_package, test) in passed, "missing mandatory both-binary path contract: " + test)
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
