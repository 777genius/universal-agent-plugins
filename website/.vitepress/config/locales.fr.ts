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

export const frLocaleConfig = {
  title: "Agent Plugins · Documentation",
  description: "Guides pour installer, gérer et créer des Agent Plugins.",
  label: "Français",
  lang: "fr-FR",
  link: "/fr/",
  themeConfig: {
    langMenuLabel: "Changer de langue",
    lightModeSwitchTitle: "Activer le thème clair",
    darkModeSwitchTitle: "Activer le thème sombre",
    skipToContentLabel: "Aller au contenu",
    outlineTitle: "Sur cette page",
    lastUpdatedText: "Mis à jour",
    returnToTopLabel: "Retour en haut",
    sidebarMenuLabel: "Menu",
    darkModeSwitchLabel: "Apparence",
    docFooter: {
      prev: "Page précédente",
      next: "Page suivante"
    },
    footer: {
      message: "Documentation publique pour les auteurs de plugins et les intégrateurs.",
      copyright: "Sous licence Apache 2.0"
    },
    nav: [
      ...journeyNav("fr"),
      { text: "Démarrage rapide", link: "/fr/guide/quickstart" },
      { text: "API 0.1.65 · EN", link: "/en/api/cli/prepared-authoring-v2-agentplugins-author" }
    ],
    sidebar: readSidebar("sidebars.fr.json"),
    editLink: {
      pattern: "https://github.com/777genius/universal-agent-plugins/edit/main/website/source/:path",
      text: "Modifier cette page"
    }
  }
};
