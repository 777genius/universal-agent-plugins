import fs from "node:fs/promises";
import { generatedRegistryPaths } from "../config/site.mjs";

const entities = JSON.parse(await fs.readFile(generatedRegistryPaths.entities, "utf8"));
const leaked = entities.filter((entry) => entry.publicVisibility === "internal");

if (leaked.length) {
  console.error("Internal entities leaked into public registry:");
  for (const entry of leaked) {
    console.error(`- ${entry.canonicalId}`);
  }
  process.exit(1);
}
