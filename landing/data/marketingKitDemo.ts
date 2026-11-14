import type { RegistryPlugin } from '../types/registry';

/**
 * This is a declarative, verified package-version example, not a claim that
 * 'add marketing-kit@0.1.0' is supported as a positional CLI selector.
 * The three target IDs match UAP's real manifest contract.
 */
export const marketingKitName = 'marketing-kit';
export const marketingKitVersion = '0.1.0';
export const marketingKitDependency = '777genius/marketing-kit';
export const marketingKitUserManifest = `schema_version = 1

[dependencies]
"777genius/marketing-kit" = "0.1.0"

[targets]
claude = ["777genius/marketing-kit"]
codex = ["777genius/marketing-kit"]
gemini = ["777genius/marketing-kit"]`;

// Source argument installs the published Marketing Kit and its dependencies.
// Exact 0.1.0 belongs in the manifest and resulting lock; CLI does not expose
// a marketing-kit@0.1.0 positional selector. Schema-1 published policies
// remain User-only publication evidence; released CLI 0.1.101 separately
// admits explicit Project placement after checking dependencies and adapters.
export const marketingKitInstallCommand = 'npx -y universal-agent-plugins install marketing-kit --scope user';
// Import uap.toml + uap.lock into this user environment first, then converge.
export const marketingKitRestoreCommand = 'npx -y universal-agent-plugins install --scope user';

/** Names denote declared compatible targets, never detection of local installs. */
export const marketingKitTargetIDs = ['claude', 'codex', 'gemini'] as const;

// Only a signed/reviewed canonical Directory listing may use this exact
// manifest-first Marketing Kit example; unrelated listings can share a name.
export function isReviewedMarketingKit(plugin: RegistryPlugin): boolean {
  return plugin.name === marketingKitName &&
    plugin.install_source === marketingKitName &&
    plugin.trust_state === 'reviewed' &&
    plugin.installable;
}
