"""Native evidence controls plus Linux run34015662605/job101438843915 regression."""
import importlib.util
from pathlib import Path
import unittest
from unittest import mock
import contextlib
import ctypes
import hashlib
import io
import json
import os
import tempfile

SPEC = importlib.util.spec_from_file_location(
    "checker", Path(__file__).with_name("check-authoring-native-results.py"))
checker = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(checker)
# Exact downloaded output events; fixture data, never native execution proof.
ACTUAL = [{'Time': '2026-09-06T06:08:38.158231092Z',
  'Action': 'output',
  'Package': 'github.com/777genius/plugin-kit-ai/cli/internal/authoring/skills',
  'Test': 'TestSkillContainmentAndSourceGate/incomplete',
  'Output': '=== RUN   TestSkillContainmentAndSourceGate/incomplete\n'},
 {'Time': '2026-09-06T06:08:38.161863805Z',
  'Action': 'output',
  'Package': 'github.com/777genius/plugin-kit-ai/cli/internal/authoring/skills',
  'Test': 'TestSkillContainmentAndSourceGate/incomplete',
  'Output': '--- PASS: TestSkillContainmentAndSourceGate/incomplete (0.00s)\n'}]


class UnavailableDiagnosticTests(unittest.TestCase):
    def test_actual_failing_events(self):
        for event in ACTUAL:
            with self.subTest(output=event["Output"]):
                self.assertFalse(checker.unavailable_diagnostic(
                    event["Output"], event["Test"]))

    def test_go_status_names(self):
        for name in (ACTUAL[0]["Test"], "TestCapability/platform_unavailable",
                     "TestCapability/not-available", "TestGate/unproven", "TestCapability/scratch_unavailable"):
            for line in (f"=== RUN   {name}", f"=== PAUSE {name}",
                         f"=== CONT  {name}", f"--- PASS: {name} (0.00s)",
                         f"--- FAIL: {name} (1.23s)", f"--- SKIP: {name} (0.00s)"):
                with self.subTest(line=line):
                    self.assertFalse(checker.unavailable_diagnostic("    " + line + "\n", name))

    def test_real_diagnostics_still_rejected(self):
        name = ACTUAL[0]["Test"]
        for warning in ("platform_unavailable", "not available", "not_available",
                        "not-available", "NOT AVAILABLE", "gate incomplete",
                        "gate unproven", "scratch_unavailable", name):
            for text in (warning, "    native_test.go:42: " + warning + "\n",
                         ACTUAL[0]["Output"] + warning + "\n" + ACTUAL[1]["Output"],
                         ACTUAL[1]["Output"].rstrip() + " " + warning):
                with self.subTest(text=text):
                    self.assertTrue(checker.unavailable_diagnostic(text, name))

    def test_structure_must_match_whole_line_and_event_identity(self):
        for event in ACTUAL:
            text, name = event["Output"], event["Test"]
            for output, test in ((text, ""), (text, "TestOther"),
                                 ("native_test.go:42: " + text, name),
                                 (text.rstrip() + " warning\n", name)):
                with self.subTest(output=output, test=test):
                    self.assertTrue(checker.unavailable_diagnostic(output, test))

    def test_test_names_are_literal(self):
        name = "TestGate/incomplete[1]"
        self.assertFalse(checker.unavailable_diagnostic(f"=== RUN   {name}\n", name))
        self.assertTrue(checker.unavailable_diagnostic("=== RUN   TestGate/incomplete1\n", name))


# Synthetic passing transcripts, never claims of native execution.
CONTRACTS = {
    'linux': [
        'TestDeviceMetadataOnly',
        'TestReplacementRacesNeverFollowSpecialOrOutside/before-name-check/device',
        'TestReplacementRacesNeverFollowSpecialOrOutside/after-name-check/device',
    ],
    'windows': [
        'TestWindowsPureNamespaceReplacement/before_name_check',
        'TestWindowsPureNamespaceReplacement/after_name_check',
        'TestWindowsFinalCleanupAcquisitionStages',
        'TestWindowsSharedRenameCalibration/regular',
        'TestWindowsSharedRenameCalibration/directory',
        'TestWindowsNTSelfOpenAccessAndRead',
        'TestWindowsNTSelfOpenDirectoryAfterNameReplacement',
        'TestWindowsNTSelfOpenExistingConflictsAndCleanup',
        'TestWindowsNTSelfOpenInvalidHandles',
        'TestWindowsBootstrapDirectoryGuard',
        'TestWindowsBootstrapDirectoryGuardWrongKind',
        'TestWindowsBootstrapDirectoryGuardUnsafe',
        'TestNativeCaptureAndCleanup',
        'TestNativeTraversalOrderAndBoundedLinks',
        'TestNativeLegacyMetadataAndHardlinks',
        'TestNativeLegacySymlinkAlias',
        'TestNativeRootLinkRejected',
        'TestWindowsReplacementBeforeAndAfterNameCheck',
        'TestWindowsReplacementWithPipeNamespaceLink',
        'TestWindowsSameObjectReopenAfterNameReplacement',
        'TestWindowsMetadataProbeReplacement',
        'TestWindowsExistingWriterAndReparseSetterDenied',
        'TestWindowsAttributesOnlyHandleCannotSetReparse',
        'TestWindowsDirectoryAncestryHeld',
        'TestWindowsInventoryMutationRejected',
        'TestWindowsJunctionsAndNamespaceRoots',
        'TestWindowsInertFIFOReparseRejected',
        'TestWindowsHandleLifetimeAndFailureCleanup',
        'TestWindowsOfflineFileHasNoDataOpen',
        'TestWindowsScratchPhysicalAliases',
        'TestWindowsScratchIdentityUnavailableFailsClosed',
        'TestWindowsScratchAncestryHeld',
        'TestWindowsScratchDistinctNTFSVolumes',
        'TestWindowsBasicInfoABI',
        'TestWindowsBootstrapStages',
        'TestWindowsScratchAliasResolutionStages',
        'TestWindowsReopenProtectedParameters',
        'TestWindowsModeAndChangeMetadata',
        'TestWindowsRootAndIntermediateReparseRejected',
        'TestWindowsScratchAliasFailuresStayBeforeData',
    ],
}


def executable(system, arch):
    header = bytearray(64)
    if system == "linux":
        header[:7] = b"\x7fELF\x02\x01\x01"
        header[18:20] = {"amd64": 62, "arm64": 183}[arch].to_bytes(2, "little")
    else:
        header[:2] = b"MZ"
        header[60:64] = (64).to_bytes(4, "little")
        pe = bytearray(26)
        pe[:4] = b"PE\0\0"
        pe[4:6] = {"amd64": 0x8664, "arm64": 0xAA64}[arch].to_bytes(2, "little")
        pe[24:26] = b"\x0b\x02"
        header += pe
    return header


class NativeEvidenceTests(unittest.TestCase):
    def fixture(self, system, arch):
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        root = Path(temp.name)
        evidence = root / "evidence"
        evidence.mkdir()
        (root / "bin").mkdir()
        (root / "go" / "bin").mkdir(parents=True)
        head = "a" * 40
        (evidence / "head.txt").write_text(head)
        self.system, self.arch, self.root = system, arch, root
        self.env = dict(GOOS=system, GOHOSTOS=system, GOARCH=arch, GOHOSTARCH=arch,
                        GOVERSION="go1.25.13", GOROOT=str(root / "go"))
        self.expected = dict(EXPECTED_OS=system, EXPECTED_ARCH=arch, EXPECTED_HEAD=head,
                             RUNNER_ARCH="X64" if arch == "amd64" else "ARM64")
        self.suffix = ".exe" if system == "windows" else ""
        (root / "go" / "bin" / ("go" + self.suffix)).write_bytes(executable(system, arch))
        self.builds = {}
        for name in ("agentplugins", "plugin-kit-ai"):
            (root / "bin" / (name + self.suffix)).write_bytes(executable(system, arch))
            settings = dict(GOOS=system, GOARCH=arch, **{"vcs.revision": head, "vcs.modified": "false"})
            self.builds[name] = dict(GoVersion="go1.25.13",
                Path="github.com/777genius/plugin-kit-ai/cli/cmd/" + name,
                Settings=[dict(Key=k, Value=v) for k, v in settings.items()])
        prefix = "github.com/777genius/plugin-kit-ai/"
        self.native = prefix + "install/integrationctl/agentplugins/adapters/packageview"
        self.commands = prefix + "cli/internal/authoring/commands"
        scaffold = prefix + "cli/internal/authoring/scaffold"
        packages = [self.native, self.commands, scaffold,
            prefix + "install/integrationctl/agentplugins/adapters/packagedigest",
            prefix + "install/integrationctl/agentplugins/conformance",
            prefix + "install/integrationctl/agentplugins/adapters/loader",
            prefix + "cli/internal/authoringcli", prefix + "cli/internal/authoring/project"]
        (evidence / "packages.txt").write_text("\n".join(packages))
        tests = {(p, "TestFixture") for p in packages}
        tests.update((self.native, name) for name in CONTRACTS[system])
        tests.add((scaffold, "TestDeniedParent"))
        templates = ["skill", "mcp-remote", "mcp-stdio", "hybrid", "hybrid-remote"]
        tests.update((self.commands, "TestNativeBinaryVerticalSlice/" + t) for t in templates)
        tests.add((self.commands, "TestNativeBinaryVerticalSlice"))
        tests.update((self.commands, "TestNativeBinaryLifecycle/" + t) for t in
            ("generated-skill", "skills-init", "static-validation", "duplicate-unchanged",
             "readiness", "strict-flags", "malformed-skill", "sdk-static-only"))
        paths = ["ordinary-relative-roots", "traversal-order", "reparse-before-dot-dot"]
        if system == "windows":
            paths += ["namespace-rejection", "same-volume-overlapping-alias", "two-physical-ntfs-volumes"]
        tests.update((self.commands, "TestNativePathFixBothBinaries/" + t) for t in paths)
        self.events = [dict(Action="pass", Package=p, Test=t) for p, t in sorted(tests)]
        self.events += [dict(Action="pass", Package=p) for p in packages]
        self.discovery = [dict(Action="output", Package=p, Output=t + "\n")
                          for p, t in sorted(tests) if "/" not in t]
        self.discovery += [dict(Action="pass", Package=p) for p in packages]
        self.markers = [dict(entrypoint=name, sha256=hashlib.sha256(executable(system, arch)).hexdigest(),
            revision=head, read_profile=f"packageview-local-{system}-v1", templates=templates,
            sdk={"@modelcontextprotocol/sdk": "locked", "package-lock-sha256": "b" * 64})
            for name in self.builds]
        (evidence / "test-exit.txt").write_text("0")

    def check(self):
        evidence = self.root / "evidence"
        (evidence / "go-env.json").write_text(json.dumps(self.env))
        for name, build in self.builds.items():
            (evidence / (name + "-build.json")).write_text(json.dumps(build))
        events = self.events + [dict(Action="output", Package=self.commands,
            Test="TestNativeBinaryVerticalSlice", Output="AUTHORING_NATIVE_E2E " + json.dumps(m))
            for m in self.markers]
        for file, data in (("go-test.json", events), ("test-discovery.json", self.discovery)):
            (evidence / file).write_text("\n".join(json.dumps(e) for e in data))
        def volume(*args):
            args[6].value = "NTFS"
            return 1
        kernel = mock.Mock()
        kernel.GetDriveTypeW.return_value = 3
        kernel.GetVolumeInformationW.side_effect = volume
        with mock.patch.dict(os.environ, self.expected), \
             mock.patch.object(checker, "host_identity", return_value=(self.system, self.arch)), \
             mock.patch.object(ctypes, "windll", mock.Mock(kernel32=kernel), create=True), \
             contextlib.redirect_stdout(io.StringIO()):
            result = checker.check(self.root)
        summary = json.loads((evidence / "summary.json").read_text())
        self.assertEqual(result, bool(summary["errors"]))
        return summary

    def rejected(self, message):
        summary = self.check()
        self.assertEqual(summary["status"], "unproven", summary)
        self.assertTrue(any(message in e for e in summary["errors"]), summary["errors"])

    def test_four_lane_controls(self):
        for system in ("linux", "windows"):
            for arch in ("amd64", "arm64"):
                with self.subTest(system=system, arch=arch):
                    self.fixture(system, arch)
                    self.assertEqual(self.check()["errors"], [])

    def test_architecture_mismatch(self):
        for system in ("linux", "windows"):
            for field in ("GOARCH", "GOHOSTARCH"):
                with self.subTest(system=system, field=field):
                    self.fixture(system, "arm64")
                    self.env[field] = "amd64"
                    self.rejected("native architecture mismatch")
            self.fixture(system, "arm64")
            self.expected["EXPECTED_ARCH"] = "amd64"
            self.rejected("native architecture mismatch")
            self.fixture(system, "arm64")
            self.expected["RUNNER_ARCH"] = "X64"
            self.rejected("runner architecture mismatch")
            self.fixture(system, "arm64")
            self.expected["RUNNER_ARCH"] = ""
            self.rejected("runner architecture mismatch")
            self.fixture(system, "arm64")
            self.arch = "amd64"  # all Go evidence says ARM, native kernel does not
            self.rejected("native architecture mismatch")

    def test_emulated_executables_rejected(self):
        for system in ("linux", "windows"):
            for name in ("go", "agentplugins", "plugin-kit-ai"):
                with self.subTest(system=system, executable=name):
                    self.fixture(system, "arm64")
                    directory = self.root / ("go/bin" if name == "go" else "bin")
                    (directory / (name + self.suffix)).write_bytes(executable(system, "amd64"))
                    self.rejected("native executable architecture mismatch: " + name)

    def test_build_revision_arch_and_pin_controls(self):
        for field, value, error in (("GOARCH", "amd64", "build architecture"),
                                   ("vcs.revision", "b" * 40, "source revision"),
                                   ("vcs.modified", "true", "source revision")):
            self.fixture("windows", "arm64")
            for item in self.builds["agentplugins"]["Settings"]:
                if item["Key"] == field:
                    item["Value"] = value
            self.rejected(error)
        self.fixture("linux", "arm64")
        self.builds["plugin-kit-ai"]["GoVersion"] = "go1.25.12"
        self.rejected("binary Go pin mismatch")

    def test_both_binary_journey_identity_controls(self):
        for field, value, error in (("revision", "b" * 40, "E2E revision"),
                                   ("sha256", "0" * 64, "binary hash"),
                                   ("read_profile", "platform_unavailable", "read profile"),
                                   ("templates", ["skill"], "template journeys")):
            for name in ("agentplugins", "plugin-kit-ai"):
                self.fixture("windows", "arm64")
                next(m for m in self.markers if m["entrypoint"] == name)[field] = value
                self.rejected(error)

    def test_missing_contract_is_not_architecture_unavailable_as_pass(self):
        for system in ("linux", "windows"):
            for arch in ("amd64", "arm64"):
                names = (["TestDeviceMetadataOnly"] if system == "linux" else
                    ["TestWindowsScratchDistinctNTFSVolumes", "TestWindowsBasicInfoABI",
                     "TestWindowsNTSelfOpenAccessAndRead", "TestWindowsScratchAliasResolutionStages"])
                for name in names:
                    with self.subTest(system=system, arch=arch, test=name):
                        self.fixture(system, arch)
                        self.events = [e for e in self.events if e.get("Test") != name]
                        self.discovery = [e for e in self.discovery if e.get("Output") != name + "\n"]
                        self.rejected("missing mandatory native contract: " + name)

    def test_required_fixture_skips_and_missing_journeys(self):
        for arch in ("amd64", "arm64"):
            for name in ("TestWindowsScratchDistinctNTFSVolumes", "TestDeniedParent",
                         "TestNativePathFixBothBinaries/two-physical-ntfs-volumes",
                         "TestNativeBinaryLifecycle/sdk-static-only", "TestNativeBinaryVerticalSlice/skill"):
                for action in ("skip", "omit"):
                    with self.subTest(arch=arch, test=name, action=action):
                        self.fixture("windows", arch)
                        if action == "omit":
                            self.events = [e for e in self.events if e.get("Test") != name]
                        else:
                            next(e for e in self.events if e.get("Test") == name)["Action"] = "skip"
                        self.assertEqual(self.check()["status"], "unproven")

    def test_unattributed_scratch_unavailable_recurrence_fails(self):
        self.fixture("windows", "arm64")
        self.events.append(dict(Action="output", Package=self.native,
                                Output="scratch_unavailable\n"))
        self.rejected("unavailable native evidence")


class ExecutableIdentityTests(unittest.TestCase):
    def test_malformed_and_hybrid_headers(self):
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp) / "binary"
            for data in (b"", b"MZ", bytes(90)):
                path.write_bytes(data)
                with self.assertRaises(ValueError):
                    checker.executable_identity(path)
            for machine in (0xA641, 0xA64E, 0x14C):  # ARM64EC, ARM64X, x86
                data = executable("windows", "arm64")
                data[68:70] = machine.to_bytes(2, "little")
                path.write_bytes(data)
                self.assertEqual(checker.executable_identity(path), ("windows", None))

    def test_windows_kernel_identity_ignores_emulated_python_architecture(self):
        kernel = mock.Mock()
        def query(process, process_machine, native_machine):
            process_machine._obj.value = 0x8664
            native_machine._obj.value = 0xAA64
            return 1
        kernel.IsWow64Process2.side_effect = query
        with mock.patch.object(checker.platform, "system", return_value="Windows"), \
             mock.patch.object(checker.platform, "machine", return_value="AMD64"), \
             mock.patch.object(ctypes, "WinDLL", return_value=kernel, create=True):
            self.assertEqual(checker.host_identity(), ("windows", "arm64"))
            kernel.IsWow64Process2.side_effect = None
            kernel.IsWow64Process2.return_value = 0
            with self.assertRaises(OSError):
                checker.host_identity()


if __name__ == "__main__":
    unittest.main()
