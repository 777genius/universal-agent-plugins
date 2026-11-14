import { mkdirSync, writeFileSync } from "node:fs"
import { dirname } from "node:path"

function writeSmokeMarker(directory, worktree) {
  const markerPath = process.env.PLUGIN_KIT_AI_OPENCODE_SMOKE_MARKER
  if (!markerPath) {
    return
  }
  mkdirSync(dirname(markerPath), { recursive: true })
  writeFileSync(
    markerPath,
    JSON.stringify({ directory, worktree }) + "\n",
    "utf8",
  )
}

export const ExamplePlugin = async (ctx) => {
  const { directory, worktree } = ctx
  writeSmokeMarker(directory, worktree)
  return {
    "tool.execute.before": async () => {},
  }
}
