// Validate committed WebP sources and stage the two public URL assets.
// Build/development must never depend on converting unoptimized PNG originals.
import { copyFile, mkdir, readFile, readdir } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const landing = fileURLToPath(new URL('..', import.meta.url));
const imageRoot = path.join(landing, 'assets/images');
const publicCopies = [
  [path.join(landing, 'assets/logo-192.webp'), path.join(landing, 'public/logo-192.webp')],
  [path.join(imageRoot, 'showcase/agent-notifications.webp'), path.join(landing, 'public/showcase/agent-notifications.webp')],
];

async function* sources(dir) {
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const location = path.join(dir, entry.name);
    if (entry.isDirectory()) yield* sources(location);
    else if (entry.isFile()) {
      if (/\.png$/i.test(entry.name)) {
        throw new Error('Unoptimized raster asset: ' + location + '; commit a WebP instead');
      }
      if (/\.webp$/i.test(entry.name)) yield location;
    }
  }
}

async function requireWebp(source) {
  const bytes = await readFile(source);
  if (bytes.length < 20 || bytes.toString('ascii', 0, 4) !== 'RIFF'
      || bytes.toString('ascii', 8, 12) !== 'WEBP') {
    throw new Error('Invalid committed WebP asset: ' + source);
  }
  return bytes.length;
}

let count = 0;
let sourceBytes = 0;
for await (const source of sources(imageRoot)) {
  sourceBytes += await requireWebp(source);
  count += 1;
}
if (!count) throw new Error('No committed WebP images found');

for (const [source, target] of publicCopies) {
  await requireWebp(source);
  await mkdir(path.dirname(target), { recursive: true });
  await copyFile(source, target);
}
console.log('Validated ' + count + ' committed WebP assets (' + sourceBytes
  + ' bytes); staged ' + publicCopies.length + ' public copies');
