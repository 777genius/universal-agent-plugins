/** This extension contributes no definitions, permissions, tools or commands. */
import { run } from './runner.js';

export async function activate(): Promise<void> { await run(); }
export function deactivate(): void {}
