import fs from "node:fs/promises";
import { createHash } from "node:crypto";
export const inventory = JSON.parse(await fs.readFile(new URL("./locale-dispositions.json", import.meta.url), "utf8"));
const hash = (text) => createHash("sha256").update(text).digest("hex");
const embeddedArchiveStart = "<!-- locale-historical-source:start";
const embeddedArchiveEnd = "locale-historical-source:end -->";
const embeddedFrontmatterStart = "\nlocale-historical-frontmatter:start\n";
const embeddedFrontmatterEnd = "\nlocale-historical-frontmatter:end";

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
    let frontmatter = body.slice(0, end);
    const detailsStart = body.indexOf("</summary>");
    const embeddedStart = body.indexOf(embeddedArchiveStart);
    const embeddedEnd = body.indexOf(embeddedArchiveEnd);
    let archive = "";
    let preserved = false;
    if (detailsStart >= 0 && body.endsWith("\n</details>\n")) {
      archive = body.slice(detailsStart + "</summary>".length + 1, -"\n</details>\n".length);
      preserved = true;
    } else if (embeddedStart >= 0 && embeddedEnd > embeddedStart &&
        body.indexOf(embeddedArchiveStart, embeddedStart + 1) < 0 &&
        body.indexOf(embeddedArchiveEnd, embeddedEnd + 1) < 0) {
      const embedded = body.slice(embeddedStart + embeddedArchiveStart.length, embeddedEnd);
      if (embedded.startsWith(embeddedFrontmatterStart)) {
        const frontmatterEnd = embedded.indexOf(embeddedFrontmatterEnd, embeddedFrontmatterStart.length);
        if (frontmatterEnd >= 0) {
          frontmatter = embedded.slice(embeddedFrontmatterStart.length, frontmatterEnd);
          archive = embedded.slice(frontmatterEnd + embeddedFrontmatterEnd.length);
        }
      } else {
        archive = embedded;
      }
      preserved = true;
    }
    if (hash(frontmatter) !== entry.frontmatterSha256) errors.push("preserved frontmatter changed");
    if (!preserved || hash(archive) !== entry.bodySha256) errors.push("preserved historical body changed/missing");
    if (hash(frontmatter + archive) !== entry.originalSha256) errors.push("original file identity mismatch");
  } else if (disposition === "historical-snapshot") errors.push("historical snapshot needs reviewed preservation inventory");
  return errors;
}
