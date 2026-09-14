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
  label: "English",
  lang: "en-US",
  link: "/en/",
  themeConfig: {
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
