#!/usr/bin/env python3
"""Provision only the pinned scanner; never provision or execute clients."""
import argparse
import importlib.util
import json
from pathlib import Path
import urllib.request


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--target', required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    spec = importlib.util.spec_from_file_location('native_matrix', Path(__file__).resolve().parents[1] / 'run-native-client-matrix.py')
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    _, repo, version, archive, digest, basename = module.PINS[args.target]['lintai']
    url = f'https://github.com/{repo}/releases/download/{version}/{archive}'
    with urllib.request.urlopen(url, timeout=120) as response:
        body = response.read(100 << 20)
    module.verify_digest(body, digest)
    args.output.mkdir(parents=True, exist_ok=True)
    (args.output / archive).write_bytes(body)
    (args.output / 'scanner.json').write_text(json.dumps({'url': url, 'digest': digest}))


if __name__ == '__main__':
    main()
