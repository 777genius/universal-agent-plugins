import { agentDiscovery } from '../../utils/agentDiscovery.ts';

export default defineEventHandler((event) => {
  const config = useRuntimeConfig(event);
  setHeader(event, 'content-type', 'text/plain; charset=utf-8');
  return agentDiscovery(config.public.siteUrl, config.app.baseURL, config.public.docsUrl).llms;
});
