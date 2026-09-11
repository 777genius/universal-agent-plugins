import fs from "node:fs/promises";
import { createHash } from "node:crypto";
export const inventory = JSON.parse(await fs.readFile(new URL("./locale-dispositions.json", import.meta.url), "utf8"));
const hash = (text) => createHash("sha256").update(text).digest("hex");

// Archive membership is immutable. A later translation can precede the preserved
// disclosure; new current pages opt in through localeDisposition frontmatter.
export function dispositionErrors(relative, body, meta) {
  const entry = inventory.pages[relative];
  const disposition = meta.localeDisposition || entry?.currentDisposition || entry?.disposition;
  const errors = [];
  if (!["english-fallback", "historical-snapshot", "current-translation"].includes(disposition)) errors.push("missing/invalid locale disposition");
  if (disposition === "english-fallback") {
    const target = `/en/${relative.split('/').slice(1).join('/').replace(/index\.md$/, '').replace(/\.md$/, '')}`;
    if (!body.includes(`](${target})`)) errors.push("missing explicit English pointer");
    if (/```|~~~|`(?:plugin-kit-ai|agentplugins)\s/.test(body)) errors.push("executable fallback copy");
  }
  if (entry?.disposition === "historical-snapshot") {
    const end = body.indexOf("\n---", 4) + 4;
    const start = body.indexOf("</summary>");
    const archive = start < 0 ? "" : body.slice(start + "</summary>".length + 1, -"\n</details>\n".length);
    if (hash(body.slice(0, end)) !== entry.frontmatterSha256) errors.push("preserved frontmatter changed");
    if (!body.endsWith("\n</details>\n") || hash(archive) !== entry.bodySha256) errors.push("preserved historical body changed/missing");
    if (hash(body.slice(0, end) + archive) !== entry.originalSha256) errors.push("original file identity mismatch");
  } else if (disposition === "historical-snapshot") errors.push("historical snapshot needs reviewed preservation inventory");
  return errors;
}
