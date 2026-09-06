import fs from "node:fs/promises";
import path from "node:path";
import { websiteRoot, docsToolsRoot } from "../config/site.mjs";
import { run } from "./process.mjs";

// Report every missing offline prerequisite before extractors write any output.
export async function generationPrerequisites() {
  const errors = [];
  const manifest = JSON.parse(await fs.readFile(path.join(websiteRoot, "package.json"), "utf8"));
  if (process.versions.node !== manifest.engines.node) errors.push(`Node ${manifest.engines.node} required; found ${process.versions.node}`);
  for (const [name, version] of Object.entries(manifest.devDependencies)) {
    try {
      const installed = JSON.parse(await fs.readFile(path.join(websiteRoot, "node_modules", name, "package.json"), "utf8"));
      if (installed.version !== version) errors.push(`${name}@${version} required; found ${installed.version}`);
    } catch { errors.push(`Missing installed ${name}@${version}`); }
  }
  const python = path.join(docsToolsRoot, "python-venv", "bin", "python");
  try {
    await run(python, ["-c", "import importlib.metadata; assert importlib.metadata.version('pydoc-markdown') == '4.8.2'"]);
  } catch { errors.push(`Missing ${python} with pydoc-markdown==4.8.2`); }
  for (const [name, value] of Object.entries({ GOPROXY: "off", GOSUMDB: "off", GOENV: "off", GOTOOLCHAIN: "local", GOMAXPROCS: "2" })) {
    if (process.env[name] !== value) errors.push(`${name}=${value} required for offline preparation`);
  }
  if (!process.env.GOWORK || !path.isAbsolute(process.env.GOWORK)) errors.push("Explicit absolute GOWORK required");
  try {
    const version = await run("go", ["version"]);
    if (!version.includes(" go1.25.13 ")) errors.push(`Go 1.25.13 required; found ${version.trim()}`);
  } catch { errors.push("Missing Go 1.25.13 executable"); }
  const moduleCache = process.env.GOMODCACHE;
  if (!moduleCache) errors.push("Explicit existing GOMODCACHE required");
  else {
    try { await fs.access(path.join(moduleCache, "github.com/princjef/gomarkdoc@v1.1.0")); }
    catch { errors.push(`Missing offline gomarkdoc@v1.1.0 in ${moduleCache}`); }
  }
  return errors;
}

export async function requireGenerationPrerequisites() {
  const errors = await generationPrerequisites();
  if (errors.length) throw new Error(`Full docs generation gate (no installation attempted):\n${errors.join("\n")}`);
}
