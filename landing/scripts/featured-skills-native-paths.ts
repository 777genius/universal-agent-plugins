import { existsSync, lstatSync, readFileSync } from 'node:fs';
import { dirname, isAbsolute, join, relative, resolve, sep } from 'node:path';

// Native clients choose their own project Skill directory names. Claude uses
// managed physical skill IDs; Codex/OpenCode may expose the original slug.
// Discover the one exact source-named native SKILL.md under the selected root,
// without trusting a guessed slug path or following foreign symlinks.
export function installedDirectory(output: string, slug: string, targetId: string, repeated = false): string | null {
  try {
    const response = JSON.parse(output) as Record<string, unknown>;
    if (response.command !== 'skills add' || response.result !== 'success' ||
        response.status !== 'succeeded' || response.scope !== 'project' ||
        !Array.isArray(response.items) || response.items.length !== 1) return null;
    const item = response.items[0] as Record<string, unknown>;
    if (item.alias !== slug || item.targetId !== targetId ||
        (item.status !== (repeated ? 'unchanged' : 'succeeded')) ||
        typeof item.destinationPath !== 'string' || !isAbsolute(item.destinationPath) ||
        resolve(item.destinationPath) !== item.destinationPath) return null;
    return item.destinationPath;
  } catch {
    return null;
  }
}

// The official client can install a selected Skill under a physical ID (e.g.
// Claude's skill-<digest>) rather than the original slug. Validate the CLI's
// exact source-bound destination and expected native projection. This does not
// infer or claim that a fresh vendor session activated the Skill.
export function nativeSkillFile(
  project: string, relativeRoot: string, reportedDirectory: string | null, targetId: string, slug: string,
): string | null {
  if (!reportedDirectory || !isAbsolute(reportedDirectory) || resolve(reportedDirectory) !== reportedDirectory ||
      !isAbsolute(project) || resolve(project) !== project || dirname(slug) !== '.' ||
      slug === '.' || slug === '..' || slug.includes('\\') || slug.includes('\0')) return null;
  const root = resolve(project, relativeRoot);
  const reported = resolve(reportedDirectory);
  // Claude's physical receipt is its native skill root; Codex/OpenCode
  // separately project a managed package into the native per-skill directory.
  const expectedReceiptParent = targetId === 'claude' ? root :
    resolve(project, '.uap', 'managed', 'clients', targetId);
  // Claude Project uses the released marketplace bundle wrapper. Its native
  // projection is inside that bundle, unlike the User-scope runtime layout.
  const directory = targetId === 'claude'
    ? join(reported, '.agentplugins-plugin', '.agentplugins-runtime', 'skills', slug)
    : join(root, slug);
  const expectedSkillParent = targetId === 'claude'
    ? join(reported, '.agentplugins-plugin', '.agentplugins-runtime', 'skills') : root;
  if (dirname(reported) !== expectedReceiptParent || dirname(directory) !== expectedSkillParent ||
      !ordinaryDirectoryChain(project, root) || !ordinaryDirectoryChain(project, reported) ||
      !ordinaryDirectoryChain(project, directory)) return null;
  const file = join(directory, 'SKILL.md');
  if (!existsSync(file) || !lstatSync(file).isFile() || lstatSync(file).isSymbolicLink())
    return null;
  const lines = readFileSync(file, 'utf8').split(/\r?\n/);
  if (lines[0] !== '---') return null;
  const end = lines.indexOf('---', 1);
  if (end < 0 || !lines.slice(1, end).some((line) => line.startsWith('name:')) ||
      !lines.slice(1, end).some((line) => line.startsWith('description:'))) return null;
  return file;
}

// The selected root itself can look ordinary after following a symlink in an
// earlier component. Check every owned-project ancestor before reading a leaf.
function ordinaryDirectoryChain(project: string, directory: string): boolean {
  const child = relative(project, directory);
  if (isAbsolute(child) || child === '..' || child.startsWith('..' + sep)) return false;
  let current = project;
  for (const segment of ['', ...child.split(sep).filter(Boolean)]) {
    current = segment ? join(current, segment) : current;
    if (!existsSync(current) || !lstatSync(current).isDirectory() || lstatSync(current).isSymbolicLink())
      return false;
  }
  return true;
}
