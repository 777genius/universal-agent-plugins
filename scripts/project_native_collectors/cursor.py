"""Measured immutable Cursor distribution; acquisition never runs its payload."""

CLIENT = "cursor"
VERSION = "2026.10.22-27dcf5f"
ACQUISITION_ONLY = True
ACQUISITION = {
    "url": "https://downloads.cursor.com/lab/2026.10.22-27dcf5f/linux/x64/agent-cli-package.tar.gz",
    "integrity": "sha256:85296944e4600a83c4b764f578d1502bf9c416db200d33a593e8f3ecabfdc294",
    "expected_bytes": 184859176,
    "max_bytes": 256 * 1024 * 1024,
    "max_expanded_bytes": 640 * 1024 * 1024,
    "format": "tar.gz",
    "entrypoint": "dist-package/cursor-agent",
    "binary_sha256": "2ccc9a8e167797641448b5e5c936f006ba137a2555f117f38c5eb76a5238a233",
    # This exact shell launcher selects its sibling bundled Node and index.js.
    "runtime": "native",
    "transport": "cursor-publisher-curl",
}


def collect(ctx):
    """Refuse native work, including context access, before collector review."""
    raise RuntimeError(
        "Cursor is acquisition-only: the measured archive pin does not authorize "
        "native observation without a separately reviewed collector"
    )
