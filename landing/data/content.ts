import en from '~/content/en.json';
import ru from '~/content/ru.json';
import es from '~/content/es.json';
import fr from '~/content/fr.json';
import zh from '~/content/zh.json';
import { resolvePluginLogo } from '~/data/pluginLogos';
import type { LandingContent, LocalizedContent, PluginCard } from '~/types/content';
import type { LocaleCode } from '~/data/i18n';

export const contentByLocale = {
  en: en as LandingContent,
  ru: ru as LandingContent,
  es: es as LandingContent,
  fr: fr as LandingContent,
  zh: zh as LandingContent,
} satisfies LocalizedContent;

const normalizePlugins = (plugins: PluginCard[]): PluginCard[] =>
  plugins.map((plugin) => {
    const resolvedLogo = resolvePluginLogo(plugin.logoSrc);

    return {
      ...plugin,
      slug: plugin.slug || plugin.id,
      pluginType: plugin.pluginType ?? 'online-service',
      logoSrc: resolvedLogo.src,
      logoSurface: resolvedLogo.surface ?? plugin.logoSurface ?? 'default',
    };
  });

export const getContent = (locale: LocaleCode): LandingContent => {
  const content = contentByLocale[locale] ?? contentByLocale.en;

  return {
    ...content,
    plugins: normalizePlugins(content.plugins),
  };
};

export const getPluginBySlug = (locale: LocaleCode, slug: string): PluginCard | undefined => {
  return getContent(locale).plugins.find((plugin) => plugin.slug === slug);
};
