// Primitive validation shared by legacy and signed directory codecs.
import type {
  ClientEvidence,
  ClientID,
  ComponentID,
  DistributionKind,
  PluginAuthor,
  PluginIcon,
  PluginSource,
  TargetAuthentication,
  RegistryPlugin,
} from '../../types/registry';

export const REPOSITORY = /^[a-z0-9](?:[a-z0-9-]{0,37}[a-z0-9])?\/[a-z0-9](?:[a-z0-9._-]{0,98}[a-z0-9])?$/;
export const REVISION = /^[a-f0-9]{40}$/;
export const DIGEST = /^sha256:[a-f0-9]{64}$/;
export const PLUGIN_NAME = /^(?!.*(?:--|\.\.))[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$/;
export const COMPONENTS = new Set<ComponentID>(['extensions', 'mcp', 'skills']);
export const CLIENTS = new Set<ClientID>([
  'codex',
  'chatgpt',
  'cursor',
  'copilot',
  'vscode',
  'kiro',
  'claude',
  'gemini',
  'opencode',
  'cline',
  'windsurf',
  'grok',
  'kimi',
  'qoder',
  'pi',
]);
export const KINDS = new Set<DistributionKind>(['upstream', 'community_bridge', 'community']);
export const RFC3339_INSTANT =
  /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.(\d+))?(Z|[+-]\d{2}:\d{2})$/;
export const EVIDENCE_WORKFLOW =
  /^[a-z0-9][a-z0-9-]*\/[a-z0-9][a-z0-9._-]*\/\.github\/workflows\/[A-Za-z0-9._-]+\.ya?ml$/;
export const EVIDENCE_SOURCE_REF = /^refs\/heads\/[A-Za-z0-9._/-]+$/;
export const JSON_SAFE_INTEGER_MAX = 9_007_199_254_740_991;

export function record(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

export function requiredString(item: Record<string, unknown>, field: string, context: string): string {
  const value = item[field];
  if (typeof value !== 'string' || value.length === 0)
    throw new Error(`${context}: ${field} must be a non-empty string`);
  return value;
}

export function optionalString(item: Record<string, unknown>, field: string): string | undefined {
  return typeof item[field] === 'string' && item[field] ? item[field] : undefined;
}

export function stringArray(value: unknown, field: string, context: string): string[] {
  if (
    !Array.isArray(value) ||
    value.some((entry) => typeof entry !== 'string' || entry.length === 0)
  ) {
    throw new Error(`${context}: ${field} must be an array of non-empty strings`);
  }
  const values = value as string[];
  if (new Set(values).size !== values.length)
    throw new Error(`${context}: ${field} must be unique`);
  return values;
}

export function digestValue(value: unknown, context: string): string {
  if (typeof value !== 'string' || !DIGEST.test(value))
    throw new Error(`${context} must be a sha256 digest`);
  return value;
}

export function safeSequence(value: unknown, context: string): number {
  if (
    typeof value !== 'number' ||
    !Number.isSafeInteger(value) ||
    value < 1 ||
    value > JSON_SAFE_INTEGER_MAX
  ) {
    throw new Error(`${context} must be a safe positive integer`);
  }
  return value;
}

export interface ParsedInstant {
  epochSeconds: number;
  fraction: string;
}

export function parseRFC3339Instant(value: unknown, context: string): ParsedInstant {
  if (typeof value !== 'string') throw new Error(`${context} must be a strict RFC3339 instant`);
  const match = RFC3339_INSTANT.exec(value);
  if (!match) throw new Error(`${context} must be a strict RFC3339 instant`);
  const [, yearText, monthText, dayText, hourText, minuteText, secondText, fraction = '', zone] =
    match;
  const [year, month, day, hour, minute, second] = [
    yearText,
    monthText,
    dayText,
    hourText,
    minuteText,
    secondText,
  ].map(Number);
  const leapYear = year! % 4 === 0 && (year! % 100 !== 0 || year! % 400 === 0);
  const monthDays = [31, leapYear ? 29 : 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31];
  if (
    month! < 1 ||
    month! > 12 ||
    day! < 1 ||
    day! > monthDays[month! - 1]! ||
    hour! > 23 ||
    minute! > 59 ||
    second! > 59
  ) {
    throw new Error(`${context} must be a real RFC3339 calendar instant`);
  }
  let offsetSeconds = 0;
  if (zone !== 'Z') {
    const offsetHour = Number(zone!.slice(1, 3));
    const offsetMinute = Number(zone!.slice(4, 6));
    if (offsetHour > 23 || offsetMinute > 59)
      throw new Error(`${context} has an invalid RFC3339 offset`);
    offsetSeconds = (offsetHour * 60 + offsetMinute) * 60 * (zone![0] === '+' ? 1 : -1);
  }
  const calendar = new Date(0);
  calendar.setUTCFullYear(year!, month! - 1, day!);
  calendar.setUTCHours(hour!, minute!, second!, 0);
  return { epochSeconds: calendar.getTime() / 1000 - offsetSeconds, fraction };
}

export function compareInstants(left: ParsedInstant, right: ParsedInstant): number {
  if (left.epochSeconds !== right.epochSeconds) return left.epochSeconds - right.epochSeconds;
  const width = Math.max(left.fraction.length, right.fraction.length);
  return left.fraction.padEnd(width, '0').localeCompare(right.fraction.padEnd(width, '0'));
}

export function author(value: unknown, context: string): PluginAuthor {
  if (!record(value)) throw new Error(`${context}: author must be an object`);
  const result: PluginAuthor = { name: requiredString(value, 'name', `${context} author`) };
  for (const field of ['email', 'url'] as const) {
    const parsed = optionalString(value, field);
    if (parsed) result[field] = parsed;
  }
  return result;
}

export function source(value: unknown, context: string, allowUnresolved = false): PluginSource {
  if (!record(value)) throw new Error(`${context}: source must be an object`);
  const repository = requiredString(value, 'repository', `${context} source`);
  const revision =
    value.revision === null && allowUnresolved
      ? null
      : requiredString(value, 'revision', `${context} source`);
  if (!REPOSITORY.test(repository)) throw new Error(`${context}: source repository is invalid`);
  if (revision !== null && !REVISION.test(revision))
    throw new Error(`${context}: source revision must be a full commit SHA`);
  return {
    repository,
    revision,
    path: requiredString(value, 'path', `${context} source`),
    manifest_sha256: digestValue(
      value.manifest_sha256 ?? value.manifest_digest,
      `${context} source manifest digest`,
    ),
    tree_sha256: digestValue(
      value.tree_sha256 ?? value.tree_digest,
      `${context} source tree digest`,
    ),
    ...(value.icon_sha256 === undefined
      ? {}
      : { icon_sha256: digestValue(value.icon_sha256, `${context} source icon digest`) }),
  };
}

export function icon(value: unknown, context: string): PluginIcon {
  if (!record(value)) throw new Error(`${context}: icon must be an object`);
  return {
    path: requiredString(value, 'path', `${context} icon`),
    sha256: digestValue(value.sha256, `${context} icon digest`),
  };
}

export function clientIDs(value: unknown, context: string): ClientID[] {
  const values = stringArray(value, 'clients', context);
  if (values.some((client) => !CLIENTS.has(client as ClientID)))
    throw new Error(`${context}: contains an invalid client`);
  return values as ClientID[];
}

export const legacyDelivery: RegistryPlugin['client_support']['delivery'] = {
  codex: 'prepared',
  chatgpt: 'manual_activation',
  cursor: 'managed',
  copilot: 'managed',
  vscode: 'prepared',
  kiro: 'manual_activation',
  claude: 'managed',
  gemini: 'managed',
  opencode: 'managed',
  cline: 'managed',
  windsurf: 'prepared',
  grok: 'managed',
  kimi: 'managed',
  qoder: 'managed',
};

export function legacyAuthentication(name: string): RegistryPlugin['authentication'] {
  if (['agent-code-navigator', 'chrome-devtools', 'cloudflare-docs', 'context7'].includes(name))
    return 'none';
  if (['docker-hub', 'firebase', 'hubspot-developer'].includes(name)) return 'client_managed';
  if (
    [
      'atlassian',
      'cloudflare',
      'cloudflare-bindings',
      'cloudflare-observability',
      'cloudflare-radar',
      'figma',
      'github',
      'gitlab',
      'greptile',
      'heroku',
      'hubspot-crm',
      'linear',
      'neon',
      'notion',
      'sentry',
      'statsig',
      'stripe',
      'supabase',
      'vercel',
    ].includes(name)
  )
    return 'oauth';
  return 'unknown';
}

export function legacyTargetAuthentication(
  authentication: RegistryPlugin['authentication'],
): TargetAuthentication {
  if (authentication === 'none') return 'not_required';
  if (authentication === 'oauth' || authentication === 'client_managed') return 'required';
  return 'unknown';
}

export function legacyEvidence(value: unknown, context: string): ClientEvidence[] {
  if (!record(value)) throw new Error(`${context}: validation must be an object`);
  if (value.schema !== 'agent-plugins-1.0')
    throw new Error(`${context}: validation schema is invalid`);
  const runtime = clientIDs(value.runtime_evidence, `${context} runtime evidence`);
  return runtime.map((client) => ({
    id: `legacy-${client}-runtime`,
    client,
    level: 'runtime',
    outcome: 'passed',
    trusted_for_eligibility: false,
  }));
}

