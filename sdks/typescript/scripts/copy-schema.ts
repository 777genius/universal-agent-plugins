import { copyFile, mkdir } from 'node:fs/promises';
const output = new URL('../dist/generated/', import.meta.url);
await mkdir(output, { recursive: true });
await copyFile(new URL('../src/generated/schema.d.ts', import.meta.url), new URL('schema.d.ts', output));
