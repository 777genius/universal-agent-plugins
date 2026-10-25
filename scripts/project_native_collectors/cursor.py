"""Immutable Cursor distribution candidate; acquisition does not run its payload.

The shared hosted runner may download and inventory this archive. Its launcher,
bundled Node runtime, support files and digests are deliberately not inferred
from the installer script. A reviewed pin amendment must freeze those measured
identities before a native observation collector can replace this refusal.
"""

CLIENT = "cursor"
VERSION = "2026.10.22-27dcf5f"
ACQUISITION_ONLY = True
ACQUISITION = {
    "url": "https://downloads.cursor.com/lab/2026.10.22-27dcf5f/linux/x64/agent-cli-package.tar.gz",
    "integrity": None,
    "max_bytes": 256 * 1024 * 1024,
    "format": "tar.gz",
    "entrypoint": None,
    "runtime": "node",
}


def collect(ctx):
    """Refuse native work, including context access, before measured pin review."""
    raise RuntimeError(
        "Cursor is acquisition-only: archive, launcher, Node and support-file "
        "identities require a reviewed pin amendment before native observation"
    )
