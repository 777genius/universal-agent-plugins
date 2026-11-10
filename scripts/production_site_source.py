"""Select the accepted production site source; retain control provenance outside it."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import stat
import subprocess


def git(root, *args):
    return subprocess.check_output(["git", "-C", str(root), *args], text=True).strip()


def validate(pin):
    if (type(pin) is not dict or set(pin) != {"schema_version", "source_commit", "source_tree"}
            or type(pin["schema_version"]) is not int or pin["schema_version"] != 1
            or any(type(pin[key]) is not str or not re.fullmatch("[0-9a-f]{40}", pin[key]) or pin[key] == "0" * 40
                   for key in ("source_commit", "source_tree"))):
        raise ValueError("invalid immutable production source pin")
    return pin


def verify(root, pin, generated=False):
    if (git(root, "rev-parse", "HEAD") != pin["source_commit"]
            or git(root, "rev-parse", "HEAD^{tree}") != pin["source_tree"]):
        raise ValueError("production source differs from accepted clean commit/tree")
    changes = subprocess.check_output(["git", "-C", str(root), "status", "--porcelain", "-z", "--untracked-files=no"])
    if changes and (not generated or changes not in (b" M landing/public/og-image.png\0", b"M  landing/public/og-image.png\0", b"MM landing/public/og-image.png\0")):
        raise ValueError("production source has unexpected tracked edits")
    if generated:
        image = root / "landing/public/og-image.png"
        if (not stat.S_ISREG(image.lstat().st_mode)
                or git(root, "diff", "--summary", "HEAD", "--", "landing/public/og-image.png")):
            raise ValueError("generated social image must remain a regular file")


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError("duplicate production source pin member")
        result[key] = value
    return result


def select(root, control, expected_control=None):
    raw = (root / "website/production-source.json").read_bytes()
    pin = validate(json.loads(raw, object_pairs_hook=unique_object))
    commit = git(root, "rev-parse", "HEAD")
    if (expected_control if expected_control is not None else os.environ.get("GITHUB_SHA", commit)) != commit:
        raise ValueError("workflow/control source commit mismatch")
    if git(root, "status", "--porcelain", "--untracked-files=no"):
        raise ValueError("control source is modified")
    if git(root, "rev-parse", pin["source_commit"] + "^{tree}") != pin["source_tree"]:
        raise ValueError("production source tree does not match pin")
    subprocess.run(["git", "-C", str(root), "merge-base", "--is-ancestor", pin["source_commit"], commit], check=True)
    control.mkdir(mode=0o700)
    shutil.copyfile(__file__, control / "production_site_source.py")
    shutil.copyfile(root / "landing/scripts/check-published-capabilities.mjs", control / "check-published-capabilities.mjs")
    receipt = {**pin, "control_commit": commit, "manifest_sha256": hashlib.sha256(raw).hexdigest()}
    (control / "receipt.json").write_text(json.dumps(receipt) + "\n")
    subprocess.run(["git", "-C", str(root), "checkout", "--quiet", "--detach", pin["source_commit"]], check=True)
    verify(root, pin)


def seal(root, control):
    receipt = json.loads((control / "receipt.json").read_text())
    pin = validate({key: receipt[key] for key in ("schema_version", "source_commit", "source_tree")})
    verify(root, pin, generated=True)
    image = "landing/public/og-image.png"
    built_image = (root / ".pages-dist/og-image.png").read_bytes()
    if built_image != (root / image).read_bytes():
        raise ValueError("built social image differs from generated output")
    receipt.update(social_image_source_blob=git(root, "rev-parse", pin["source_commit"] + ":" + image),
                   social_image_sha256=hashlib.sha256(built_image).hexdigest())
    if receipt["control_commit"] != os.environ["GITHUB_SHA"]:
        raise ValueError("workflow/control receipt differs")
    marker = root / ".pages-dist/MIRROR_METADATA.json"
    receipt.update(workflow_commit=os.environ["GITHUB_SHA"], run_id=os.environ["GITHUB_RUN_ID"],
                   run_attempt=os.environ["GITHUB_RUN_ATTEMPT"],
                   mirror_marker_sha256=hashlib.sha256(marker.read_bytes()).hexdigest())
    (root / ".pages-dist/BUILD_PROVENANCE.json").write_text(json.dumps(receipt, indent=2) + "\n")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("operation", choices=("select", "seal"))
    parser.add_argument("root", type=Path)
    parser.add_argument("control", type=Path)
    parser.add_argument("--expected-control")
    args = parser.parse_args()
    root, control = args.root.resolve(), args.control.resolve()
    if root == control or root in control.parents or control in root.parents:
        raise ValueError("control receipt must be outside production checkout")
    if args.operation == "select":
        select(root, control, args.expected_control)
    else:
        if args.expected_control is not None:
            raise ValueError("control override is only valid for source selection")
        seal(root, control)
