#!/usr/bin/env python3
"""One-time, lossless-first migration of tracked raster artwork to WebP.

Requires libwebp's cwebp and dwebp. Leaves PNG-only integration surfaces and
the immutable owner-approved design reference unchanged.
"""
from __future__ import annotations

import hashlib
import json
from pathlib import Path
import shutil
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[1]
PNG_EXCEPTIONS = {
    "docs/design/restore-team/approved-reference.png",  # immutable design evidence
    "landing/public/apple-touch-icon.png",               # iOS pinned touch icon
    "landing/public/favicon-32.png",                      # legacy favicon fallback
    "landing/public/og-image.png",                        # social-crawler compatibility
}
EXPECTED_CONVERSIONS = 18
MONITOR_SOURCE = "landing/assets/images/restore-monitor.png"
PROVENANCE_PATH = ROOT / "docs/design/restore-team/asset-provenance.json"


def run(*args: str) -> subprocess.CompletedProcess[str]:
    return subprocess.run(args, cwd=ROOT, check=True, text=True, capture_output=True)


def is_webp(path: Path) -> bool:
    data = path.read_bytes()
    return len(data) >= 20 and data[:4] == b"RIFF" and data[8:12] == b"WEBP"


def png_dimensions(path: Path) -> tuple[int, int]:
    header = path.read_bytes()[:26]
    if header[:8] != b"\x89PNG\r\n\x1a\n" or header[12:16] != b"IHDR":
        raise ValueError(f"Not a valid PNG: {path}")
    return int.from_bytes(header[16:20], "big"), int.from_bytes(header[20:24], "big")


def convert(source: Path, dest: Path, temp: Path) -> tuple[int, int]:
    source_size = source.stat().st_size
    source_size_pixels = png_dimensions(source)
    lossless = temp / "lossless.webp"
    run("cwebp", "-quiet", "-lossless", "-z", "9", "-m", "6", str(source), "-o", str(lossless))
    chosen = lossless

    # Artistic gradient rendering benefits from high-quality lossy RGB with
    # separately losslessly-coded alpha. Never lossily encode text or logos.
    if source.relative_to(ROOT).as_posix() in {
        MONITOR_SOURCE,
        "landing/assets/images/showcase/agent-notifications.png",
    }:
        lossy = temp / "artwork.webp"
        run("cwebp", "-quiet", "-q", "94", "-m", "6", "-alpha_q", "100",
            str(source), "-o", str(lossy))
        if lossy.stat().st_size < lossless.stat().st_size * 0.85:
            chosen = lossy

    if not is_webp(chosen):
        raise ValueError(f"Encoder did not produce WebP: {source}")
    decoded = temp / "decoded.png"
    run("dwebp", "-quiet", str(chosen), "-o", str(decoded))
    if png_dimensions(decoded) != source_size_pixels:
        raise ValueError(f"Dimensions changed: {source}")
    if dest.exists():
        raise ValueError(f"Refusing to overwrite an existing WebP source: {dest}")
    shutil.copyfile(chosen, dest)
    return source_size, dest.stat().st_size


def replace_in_text(path: Path, before: str, after: str) -> None:
    content = path.read_text(encoding="utf-8")
    if before not in content:
        raise ValueError(f"Expected reference missing: {path}: {before}")
    path.write_text(content.replace(before, after), encoding="utf-8")


def main() -> None:
    tracked = run("git", "ls-files", "-z").stdout
    # NUL-delimited; capture_output(text=True) preserves the delimiters.
    pngs = sorted(path for path in tracked.split("\0") if path.lower().endswith(".png"))
    candidates = [p for p in pngs if p not in PNG_EXCEPTIONS]
    if set(pngs) - set(candidates) != PNG_EXCEPTIONS or len(candidates) != EXPECTED_CONVERSIONS:
        raise ValueError(f"PNG inventory changed; audit before migrating: {pngs}")
    total_before = 0
    total_after = 0
    monitor_digest = None
    for relative in candidates:
        original = ROOT / relative
        dest = original.with_suffix(".webp")
        with tempfile.TemporaryDirectory(prefix="uap-webp-") as directory:
            old, new = convert(original, dest, Path(directory))
        if relative == MONITOR_SOURCE:
            monitor_digest = hashlib.sha256(dest.read_bytes()).hexdigest()
        total_before += old
        total_after += new
        original.unlink()
        print(f"{relative} -> {dest.relative_to(ROOT)}: {old} -> {new} bytes")

    # README paths need actual committed WebP sources, not external binary URLs.
    replace_in_text(ROOT / "README.md", "assets/readme-banner.png", "assets/readme-banner.webp")
    replace_in_text(
        ROOT / "README.md",
        "landing/assets/images/showcase/agent-notifications.png",
        "landing/assets/images/showcase/agent-notifications.webp",
    )

    # The approved reference is immutable, while the runtime monitor is now
    # an optimized derivative: retain byte-exact provenance for both.
    provenance = json.loads(PROVENANCE_PATH.read_text(encoding="utf-8"))
    provenance["asset"]["path"] = "landing/assets/images/restore-monitor.webp"
    provenance["asset"]["sha256"] = monitor_digest
    provenance["asset"]["tool"] = "built-in image_gen; lossless-alpha WebP optimization (libwebp)"
    PROVENANCE_PATH.write_text(json.dumps(provenance, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")

    # The landing already imports and serves .webp. Do not leave source PNGs
    # invisible behind the build-time encoder or break runtime references.
    remaining = sorted(
        p.relative_to(ROOT).as_posix() for p in ROOT.rglob("*.png")
        if ".git" not in p.parts and "node_modules" not in p.parts
    )
    if set(remaining) != PNG_EXCEPTIONS:
        raise ValueError(f"Unexpected PNGs after migration: {remaining}")
    saved = total_before - total_after
    print(f"Converted {len(candidates)} sources: {total_before} -> {total_after} bytes "
          f"({saved:+} bytes saved; {100 * saved / total_before:.1f}% reduction)")


if __name__ == "__main__":
    main()
