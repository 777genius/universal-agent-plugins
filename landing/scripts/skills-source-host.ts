// Validate immutable Skill fixture provenance by URL host, never a substring.
export function isGitHubSkillSource(source: string): boolean {
  let url: URL;
  try {
    url = new URL(source);
  } catch {
    return false;
  }
  return url.protocol === 'https:' && !url.username && !url.password
    && (url.hostname === 'github.com' || url.hostname === 'raw.githubusercontent.com');
}
