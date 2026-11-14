import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';

// Vue/ESLint still consume the separate classic compiler API. Their adapter
// must never silently replace the native compiler used for project checking.
const metadata = JSON.parse(readFileSync(new URL(import.meta.resolve('@typescript/native/package.json')), 'utf8'));
assert.equal(metadata.name, 'typescript');
assert.equal(metadata.version, '7.0.2');
assert.equal(execFileSync('tsc', ['--version'], { encoding: 'utf8' }).trim(), 'Version 7.0.2');
console.log('Native TypeScript 7.0.2 compiler verified; Vue uses its separate API compatibility layer.');
