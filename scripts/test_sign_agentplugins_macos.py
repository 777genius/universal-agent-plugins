#!/usr/bin/env python3
"""Fail-closed and secret-hygiene checks for sign-agentplugins-macos.sh.

Apple tools are replaced by recording fakes, so this runs on any POSIX host.
Real signing and notarization are exercised only by the release workflow and
the one-time identity check in docs/agentplugins-release.md.
"""
import base64
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

SCRIPT = Path(__file__).resolve().with_name("sign-agentplugins-macos.sh")
TEAM = "86399583GS"
IDENTITY = f"Developer ID Application: Example Maintainer ({TEAM})"
IDENTITY_HASH = "A" * 40
P12, PASSWORD, P8 = b"fake-p12-bytes", "fake-certificate-password", b"fake-p8-key"
SUBMISSION = "2efe2717-52ef-43a5-96dc-0797e4ca1041"
TAG, VERSION = "agentplugins-v1.2.3", "1.2.3"

FAKE = r'''
import hashlib, json, os, pathlib, sys, zipfile
tool, args = sys.argv[1], sys.argv[2:]
mode = os.environ.get("FAKE_MODE", "")
state_path = pathlib.Path(os.environ["FAKE_STATE"])
state = json.loads(state_path.read_text())
state["calls"].append([tool, *args])
def save():
    state_path.write_text(json.dumps(state))
def cdhash(path):
    return hashlib.sha1(pathlib.Path(path).read_bytes()).hexdigest()
def option(name):
    return args[args.index(name) + 1]
code = 0
if tool == "security":
    command = args[0]
    if command == "list-keychains":
        if "-s" in args:
            state["search"] = args[args.index("-s") + 1:]
        else:
            print("\n".join('    "%s"' % item for item in state["search"]))
    elif command == "create-keychain":
        state["keychain"], state["keychain_password"] = args[-1], option("-p")
        pathlib.Path(args[-1]).write_text("keychain")
    elif command == "unlock-keychain":
        code = 0 if option("-p") == state["keychain_password"] else 1
    elif command == "import":
        ok = pathlib.Path(args[1]).read_bytes() == b"fake-p12-bytes"
        ok = ok and option("-P") == "fake-certificate-password" and option("-T") == "/usr/bin/codesign"
        code = 0 if ok else 1
    elif command == "find-identity":
        line = '  1) %s "Developer ID Application: Example Maintainer (86399583GS)"' % ("A" * 40)
        print(line)
        if mode == "two-identities":
            print(line.replace("1) " + "A" * 40, "2) " + "B" * 40))
        print("     1 valid identities found")
    elif command == "delete-keychain":
        pathlib.Path(args[-1]).unlink()
        state["deleted"].append(args[-1])
elif tool == "codesign":
    target = pathlib.Path(args[-1])
    if "--force" in args:
        state["signed"].append(args)
        target.write_bytes(target.read_bytes() + b"\n# fake developer id signature\n")
    elif "--verify" in args:
        code = 0 if target.read_bytes().endswith(b"# fake developer id signature\n") else 1
    elif "--display" in args:
        team = "AAAAAAAAAA" if mode == "wrong-team" else "86399583GS"
        lines = ["Executable=%s" % target,
                 "Identifier=" + ("a.out" if mode == "wrong-identifier" else "com.777genius.agentplugins"),
                 "CodeDirectory v=20500 size=9 flags=%s hashes=1+7 location=embedded"
                 % ("0x0(none)" if mode == "no-runtime" else "0x10000(runtime)"),
                 "CDHash=" + cdhash(target)]
        if mode == "adhoc":
            lines += ["Signature=adhoc", "TeamIdentifier=not set"]
        else:
            lines += ["Authority=Developer ID Application: Example Maintainer (%s)" % team,
                      "Authority=Developer ID Certification Authority", "Authority=Apple Root CA",
                      "Signed Time=21 Nov 2026" if mode == "no-timestamp" else "Timestamp=21 Nov 2026",
                      "TeamIdentifier=" + team]
        sys.stderr.write("\n".join(lines) + "\n")
elif tool == "lipo":
    amd64 = args[-1].endswith("_darwin_amd64")
    print("x86_64" if amd64 != (mode == "swapped-arch") else "arm64")
elif tool == "ditto":
    source, archive = pathlib.Path(args[-2]), args[-1]
    with zipfile.ZipFile(archive, "w") as stream:
        for child in sorted(source.iterdir()):
            stream.write(child, source.name + "/" + child.name)
    state["archive"] = archive
elif tool == "xcrun":
    assert args[0] == "notarytool"
    key = pathlib.Path(option("--key"))
    assert key.read_bytes() == b"fake-p8-key" and key.stat().st_mode & 0o777 == 0o600, "unsafe key file"
    state["keys"].append(str(key))
    if args[1] == "submit":
        status = "Invalid" if mode == "invalid" else "Accepted"
        print(json.dumps({"id": "2efe2717-52ef-43a5-96dc-0797e4ca1041", "status": status, "message": "done"}))
    else:
        tickets = []
        with zipfile.ZipFile(state["archive"]) as stream:
            for name in stream.namelist():
                digest = hashlib.sha1(stream.read(name)).hexdigest()
                tickets.append({"path": pathlib.Path(state["archive"]).name + "/" + name,
                                "digestAlgorithm": "SHA-256",
                                "cdhash": "0" * 40 if mode == "wrong-cdhash" else digest})
        status = "Invalid" if mode == "invalid" else "Accepted"
        pathlib.Path(args[-1]).write_text(json.dumps({"status": status, "ticketContents": tickets, "issues": None}))
elif tool == "xattr":
    if args[0] == "-w":
        state["quarantined"][args[-1]] = args[-2]
    elif args[-1] not in state["quarantined"]:
        code = 1
elif tool == "spctl":
    sys.stderr.write(args[-1] + ": accepted\nsource=Notarized Developer ID\n")
elif tool == "uname":
    print("arm64")
save()
sys.exit(code)
'''


class SignAgentpluginsMacOSTests(unittest.TestCase):
    def setUp(self):
        temp = tempfile.TemporaryDirectory(prefix="TEST-macos-signing-")
        self.addCleanup(temp.cleanup)
        self.root = Path(temp.name)
        self.bin = self.root / "fakes"
        self.bin.mkdir()
        (self.bin / "fake.py").write_text(FAKE)
        for tool in ("security", "codesign", "lipo", "ditto", "xcrun", "xattr", "spctl", "uname"):
            wrapper = self.bin / tool
            wrapper.write_text(f'#!/bin/sh\nexec python3 "{self.bin / "fake.py"}" {tool} "$@"\n')
            wrapper.chmod(0o755)
        (self.bin / "sleep").write_text("#!/bin/sh\nexit 0\n")
        (self.bin / "sleep").chmod(0o755)
        self.binaries = self.root / "darwin"
        self.binaries.mkdir()
        self.block = self.root / "gatekeeper-block"
        for target in ("darwin_amd64", "darwin_arm64"):
            path = self.binaries / f"agentplugins_{VERSION}_{target}"
            path.write_text(f'#!/bin/sh\nif [ "$(basename "$0")" = agentplugins ] && [ -e "{self.block}" ]; '
                            f'then echo "blocked by Gatekeeper" >&2; exit 137; fi\necho "agentplugins {VERSION}"\n')
        self.runner_temp = self.root / "runner-temp"
        self.runner_temp.mkdir()
        self.evidence = self.root / "evidence"
        self.outputs = self.root / "github-output"
        self.outputs.write_text("")
        self.state = self.root / "state.json"
        self.original_search = ["/fake/login.keychain-db", "/fake/System.keychain"]
        self.state.write_text(json.dumps({"calls": [], "search": self.original_search, "deleted": [],
                                          "signed": [], "keys": [], "quarantined": {}}))
        self.secrets = {
            "APPLE_CERTIFICATE": base64.b64encode(P12).decode(),
            "APPLE_CERTIFICATE_PASSWORD": PASSWORD,
            "APPLE_TEAM_ID": TEAM,
            "APPLE_API_KEY_BASE64": base64.b64encode(P8).decode(),
            "APPLE_API_KEY_ID": "FAKEKEYID1",
            "APPLE_API_ISSUER": "00000000-0000-0000-0000-000000000000",
        }

    def run_script(self, mode="", **overrides):
        env = {"PATH": f"{self.bin}{os.pathsep}{os.environ['PATH']}", "HOME": str(self.root),
               "RUNNER_TEMP": str(self.runner_temp), "GITHUB_OUTPUT": str(self.outputs),
               "FAKE_STATE": str(self.state), "FAKE_MODE": mode, **self.secrets, **overrides}
        result = subprocess.run(["bash", str(SCRIPT), TAG, str(self.binaries), str(self.evidence)],
                                env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True,
                                timeout=60)
        printed = result.stdout + result.stderr
        for name in ("APPLE_CERTIFICATE", "APPLE_CERTIFICATE_PASSWORD", "APPLE_API_KEY_BASE64"):
            self.assertNotIn(self.secrets[name], printed, name + " was printed")
        self.assertNotIn(P8.decode(), printed)
        return result, json.loads(self.state.read_text())

    def assert_cleaned(self, state):
        if "keychain" in state:
            self.assertEqual(state["deleted"], [state["keychain"]])
            self.assertEqual(state["search"], self.original_search)
        self.assertEqual(list(self.runner_temp.iterdir()), [], "work directory with key material remains")
        for key in state["keys"]:
            self.assertFalse(Path(key).exists())

    def test_signs_notarizes_and_binds_exact_bytes(self):
        result, state = self.run_script()
        self.assertEqual(result.returncode, 0, result.stderr)
        outputs = dict(line.split("=", 1) for line in self.outputs.read_text().splitlines())
        expected = {f"{target}_sha256": hashlib.sha256(
            (self.binaries / f"agentplugins_{VERSION}_{target}").read_bytes()).hexdigest()
            for target in ("darwin_amd64", "darwin_arm64")}
        self.assertEqual(outputs, expected)
        self.assertEqual(len(state["signed"]), 2)
        for argv in state["signed"]:
            for flag in ("--force", "--timestamp", "--options", "runtime", "--identifier",
                         "com.777genius.agentplugins", "--sign", IDENTITY_HASH, "--keychain", state["keychain"]):
                self.assertIn(flag, argv)
        self.assertIn(state["keychain"], [call[-1] for call in state["calls"] if call[1] == "delete-keychain"])
        summary = json.loads((self.evidence / "signing-summary.json").read_text())
        self.assertEqual(summary["identity"], IDENTITY)
        self.assertEqual(summary["notary_submission_id"], SUBMISSION)
        for name in ("notary-submission.json", "notary-log.json", "codesign-darwin_amd64.txt",
                     "codesign-darwin_arm64.txt"):
            self.assertTrue((self.evidence / name).is_file(), name)
        self.assert_cleaned(state)

    def test_every_unverified_outcome_fails_closed_and_cleans_up(self):
        for mode in ("adhoc", "no-runtime", "no-timestamp", "wrong-team", "wrong-identifier",
                     "two-identities", "invalid", "wrong-cdhash"):
            with self.subTest(mode=mode):
                self.setUp()
                result, state = self.run_script(mode)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(self.outputs.read_text(), "")
                self.assertIn("keychain", state)
                self.assert_cleaned(state)

    def test_wrong_inputs_stop_before_any_keychain_work(self):
        cases = {"wrong team secret": {"APPLE_TEAM_ID": "AAAAAAAAAA"},
                 "missing secret": {"APPLE_API_ISSUER": ""}}
        for name, overrides in cases.items():
            with self.subTest(case=name):
                self.setUp()
                result, state = self.run_script(**overrides)
                self.assertNotEqual(result.returncode, 0)
                self.assertNotIn("keychain", state)
        for name in ("swapped-arch", "extra file"):
            with self.subTest(case=name):
                self.setUp()
                if name == "extra file":
                    (self.binaries / "agentplugins_1.2.3_linux_amd64").write_text("x")
                result, state = self.run_script("swapped-arch" if name == "swapped-arch" else "")
                self.assertNotEqual(result.returncode, 0)
                self.assertNotIn("keychain", state)
                self.assertEqual(state["signed"], [])

    def test_wrong_certificate_password_fails_and_cleans_up(self):
        result, state = self.run_script(APPLE_CERTIFICATE_PASSWORD="not-the-password")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(state["signed"], [])
        self.assert_cleaned(state)


if __name__ == "__main__":
    unittest.main()
