import path from "node:path";
import { docsToolsRoot, runtimeRoot, websiteRoot } from "./config/site.mjs";
import { rimraf } from "./lib/fs.mjs";

await Promise.all([
  rimraf(path.join(websiteRoot, "dist")),
  rimraf(runtimeRoot),
  rimraf(docsToolsRoot)
]);
