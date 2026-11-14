// Execute the supplied official Gemini CLI 0.62.0 resolver unchanged.
import { resolveEnvVarsInString } from './env-var-resolver.ts';
let input = '';
for await (const chunk of process.stdin) input += chunk;
const { command } = JSON.parse(input);
process.stdout.write(JSON.stringify(resolveEnvVarsInString(command, {
  U2_RENDER_VAR: 'native-expanded',
  'U2_RENDER_DEFAULT': 'native-default-expanded',
})));
