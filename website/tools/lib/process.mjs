import { execFile } from "node:child_process";
import { promisify } from "node:util";

const execFileAsync = promisify(execFile);

export async function run(command, args, options = {}) {
  const { stdout, stderr } = await execFileAsync(command, args, {
    cwd: options.cwd,
    env: { ...process.env, ...(options.env || {}) },
    maxBuffer: 20 * 1024 * 1024
  });
  if (options.allowStderr !== true && stderr && stderr.trim()) {
    process.stderr.write(stderr);
  }
  return stdout;
}
