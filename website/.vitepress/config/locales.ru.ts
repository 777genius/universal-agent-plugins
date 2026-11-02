import { journeyNav } from "../../tools/lib/journeys.mjs";
import fs from "node:fs";
import path from "node:path";

const registryRoot = path.resolve(__dirname, "..", "..", "generated", "registries");

function readSidebar(fileName: string) {
  const full = path.join(registryRoot, fileName);
  if (!fs.existsSync(full)) {
    return {};
  }
  return JSON.parse(fs.readFileSync(full, "utf8"));
}

export const ruLocaleConfig = {
  title: "Agent Plugins · Документация",
  description: "Руководства по установке, управлению и созданию Agent Plugins.",
  label: "Русский",
  lang: "ru-RU",
  link: "/ru/",
  themeConfig: {
    langMenuLabel: "Выбрать язык",
    lightModeSwitchTitle: "Включить светлую тему",
    darkModeSwitchTitle: "Включить тёмную тему",
    skipToContentLabel: "Перейти к содержимому",
    outlineTitle: "На этой странице",
    lastUpdatedText: "Обновлено",
    returnToTopLabel: "Наверх",
    sidebarMenuLabel: "Меню",
    darkModeSwitchLabel: "Оформление",
    docFooter: {
      prev: "Предыдущая страница",
      next: "Следующая страница"
    },
    footer: {
      message: "Публичная документация для авторов плагинов и интеграторов.",
      copyright: "Лицензия Apache 2.0"
    },
    nav: [
      ...journeyNav("ru"),
      { text: "Быстрый старт", link: "/ru/guide/quickstart" },
      { text: "API 0.1.65 · EN", link: "/en/api/cli/prepared-authoring-v2-agentplugins-author" }
    ],
    sidebar: readSidebar("sidebars.ru.json"),
    editLink: {
      pattern: "https://github.com/777genius/universal-agent-plugins/edit/main/website/source/:path",
      text: "Редактировать страницу"
    }
  }
};
