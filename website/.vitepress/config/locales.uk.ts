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

export const ukLocaleConfig = {
  label: "Українська",
  lang: "uk-UA",
  link: "/uk/",
  title: "Agent Plugins — документація",
  description: "Установлюйте плагіни для ШІ-агентів або створюйте переносні пакети зі Skills та MCP.",
  themeConfig: {
    langMenuLabel: "Вибрати мову",
    skipToContentLabel: "Перейти до вмісту",
    outlineTitle: "На цій сторінці",
    lastUpdatedText: "Оновлено",
    returnToTopLabel: "На початок",
    sidebarMenuLabel: "Меню",
    darkModeSwitchLabel: "Оформлення",
    darkModeSwitchTitle: "Увімкнути темну тему",
    lightModeSwitchTitle: "Увімкнути світлу тему",
    docFooter: {
      prev: "Попередня сторінка",
      next: "Наступна сторінка"
    },
    footer: {
      message: "Публічна документація для авторів плагінів та інтеграторів.",
      copyright: "Ліцензія Apache-2.0"
    },
    nav: [
      ...journeyNav("uk"),
      { text: "Швидкий старт", link: "/uk/guide/quickstart" },
      { text: "API 0.1.65 · EN", link: "/en/api/cli/prepared-authoring-v2-agentplugins-author" }
    ],
    sidebar: readSidebar("sidebars.uk.json"),
    editLink: {
      pattern: "https://github.com/777genius/universal-agent-plugins/edit/main/website/source/:path",
      text: "Редагувати цю сторінку"
    }
  }
};
