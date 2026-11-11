import atlassianLogo from '~/assets/images/plugin-logos/atlassian.svg';
import cloudflareLogo from '~/assets/images/plugin-logos/cloudflare.svg';
import context7Logo from '~/assets/images/plugin-logos/context7.webp';
import firebaseLogo from '~/assets/images/plugin-logos/firebase.svg';
import githubLogo from '~/assets/images/plugin-logos/github.svg';
import gitlabLogo from '~/assets/images/plugin-logos/gitlab.svg';
import greptileLogo from '~/assets/images/plugin-logos/greptile.svg';
import herokuLogo from '~/assets/images/plugin-logos/heroku.webp';
import dockerLogo from '~/assets/images/plugin-logos/docker.svg';
import hubspotLogo from '~/assets/images/plugin-logos/hubspot.webp';
import linearLogo from '~/assets/images/plugin-logos/linear.svg';
import neonLogo from '~/assets/images/plugin-logos/neon.svg';
import notionLogo from '~/assets/images/plugin-logos/notion.webp';
import sentryLogo from '~/assets/images/plugin-logos/sentry.svg';
import slackLogo from '~/assets/images/plugin-logos/slack.svg';
import stripeLogo from '~/assets/images/plugin-logos/stripe.svg';
import supabaseLogo from '~/assets/images/plugin-logos/supabase.svg';
import vercelLogo from '~/assets/images/plugin-logos/vercel.webp';
import type { PluginLogoSurface } from '~/types/content';

interface PluginLogoDefinition {
  src: string;
  surface?: PluginLogoSurface;
}

const pluginLogos: Record<string, PluginLogoDefinition> = {
  'atlassian.svg': { src: atlassianLogo },
  'cloudflare.svg': { src: cloudflareLogo },
  'context7.svg': { src: context7Logo },
  'docker.svg': { src: dockerLogo },
  'firebase.svg': { src: firebaseLogo },
  'github.svg': { src: githubLogo, surface: 'light' },
  'gitlab.svg': { src: gitlabLogo },
  'greptile.svg': { src: greptileLogo },
  'heroku.svg': { src: herokuLogo },
  'hubspot.svg': { src: hubspotLogo },
  'linear.svg': { src: linearLogo },
  'neon.svg': { src: neonLogo },
  'notion.svg': { src: notionLogo },
  'sentry.svg': { src: sentryLogo, surface: 'light' },
  'slack.svg': { src: slackLogo },
  'stripe.svg': { src: stripeLogo },
  'supabase.svg': { src: supabaseLogo },
  'vercel.svg': { src: vercelLogo },
};

export const resolvePluginLogo = (logoSrc: string): PluginLogoDefinition => {
  return pluginLogos[logoSrc] ?? { src: logoSrc, surface: 'default' };
};
