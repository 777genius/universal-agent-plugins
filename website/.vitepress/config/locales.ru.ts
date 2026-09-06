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
      { text: "v1", items: [
        { text: "Гайды · v1", link: "/ru/guide/" },
        { text: "Концепции · v1", link: "/ru/concepts/" },
        { text: "Справочник · v1", link: "/ru/reference/" },
        { text: "API · v1", link: "/ru/api/" },
      ] },
      { text: "Релизы", link: "/ru/releases/" }
    ],
    sidebar: readSidebar("sidebars.ru.json"),
    editLink: {
      pattern: "https://github.com/777genius/universal-agent-plugins/edit/main/website/source/:path",
      text: "Редактировать страницу"
    }
  }
};
