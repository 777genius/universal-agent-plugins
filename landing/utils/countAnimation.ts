export const PLUGIN_COUNT_ANIMATION_MS = 2_000;

export function countAtElapsed(
  target: number,
  elapsedMs: number,
  durationMs = PLUGIN_COUNT_ANIMATION_MS,
): number {
  const safeTarget = Math.max(0, Math.floor(target));
  if (safeTarget === 0) return 0;
  if (durationMs <= 0 || elapsedMs >= durationMs) return safeTarget;
  if (elapsedMs <= 0) return 0;

  return Math.floor(safeTarget * (elapsedMs / durationMs));
}
