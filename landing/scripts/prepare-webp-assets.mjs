// Generate WebP build assets from the retained editable PNG originals.
// Chromium is already installed by the landing build for the social preview.
import { chromium } from '@playwright/test';
import { mkdir, readFile, readdir, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const landing = fileURLToPath(new URL('..', import.meta.url));
const imageRoot = path.join(landing, 'assets/images');
const seoLogo = path.join(landing, 'assets/logo-192.png');
const signature = Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]);

async function* sources(dir) {
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const location = path.join(dir, entry.name);
    if (entry.isDirectory()) yield* sources(location);
    else if (entry.isFile() && entry.name.endsWith('.png')) yield location;
  }
}

const browser = await chromium.launch({ headless: true });
let count = 0;
let originalBytes = 0;
let webpBytes = 0;

try {
  const page = await browser.newPage();
  for await (const source of (async function* () {
    yield* sources(imageRoot);
    yield seoLogo;
  })()) {
    const png = await readFile(source);
    if (!png.subarray(0, signature.length).equals(signature)) {
      throw new Error(`Invalid PNG source: ${source}`);
    }
    const relative = path.relative(imageRoot, source).replaceAll(path.sep, '/');
    const quality = /^(agent-badges|plugin-logos|showcase)\//.test(relative) ? 0.98 : 0.88;
    const encoded = await page.evaluate(async ({ url, quality }) => {
      const image = new Image();
      image.src = url;
      await image.decode();
      if (!image.naturalWidth || !image.naturalHeight) throw new Error('Empty image');
      const canvas = document.createElement('canvas');
      canvas.width = image.naturalWidth;
      canvas.height = image.naturalHeight;
      const context = canvas.getContext('2d');
      if (!context) throw new Error('Canvas unavailable');
      context.drawImage(image, 0, 0);
      const data = canvas.toDataURL('image/webp', quality);
      if (!data.startsWith('data:image/webp;base64,')) throw new Error('WebP unsupported');
      return data.slice('data:image/webp;base64,'.length);
    }, { url: `data:image/png;base64,${png.toString('base64')}`, quality });

    const webp = Buffer.from(encoded, 'base64');
    if (
      webp.length < 20 ||
      webp.toString('ascii', 0, 4) !== 'RIFF' ||
      webp.toString('ascii', 8, 12) !== 'WEBP'
    ) {
      throw new Error(`Invalid WebP output: ${relative}`);
    }
    // The public showcase asset is generated from a private source so Pages
    // never ships an unused 500KB PNG alongside the actual WebP.
    const target = source === seoLogo
      ? path.join(landing, 'public/logo-192.webp')
      : relative === 'showcase/agent-notifications.png'
        ? path.join(landing, 'public/showcase/agent-notifications.webp')
        : source.replace(/\.png$/i, '.webp');
    await mkdir(path.dirname(target), { recursive: true });
    await writeFile(target, webp);
    count++;
    originalBytes += png.length;
    webpBytes += webp.length;
  }
  if (!count) throw new Error('No source PNG assets found');
  console.log(`Generated ${count} WebP assets (${originalBytes} → ${webpBytes} bytes)`);
} finally {
  await browser.close();
}
