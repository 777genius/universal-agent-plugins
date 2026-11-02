import { createHash } from 'node:crypto';
import { cp, mkdir, readFile, readdir, rm, writeFile } from 'node:fs/promises';
import { dirname, join, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const dist = join(root, 'dist');
const destination = resolve(root, '../../cli/internal/agentpluginscli/skillspicker/webdist');
await readFile(join(dist, 'index.html'));
const files: Record<string, string> = {};
async function inventory(directory: string) {
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) await inventory(path);
    else if (entry.isFile()) files[relative(dist, path).replaceAll('\\', '/')] = createHash('sha256').update(await readFile(path)).digest('hex');
    else throw new Error('Unexpected non-file in frontend build');
  }
}
await inventory(dist);
await mkdir(destination, { recursive: true });
// These three paths are this build's outputs. Preserve the source checkout's placeholder.
for (const entry of ['assets', 'index.html', 'assets-manifest.json']) await rm(join(destination, entry), { recursive: true, force: true });
await cp(dist, destination, { recursive: true });
await writeFile(join(destination, 'assets-manifest.json'), JSON.stringify({ production: true, files }, null, 2) + '\n');
console.log(`Bundled ${Object.keys(files).length} local assets into CLI webdist.`);
