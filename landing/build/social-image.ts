// The 1200x630 social preview, generated from the public client list so it always
// shows every supported agent. Assets are referenced relative to public/ (the site
// logo, client icons and the site's own fonts), so public/og-image.svg also renders
// on its own. scripts/render-social-image.mjs writes the SVG and rasterizes the PNG.
import type { ClientTarget } from '../types/registry';

export const SOCIAL_IMAGE_WIDTH = 1200;
export const SOCIAL_IMAGE_HEIGHT = 630;

export type SocialImageClient = Pick<ClientTarget, 'name' | 'icon'>;

/** Agents card geometry; the render script checks that every label stays inside it. */
export const SOCIAL_CARD = { x: 652, y: 128, width: 504, height: 446 } as const;
const CARD_PADDING = 22;
const GRID_TOP = 66;
const GRID_GAP = 8;
const MIN_TILE = { width: 80, height: 70 } as const;

export interface SocialGrid {
  columns: number;
  rows: number;
  tileWidth: number;
  tileHeight: number;
}

/** Fewest empty cells first, then the tile closest to the 4:3 card shape. */
export function socialGrid(count: number): SocialGrid {
  const width = SOCIAL_CARD.width - CARD_PADDING * 2;
  const height = SOCIAL_CARD.height - GRID_TOP - CARD_PADDING;
  let best: (SocialGrid & { empty: number; shape: number }) | undefined;
  for (let columns = 2; columns <= 8; columns += 1) {
    const rows = Math.ceil(count / columns);
    const tileWidth = (width - GRID_GAP * (columns - 1)) / columns;
    const tileHeight = (height - GRID_GAP * (rows - 1)) / rows;
    if (tileWidth < MIN_TILE.width || tileHeight < MIN_TILE.height) continue;
    const candidate = {
      columns,
      rows,
      tileWidth,
      tileHeight,
      empty: columns * rows - count,
      shape: Math.abs(tileWidth / tileHeight - 4 / 3),
    };
    if (
      !best ||
      candidate.empty < best.empty ||
      (candidate.empty === best.empty && candidate.shape < best.shape)
    )
      best = candidate;
  }
  if (!best) throw new Error(`The social preview cannot fit ${count} agents; adjust its layout`);
  const { columns, rows, tileWidth, tileHeight } = best;
  return { columns, rows, tileWidth, tileHeight };
}

/** Wrap a label onto at most two balanced lines at a space. */
export function labelLines(name: string, maxChars: number): string[] {
  if (name.length <= maxChars || !name.includes(' ')) return [name];
  let best: string[] = [name];
  let longest = Infinity;
  for (let index = name.indexOf(' '); index !== -1; index = name.indexOf(' ', index + 1)) {
    const lines = [name.slice(0, index), name.slice(index + 1)];
    const length = Math.max(...lines.map((line) => line.length));
    if (length < longest) [best, longest] = [lines, length];
  }
  return best;
}

const escape = (value: string) =>
  value.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
const px = (value: number) => Number(value.toFixed(2));

function pills(labels: string[], y: number): string {
  let x = 71;
  return labels
    .map((label) => {
      const width = Math.round(label.length * 9.4 + 38);
      const pill = `<g><rect x="${x}" y="${y}" width="${width}" height="31" rx="15.5" fill="#143449"/><text x="${x + width / 2}" y="${y + 20.5}" text-anchor="middle" fill="#B1F4FE" font-size="14" font-weight="700" letter-spacing=".6">${escape(label)}</text></g>`;
      x += width + 9;
      return pill;
    })
    .join('\n');
}

function agentTiles(clients: readonly SocialImageClient[]): string {
  const grid = socialGrid(clients.length);
  const left = SOCIAL_CARD.x + CARD_PADDING;
  const top = SOCIAL_CARD.y + GRID_TOP;
  const iconBox = Math.min(42, grid.tileHeight * 0.48);
  const fontSize = grid.tileWidth >= 96 ? 13 : 12;
  const maxChars = Math.floor((grid.tileWidth - 10) / (fontSize * 0.6));
  return clients
    .map((client, index) => {
      const row = Math.floor(index / grid.columns);
      const inRow = Math.min(grid.columns, clients.length - row * grid.columns);
      // Centre a shorter last row instead of leaving it ragged on one side.
      const offset = ((grid.columns - inRow) * (grid.tileWidth + GRID_GAP)) / 2;
      const x = left + offset + (index % grid.columns) * (grid.tileWidth + GRID_GAP);
      const y = top + row * (grid.tileHeight + GRID_GAP);
      const centre = x + grid.tileWidth / 2;
      const iconY = y + 9;
      const lines = labelLines(client.name, maxChars);
      const lineHeight = fontSize + 2;
      const textTop =
        iconY + iconBox + (y + grid.tileHeight - iconY - iconBox - lines.length * lineHeight) / 2;
      const label = lines
        .map(
          (line, lineIndex) =>
            `<tspan x="${px(centre)}" y="${px(textTop + fontSize - 1 + lineIndex * lineHeight)}">${escape(line)}</tspan>`,
        )
        .join('');
      const inset = iconBox * 0.15;
      return [
        `<g class="agent">`,
        `<rect x="${px(x)}" y="${px(y)}" width="${px(grid.tileWidth)}" height="${px(grid.tileHeight)}" rx="14" fill="#112237" stroke="#263F57"/>`,
        `<rect x="${px(centre - iconBox / 2)}" y="${px(iconY)}" width="${px(iconBox)}" height="${px(iconBox)}" rx="11" fill="#F7FAFD"/>`,
        `<image href="client-icons/${escape(client.icon)}" x="${px(centre - iconBox / 2 + inset)}" y="${px(iconY + inset)}" width="${px(iconBox - inset * 2)}" height="${px(iconBox - inset * 2)}"/>`,
        `<text class="agent-name" text-anchor="middle" fill="#EAF4FF" font-size="${fontSize}" font-weight="700">${label}</text>`,
        `</g>`,
      ].join('');
    })
    .join('\n');
}

export function socialImageSvg({
  clients,
  command,
}: {
  clients: readonly SocialImageClient[];
  command: string;
}): string {
  const names = clients.map((client) => client.name).join(', ');
  const card = SOCIAL_CARD;
  return `<svg xmlns="http://www.w3.org/2000/svg" width="${SOCIAL_IMAGE_WIDTH}" height="${SOCIAL_IMAGE_HEIGHT}" viewBox="0 0 ${SOCIAL_IMAGE_WIDTH} ${SOCIAL_IMAGE_HEIGHT}" role="img" aria-labelledby="og-title og-desc">
<!-- Generated by landing/scripts/render-social-image.mjs from landing/data/clients.ts. Do not edit. -->
<title id="og-title">Universal Agent Plugins: one plugin, all your agents</title>
<desc id="og-desc">Install and manage Agent Plugins 1.0, Agent Skills, and MCP tools with one CLI. Supported agents: ${escape(names)}.</desc>
<style>
@font-face{font-family:Inter;font-weight:100 900;src:url(fonts/inter-v20-latin.woff2) format('woff2')}
@font-face{font-family:'JetBrains Mono';font-weight:100 800;src:url(fonts/jetbrains-mono-v24-latin.woff2) format('woff2')}
text{font-family:Inter,Arial,Helvetica,sans-serif}
.mono{font-family:'JetBrains Mono',ui-monospace,monospace}
</style>
<defs>
<linearGradient id="bg" x1="0" y1="0" x2="1200" y2="630" gradientUnits="userSpaceOnUse"><stop stop-color="#080E1D"/><stop offset=".65" stop-color="#0B192A"/><stop offset="1" stop-color="#071C28"/></linearGradient>
<radialGradient id="halo" cx="0" cy="0" r="1" gradientUnits="userSpaceOnUse" gradientTransform="translate(1030 140) rotate(144) scale(680 430)"><stop stop-color="#22D3EE" stop-opacity=".23"/><stop offset="1" stop-color="#22D3EE" stop-opacity="0"/></radialGradient>
<linearGradient id="accent" x1="60" y1="280" x2="600" y2="305" gradientUnits="userSpaceOnUse"><stop stop-color="#8DEBFF"/><stop offset=".5" stop-color="#65DDF7"/><stop offset="1" stop-color="#82F8C9"/></linearGradient>
<linearGradient id="border" x1="650" y1="130" x2="1150" y2="570" gradientUnits="userSpaceOnUse"><stop stop-color="#4A8099" stop-opacity=".8"/><stop offset="1" stop-color="#164B5A" stop-opacity=".6"/></linearGradient>
<pattern id="grid" width="42" height="42" patternUnits="userSpaceOnUse"><path d="M42 0H0V42" stroke="#6389A3" stroke-opacity=".075" stroke-width="1" fill="none"/></pattern>
</defs>
<rect width="1200" height="630" fill="url(#bg)"/><rect width="1200" height="630" fill="url(#halo)"/><rect width="1200" height="630" fill="url(#grid)"/>
<circle cx="1136" cy="93" r="230" fill="none" stroke="#3D9CAF" stroke-opacity=".17" stroke-width="1.5"/>
<circle cx="1136" cy="93" r="295" fill="none" stroke="#3D9CAF" stroke-opacity=".1" stroke-width="1.5"/>
<rect x="31" y="31" width="1138" height="568" rx="27" fill="none" stroke="#A7D7E9" stroke-opacity=".13"/>
<image href="icon.svg" x="64" y="50" width="54" height="54"/>
<text x="130" y="86" fill="#F2F8FF" font-size="27" font-weight="700">Universal Agent Plugins</text>
<rect x="907" y="59" width="228" height="36" rx="18" fill="#133748" stroke="#2C7D8E"/><circle cx="927" cy="77" r="4" fill="#84EBCD"/><text x="943" y="83" fill="#C4F4FF" font-size="15" font-weight="700" letter-spacing=".8">OPEN SOURCE · UAP</text>
<text class="headline" x="71" y="168" fill="#8CB1CB" font-size="17" font-weight="700" letter-spacing="2.8">ONE CLI · MANY AGENTS</text>
<text class="headline" x="67" y="246" fill="#F7FBFF" font-size="64" font-weight="800" letter-spacing="-2">One plugin</text>
<text class="headline" x="67" y="320" fill="url(#accent)" font-size="64" font-weight="800" letter-spacing="-2">All your agents</text>
<text class="headline" x="70" y="376" fill="#D1E1F0" font-size="24" font-weight="600">Install · Update · Repair · Remove</text>
<text class="headline" x="70" y="411" fill="#90AFC6" font-size="21">Plugins, Skills &amp; MCP tools — together</text>
<rect x="68" y="447" width="553" height="106" rx="19" fill="#080F20" stroke="#31536A"/>
<circle cx="95" cy="470" r="4.3" fill="#FB7185"/><circle cx="111" cy="470" r="4.3" fill="#FCD34D"/><circle cx="127" cy="470" r="4.3" fill="#6EE7B7"/>
<text class="mono headline" x="92" y="511" fill="#B5DEF2" font-size="16"><tspan fill="#6EE7B7">$</tspan> ${escape(command)} add context7</text>
<text class="headline" x="92" y="537" fill="#7FADC0" font-size="15">One command, reversible state</text>
${pills(['AGENT PLUGINS 1.0', 'AGENT SKILLS', 'USER + PROJECT'], 563)}
<rect x="${card.x}" y="${card.y}" width="${card.width}" height="${card.height}" rx="28" fill="#0B182A" fill-opacity=".94" stroke="url(#border)" stroke-width="1.5"/>
<text x="${card.x + CARD_PADDING + 2}" y="${card.y + 42}" fill="#A1BBD2" font-size="17" font-weight="700" letter-spacing="1.9">SUPPORTED AGENTS</text>
<circle cx="${card.x + card.width - CARD_PADDING - 6}" cy="${card.y + 36}" r="6" fill="#68E0C5"/>
${agentTiles(clients)}
</svg>
`;
}
