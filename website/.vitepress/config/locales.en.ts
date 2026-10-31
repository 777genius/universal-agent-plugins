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

export const enLocaleConfig = {
  title: "Agent Plugins Docs",
  description: "Public guides for installing, managing and building Agent Plugins.",
  label: "English",
  lang: "en-US",
  link: "/en/",
  themeConfig: {
    langMenuLabel: "Change language",
    lightModeSwitchTitle: "Switch to light theme",
    darkModeSwitchTitle: "Switch to dark theme",
    skipToContentLabel: "Skip to content",
    outlineTitle: "On this page",
    lastUpdatedText: "Updated",
    returnToTopLabel: "Return to top",
    sidebarMenuLabel: "Menu",
    darkModeSwitchLabel: "Appearance",
    docFooter: {
      prev: "Previous page",
      next: "Next page"
    },
    footer: {
      message: "Public docs for plugin authors and integrators.",
      copyright: "Apache-2.0 Licensed"
    },
    nav: [
      ...journeyNav("en"),
      { text: "Quickstart", link: "/en/guide/quickstart" },
      { text: "API", link: "/en/api/cli/prepared-authoring-v2-agentplugins-author" }
    ],
    sidebar: readSidebar("sidebars.en.json"),
    editLink: {
      pattern: "https://github.com/777genius/universal-agent-plugins/edit/main/website/source/:path",
      text: "Edit this page"
    }
  }
};
