// Preserve only deliberately authored, visible cross-language anchors. Automatic
// heading slugs belong to VitePress and are never guessed by the language menu.
export function authoredAnchors(body) {
  const current = body.split("<!-- locale-historical-source:start", 1)[0].replace(/<!--[\s\S]*?-->/g, "");
  const anchors = new Set();
  let fence;
  for (const line of current.split("\n")) {
    const delimiter = line.match(/^\s*(`{3,}|~{3,})/);
    if (delimiter) {
      if (!fence) fence = delimiter[1];
      else if (delimiter[1][0] === fence[0] && delimiter[1].length >= fence.length) fence = undefined;
      continue;
    }
    if (fence) continue;
    const anchor = line.match(/^\s*<a id="([^"\s]+)"><\/a>\s*$/)?.[1] || line.match(/^#{1,6}\s+.+\s\{#([^}\s]+)\}\s*$/)?.[1];
    if (anchor) anchors.add(anchor);
  }
  return [...anchors];
}
