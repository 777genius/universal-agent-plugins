// Node >=22.18. The OpenAPI document is the only editable wire schema.
import { execFileSync } from 'node:child_process';
import { readFileSync, writeFileSync, mkdtempSync, mkdirSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const contractDir = dirname(fileURLToPath(import.meta.url));
const root = resolve(contractDir, '../..');
const scratch = mkdtempSync(join(tmpdir(), 'uap-management-codegen-'));
const check = process.argv.includes('--check');
const outputs = [
  ['apitypes.gen.go', 'cli/internal/agentpluginscli/management/apitypes.gen.go'],
  ['schema.d.ts', 'sdks/typescript/src/generated/schema.d.ts'],
] as const;
try {
  const config = readFileSync(join(contractDir, 'oapi-codegen.yaml'), 'utf8')
    .replace(/^output:.*$/m, `output: ${JSON.stringify(join(scratch, outputs[0][0]))}`);
  const configPath = join(scratch, 'oapi-codegen.yaml');
  writeFileSync(configPath, config);
  execFileSync('go', ['run', 'github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0',
    '-config', configPath, join(contractDir, 'openapi.yaml')],
    { cwd: root, stdio: 'inherit', env: { ...process.env, GOWORK: 'off' } });
  execFileSync(process.platform === 'win32' ? 'npx.cmd' : 'npx',
    ['--yes', 'openapi-typescript@7.13.0', join(contractDir, 'openapi.yaml'), '-o', join(scratch, outputs[1][0])],
    { cwd: root, stdio: 'inherit' });
  let drift = false;
  for (const [temporary, relative] of outputs) {
    const expected = readFileSync(join(scratch, temporary));
    const destination = join(root, relative);
    if (check) {
      if (!readFileSync(destination).equals(expected)) {
        console.error(`Generated contract drift: ${relative}`);
        drift = true;
      }
    } else {
      mkdirSync(dirname(destination), { recursive: true });
      writeFileSync(destination, expected);
    }
  }
  if (drift) process.exitCode = 1;
} finally {
  rmSync(scratch, { recursive: true, force: true });
}
