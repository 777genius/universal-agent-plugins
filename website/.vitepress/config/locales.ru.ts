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
  label: "Русский",
  lang: "ru-RU",
  link: "/ru/",
  themeConfig: {
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
      copyright: "Лицензия MIT"
    },
    nav: [
      ...journeyNav("ru"),
      { text: "Быстрый старт", link: "/ru/guide/quickstart" },
      { text: "API", link: "/en/api/cli/prepared-authoring-v2-agentplugins-author" }
    ],
    sidebar: readSidebar("sidebars.ru.json"),
    editLink: {
      pattern: "https://github.com/777genius/universal-agent-plugins/edit/main/website/source/:path",
      text: "Редактировать страницу"
    }
  }
};
