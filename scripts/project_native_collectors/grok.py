"""Versioned Grok candidate for bounded acquisition only; never executes it.

The official stable pointer now names 1.0.70. Its measured compressed and
executable identities still require an acquisition report and reviewed pin
amendment. The observed 1.0.67 hashes do not qualify these candidate bytes.
"""

CLIENT = "grok"
VERSION = "1.0.70"
AVAILABLE_PHASES = ("acquire",)
ACQUISITION_ONLY = True
ACQUISITION = {
    "url": "https://x.ai/cli/grok-1.0.70-linux-x86_64.gz",
    "integrity": None,
    "max_bytes": 96 << 20,
    "max_expanded_bytes": 256 << 20,
    "format": "gzip",
    # A gzip stream has no archive member: this is the shared runner's fixed
    # synthetic output name, never an authority derived from its gzip header.
    "entrypoint": "grok",
    "runtime": "native",
}


def collect(ctx):
    """Refuse all native observation before pin review, without using context."""
    raise RuntimeError(
        "Grok 1.0.70 is acquisition-only: measured compressed and executable "
        "identities require a reviewed pin amendment before native observation"
    )
