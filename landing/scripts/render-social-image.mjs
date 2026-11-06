// Rasterize the editable SVG and its bundled client marks for social crawlers.
// This is a static build asset, never a service or per-request renderer.
import { chromium } from '@playwright/test';
import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';

const source = fileURLToPath(new URL('../public/og-image.svg', import.meta.url));
const output = fileURLToPath(new URL('../public/og-image.png', import.meta.url));
const browser = await chromium.launch({ headless: true });

try {
  const page = await browser.newPage({
    viewport: { width: 1200, height: 630 },
    deviceScaleFactor: 1,
  });
  const svg = await readFile(source, 'utf8');
  await page.setContent(`<html><body style="margin:0;overflow:hidden">${svg}</body></html>`, {
    waitUntil: 'load',
  });
  await page.evaluate(async () => {
    await document.fonts.ready;
    const marks = [...document.querySelectorAll('image')];
    if (marks.length !== 8) throw new Error('Expected eight local client marks');
    await Promise.all(marks.map((mark) => new Promise((resolve, reject) => {
      const image = new Image();
      image.onload = resolve;
      image.onerror = () => reject(new Error('Client mark did not render'));
      image.src = mark.getAttribute('href') || '';
    })));
  });
  await page.screenshot({ path: output, animations: 'disabled' });

  const png = await readFile(output);
  const signature = Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]);
  if (
    !png.subarray(0, 8).equals(signature) ||
    png.readUInt32BE(16) !== 1200 ||
    png.readUInt32BE(20) !== 630
  ) {
    throw new Error('Social preview PNG must be 1200×630');
  }
  console.log('Rendered 1200×630 social preview from og-image.svg');
} finally {
  await browser.close();
}
