import fs from "node:fs/promises";
import path from "node:path";
import { repoRoot, outputRoot, docsToolsRoot } from "../config/site.mjs";
import { run } from "./process.mjs";

// Accepted adapter provenance is an input pin, never the generated-output commit.
export const acceptedAuthoringSHA = "801f846cac40f47c344983ccb61122cbdd3ae2f8";
export async function requireAuthoringSource() {
  const sha = process.env.DOCS_AUTHORING_SOURCE_SHA || acceptedAuthoringSHA;
  if (sha !== acceptedAuthoringSHA) throw new Error("DOCS_AUTHORING_SOURCE_SHA differs from the checked-in accepted authoring pin; review a source-contract update first");
  if (!process.env.DOCS_AUTHORING_CHECKOUT) throw new Error("Explicit immutable DOCS_AUTHORING_CHECKOUT required, separate from writable site output");
  const checkout = await fs.realpath(process.env.DOCS_AUTHORING_CHECKOUT);
  const overlaps = (a, b) => a === b || a.startsWith(b + path.sep) || b.startsWith(a + path.sep);
  for (const writable of [repoRoot, outputRoot, docsToolsRoot]) {
    // Resolve existing ancestors too, so symlinked output cannot enter source.
    async function physical(file) {
      try { return await fs.realpath(file); } catch (error) {
        if (error.code !== "ENOENT") throw error;
        return path.join(await physical(path.dirname(file)), path.basename(file));
      }
    }
    if (overlaps(checkout, await physical(writable))) throw new Error("Immutable authoring checkout must be separate from writable site/output/tools directories");
  }
  if ((await run("git", ["rev-parse", "HEAD"], { cwd: checkout })).trim() !== sha) throw new Error("Immutable authoring checkout HEAD differs from accepted pin");
  if ((await run("git", ["status", "--porcelain", "--untracked-files=no"], { cwd: checkout })).trim()) throw new Error("Immutable authoring checkout has tracked changes");
  return { checkout, sha };
}
