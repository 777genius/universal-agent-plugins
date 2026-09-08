import { cp, mkdir, mkdtemp, symlink, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

// Explicit disposable fixture only. Never extends or changes production publication.
const fixture = dirname(fileURLToPath(import.meta.url));
const landing = resolve(fixture, '../../..');
const target = await mkdtemp(resolve(tmpdir(), 'uap-i18n-candidate-'));
await cp(fixture, target, { recursive: true, filter: source => !source.endsWith('/prepare.mjs') });
for (const path of [
  'components/layout/LanguageSwitcher.vue', 'composables/useLocation.ts',
  'stores/locale.ts', 'utils/localizedRoutes.ts', 'data/routes.ts',
  'plugins/vuetify.ts', 'locales/en.json',
]) {
  await mkdir(dirname(resolve(target, path)), { recursive: true });
  await cp(resolve(landing, path), resolve(target, path));
}
await cp(resolve(landing, 'data/i18n.ts'), resolve(target, 'data/production-i18n.ts'));
await symlink(resolve(landing, 'node_modules'), resolve(target, 'node_modules'), 'dir');
await writeFile(resolve(target, 'package.json'), JSON.stringify({ private: true, type: 'module' }));
console.log(target);
