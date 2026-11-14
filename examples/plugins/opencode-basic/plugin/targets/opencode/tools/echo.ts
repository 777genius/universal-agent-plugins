import { mkdirSync, writeFileSync } from "node:fs"
import { dirname } from "node:path"
import { tool } from "@opencode-ai/plugin"

function writeSmokeMarker() {
  const markerPath = process.env.PLUGIN_KIT_AI_OPENCODE_TOOL_SMOKE_MARKER
  if (!markerPath) {
    return
  }
  mkdirSync(dirname(markerPath), { recursive: true })
  writeFileSync(
    markerPath,
    JSON.stringify({ surface: "standalone-tool", file: "echo.ts" }) + "\n",
    "utf8",
  )
}

writeSmokeMarker()

export default tool({
  description: "Echo a short value from a standalone OpenCode tool file.",
  args: {
    value: tool.schema.string().describe("Value to echo back"),
  },
  async execute(args) {
    return {
      ok: true,
      value: args.value,
    }
  },
})
