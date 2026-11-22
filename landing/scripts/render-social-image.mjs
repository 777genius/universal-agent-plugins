// Generate public/og-image.svg from the public client list and rasterize it for
// social crawlers. This is a static build asset, never a per-request renderer.
import { chromium } from '@playwright/test';
import { readFile, writeFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import {
  SOCIAL_CARD,
  SOCIAL_IMAGE_HEIGHT,
  SOCIAL_IMAGE_WIDTH,
  socialImageSvg,
} from '../build/social-image.ts';
import { clients } from '../data/clients.ts';
import { CLI_INVOCATION } from '../utils/commands.ts';

const source = new URL('../public/og-image.svg', import.meta.url);
const output = fileURLToPath(new URL('../public/og-image.png', import.meta.url));
await writeFile(source, socialImageSvg({ clients, command: CLI_INVOCATION }));
const browser = await chromium.launch({ headless: true });

try {
  const page = await browser.newPage({
    viewport: { width: SOCIAL_IMAGE_WIDTH, height: SOCIAL_IMAGE_HEIGHT },
    deviceScaleFactor: 1,
  });
  // Load the file itself so the logo, client icons and fonts resolve from public/.
  await page.goto(source.href, { waitUntil: 'load' });
  const problems = await page.evaluate(
    async ({ card, count }) => {
      await document.fonts.ready;
      const issues = [];
      if (!document.fonts.check('800 64px Inter') || !document.fonts.check('16px "JetBrains Mono"'))
        issues.push('the site fonts did not load');
      const images = [...document.querySelectorAll('image')];
      if (images.length !== count + 1) issues.push(`expected the logo and ${count} client icons`);
      await Promise.all(
        images.map(
          (image) =>
            new Promise((resolve) => {
              const probe = new Image();
              const href = new URL(image.getAttribute('href') || '', location.href).href;
              probe.onload = resolve;
              probe.onerror = () => resolve(issues.push(`missing image ${href}`));
              probe.src = href;
            }),
        ),
      );
      // Nothing may be clipped: the copy stays left of the card, labels inside their tiles.
      for (const text of document.querySelectorAll('.headline')) {
        const box = text.getBBox();
        if (box.x + box.width > card.x - 16)
          issues.push(`"${text.textContent}" runs into the agents card`);
      }
      for (const tile of document.querySelectorAll('.agent')) {
        const frame = tile.querySelector('rect').getBBox();
        const label = tile.querySelector('.agent-name');
        const box = label.getBBox();
        if (
          box.x < frame.x + 3 ||
          box.x + box.width > frame.x + frame.width - 3 ||
          box.y + box.height > frame.y + frame.height - 2
        )
          issues.push(`"${label.textContent}" does not fit its tile`);
      }
      return issues;
    },
    { card: SOCIAL_CARD, count: clients.length },
  );
  if (problems.length) throw new Error(`Social preview layout: ${problems.join('; ')}`);
  await page.screenshot({ path: output, animations: 'disabled' });

  const png = await readFile(output);
  const signature = Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]);
  if (
    !png.subarray(0, 8).equals(signature) ||
    png.readUInt32BE(16) !== SOCIAL_IMAGE_WIDTH ||
    png.readUInt32BE(20) !== SOCIAL_IMAGE_HEIGHT
  ) {
    throw new Error('Social preview PNG must be 1200×630');
  }
  console.log(`Rendered the 1200×630 social preview with ${clients.length} agents`);
} finally {
  await browser.close();
}
