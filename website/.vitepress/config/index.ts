import { defineConfig } from "vitepress";
import { sharedConfig } from "./shared";
import { enLocaleConfig } from "./locales.en";
import { esLocaleConfig } from "./locales.es";
import { frLocaleConfig } from "./locales.fr";
import { ruLocaleConfig } from "./locales.ru";
import { zhLocaleConfig } from "./locales.zh";
import { ukLocaleConfig } from "./locales.uk";
import { hiLocaleConfig } from "./locales.hi";
import { arLocaleConfig } from "./locales.ar";
import { ptLocaleConfig } from "./locales.pt";

export default defineConfig({
  ...sharedConfig,
  locales: {
    root: sharedConfig.locales?.root,
    en: enLocaleConfig,
    ru: ruLocaleConfig,
    uk: ukLocaleConfig,
    hi: hiLocaleConfig,
    ar: arLocaleConfig,
    pt: ptLocaleConfig,
    es: esLocaleConfig,
    fr: frLocaleConfig,
    zh: zhLocaleConfig
  }
});
