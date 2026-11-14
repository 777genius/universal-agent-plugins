"""Console entrypoint for the plugin-kit-ai Python wrapper."""

from __future__ import annotations

import sys

from .install import format_install_error, run_binary


def main() -> None:
    try:
        raise SystemExit(run_binary())
    except Exception as err:  # pragma: no cover - exercised through integration tests
        sys.stderr.write(format_install_error(err) + "\n")
        raise SystemExit(1)


if __name__ == "__main__":
    main()
