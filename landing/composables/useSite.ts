import type { RegistryPlugin } from '~/types/registry';
import { githubSourceUrl, mirroredIconPath } from '~/utils/registry';
export { clients } from '~/data/clients';

export function useSite() {
  const config = useRuntimeConfig();
  const baseURL = String(config.public.baseURL);
  const repositoryUrl = String(config.public.repositoryUrl);
  const asset = (path: string) => `${baseURL}${path.replace(/^\//, '')}`;

  const pluginIcon = (plugin: RegistryPlugin) => {
    // External author-controlled images are never loaded. A future sanitized
    // mirror can opt entries into locally served assets explicitly.
    const path = mirroredIconPath(plugin);
    return path ? asset(path) : undefined;
  };

  const sourceUrl = (plugin: RegistryPlugin) => {
    return githubSourceUrl(plugin);
  };

  return { asset, baseURL, pluginIcon, repositoryUrl, sourceUrl };
}
