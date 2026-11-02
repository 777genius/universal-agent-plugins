// Shared public locale contract: build tools and browser UI use the same order.
// Historical extraction keeps its separately pinned five-language inventory.
export const docsLocaleEntries = [
  { code: "en", label: "English", lang: "en-US", dir: "ltr", description: "Guides for installing, managing and building Agent Plugins.", home: "Home", fallback: "Translation unavailable — English" },
  { code: "ru", label: "Русский", lang: "ru-RU", dir: "ltr", description: "Руководства по установке, управлению и созданию Agent Plugins.", home: "Главная", fallback: "Перевод недоступен — English" },
  { code: "uk", label: "Українська", lang: "uk-UA", dir: "ltr", description: "Посібники зі встановлення, керування та створення Agent Plugins.", home: "Головна", fallback: "Переклад недоступний — English" },
  { code: "zh", label: "简体中文", lang: "zh-CN", dir: "ltr", description: "了解如何安装、管理和构建 Agent Plugins。", home: "首页", fallback: "暂无译文 — English" },
  { code: "es", label: "Español", lang: "es-ES", dir: "ltr", description: "Guías para instalar, gestionar y crear Agent Plugins.", home: "Inicio", fallback: "Traducción no disponible — English" },
  { code: "hi", label: "हिन्दी", lang: "hi-IN", dir: "ltr", description: "Agent Plugins इंस्टॉल करने, प्रबंधित करने और बनाने की मार्गदर्शिकाएँ।", home: "मुखपृष्ठ", fallback: "अनुवाद उपलब्ध नहीं — English" },
  { code: "ar", label: "العربية", lang: "ar", dir: "rtl", description: "أدلة تثبيت Agent Plugins وإدارتها وبنائها.", home: "الرئيسية", fallback: "الترجمة غير متاحة — English" },
  { code: "pt", label: "Português", lang: "pt-BR", dir: "ltr", description: "Guias para instalar, gerenciar e criar Agent Plugins.", home: "Início", fallback: "Tradução indisponível — English" },
  { code: "fr", label: "Français", lang: "fr-FR", dir: "ltr", description: "Guides pour installer, gérer et créer des Agent Plugins.", home: "Accueil", fallback: "Traduction indisponible — English" }
];
export const docsLocales = docsLocaleEntries.map(({ code }) => code);
export const historicalLocales = ["en", "ru", "es", "fr", "zh"];
export const localePathField = (locale) => `path${locale[0].toUpperCase()}${locale.slice(1)}`;
export const localeDefinition = (code) => docsLocaleEntries.find((locale) => locale.code === code) || docsLocaleEntries[0];
