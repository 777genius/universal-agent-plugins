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

export const ptLocaleConfig = {
  label: "Português",
  lang: "pt-BR",
  link: "/pt/",
  title: "Agent Plugins — documentação",
  description: "Instale plugins para agentes de IA ou crie pacotes portáteis com Skills e MCP.",
  themeConfig: {
    langMenuLabel: "Alterar idioma",
    skipToContentLabel: "Ir para o conteúdo",
    outlineTitle: "Nesta página",
    lastUpdatedText: "Atualizado em",
    returnToTopLabel: "Voltar ao topo",
    sidebarMenuLabel: "Menu",
    darkModeSwitchLabel: "Aparência",
    darkModeSwitchTitle: "Ativar tema escuro",
    lightModeSwitchTitle: "Ativar tema claro",
    docFooter: {
      prev: "Página anterior",
      next: "Próxima página"
    },
    footer: {
      message: "Documentação pública para autores de plugins e integradores.",
      copyright: "Licenciado sob Apache-2.0"
    },
    nav: [
      ...journeyNav("pt"),
      { text: "Início rápido", link: "/pt/guide/quickstart" },
      { text: "API 0.1.65 · EN", link: "/en/api/cli/prepared-authoring-v2-agentplugins-author" }
    ],
    sidebar: readSidebar("sidebars.pt.json"),
    editLink: {
      pattern: "https://github.com/777genius/universal-agent-plugins/edit/main/website/source/:path",
      text: "Editar esta página"
    }
  }
};
