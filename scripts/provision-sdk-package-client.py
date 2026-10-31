#!/usr/bin/env python3
"""Acquire only the exact native package client used by the focused SDK journey."""
import argparse
import hashlib
import importlib.util
import json
from pathlib import Path


def load(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), Path(__file__).with_name(name + '.py'))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--client', choices=('copilot', 'grok', 'qoder'), required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    output = args.output.absolute()
    output.mkdir(parents=True, exist_ok=False)
    if args.client == 'copilot':
        matrix = load('run-native-client-matrix')
        pin = ('github', 'github/copilot-cli', 'v1.0.103', 'copilot-linux-x64.tar.gz',
               'sha256:9aeb4430e9ac9523546db07b227c7aba179a4710a55f9ac29ad143b0f9e1b325', 'copilot')
        binary, evidence = matrix.provision(pin, output, 'copilot')
        expected = 'f19b3554a6b62605db3a26d5255ebe2a3ee39fcd6a357edb18301ea5037f6f0d'
    elif args.client == 'grok':
        research = load('research-grok-native')
        with research.Budget() as budget:
            binary, evidence = research.acquire_binary(output, budget)
        expected = research.BINARY_SHA256
    else:
        research = load('research-qoder-native')
        binary, evidence = research.acquire_binary(output)
        expected = research.BINARY_SHA256
    if hashlib.sha256(binary.read_bytes()).hexdigest() != expected or binary.name != args.client:
        raise ValueError('native executable identity mismatch')
    (output / 'provision.json').write_text(json.dumps({'client': args.client, 'path': str(binary), **evidence}, indent=2) + '\n')


if __name__ == '__main__':
    main()
