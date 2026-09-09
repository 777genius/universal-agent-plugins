import { spawnSync } from 'node:child_process';
import fs from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const landingRoot = path.resolve(scriptDir, '..');
const repoRoot = path.resolve(landingRoot, '..');
const landingDist = path.join(landingRoot, '.output', 'public');
const docsDist = path.join(repoRoot, 'website', 'dist');
const pagesDist = path.join(repoRoot, '.pages-dist');
const docsTarget = path.join(pagesDist, 'docs');

async function ensureBuildExists(dir, expectedFile) {
  const full = path.join(dir, expectedFile);
  try {
    await fs.access(full);
  } catch {
    throw new Error(`Expected build artifact is missing: ${full}`);
  }
}

await ensureBuildExists(landingDist, 'index.html');
await ensureBuildExists(docsDist, 'index.html');

await fs.rm(pagesDist, { recursive: true, force: true });
await fs.mkdir(pagesDist, { recursive: true });

await fs.cp(landingDist, pagesDist, { recursive: true });
await fs.mkdir(docsTarget, { recursive: true });
await fs.cp(docsDist, docsTarget, { recursive: true });
await fs.writeFile(path.join(pagesDist, '.nojekyll'), '');

const docsIndex = path.join(docsTarget, 'index.html');
await fs.access(docsIndex);

const check = spawnSync(
  process.execPath,
  ['--experimental-strip-types', path.join(scriptDir, 'check-pages-artifact.mjs')],
  { stdio: 'inherit', env: process.env },
);
if (check.error) throw check.error;
if (check.status !== 0) throw new Error(`Final Pages artifact check failed (${check.status})`);

console.log(`Combined Pages artifact created at ${pagesDist}`);
