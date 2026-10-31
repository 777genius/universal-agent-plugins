// Inert native loader witness: the registered command never calls a provider.
import { appendFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
const record = (value: Record<string, unknown>) => {
  const path = process.env.UAP_PI_SDK_RECORDS;
  if (!path) throw new Error('Missing disposable native witness path');
  appendFileSync(path, JSON.stringify({ module: fileURLToPath(import.meta.url), ...value }) + '\n');
};
record({ type: 'top-level' });
export default async function (pi: { registerCommand(name: string, command: { description: string; handler(): Promise<void> }): void }, descriptor: unknown) {
  record({ type: 'factory', descriptor });
  pi.registerCommand('uap-sdk-witness', {
    description: 'Inert UAP SDK native extension witness',
    handler: async () => record({ type: 'command' }),
  });
}
