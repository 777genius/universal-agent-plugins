export type LocaleCode = 'en' | 'ru' | 'es' | 'fr' | 'zh';

export const supportedLocales = [
  { code: 'en', iso: 'en-US', name: 'English', flag: '\u{1F1FA}\u{1F1F8}', file: 'en.json' },
  { code: 'ru', iso: 'ru-RU', name: 'Русский', flag: '\u{1F1F7}\u{1F1FA}', file: 'ru.json' },
  { code: 'es', iso: 'es-ES', name: 'Español', flag: '\u{1F1EA}\u{1F1F8}', file: 'es.json' },
  { code: 'fr', iso: 'fr-FR', name: 'Français', flag: '\u{1F1EB}\u{1F1F7}', file: 'fr.json' },
  { code: 'zh', iso: 'zh-CN', name: '简体中文', flag: '\u{1F1E8}\u{1F1F3}', file: 'zh.json' },
] as const;

export const defaultLocale: LocaleCode = 'en';
