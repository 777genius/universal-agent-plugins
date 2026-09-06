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

export const esLocaleConfig = {
  label: "Español",
  lang: "es-ES",
  link: "/es/",
  themeConfig: {
    outlineTitle: "En esta página",
    lastUpdatedText: "Actualizado",
    returnToTopLabel: "Volver arriba",
    sidebarMenuLabel: "Menú",
    darkModeSwitchLabel: "Apariencia",
    docFooter: {
      prev: "Página anterior",
      next: "Página siguiente"
    },
    footer: {
      message: "Documentación pública para autores de plugins e integradores.",
      copyright: "Licencia MIT"
    },
    nav: [
      { text: "Usar plugins", link: "/es/use/" },
      { text: "Crear plugins", link: "/es/build/" },
      { text: "Guía · v1", link: "/es/guide/" },
      { text: "Conceptos · v1", link: "/es/concepts/" },
      { text: "Referencia · v1", link: "/es/reference/" },
      { text: "API · v1", link: "/es/api/" },
      { text: "Lanzamientos", link: "/es/releases/" }
    ],
    sidebar: readSidebar("sidebars.es.json"),
    editLink: {
      pattern: "https://github.com/777genius/universal-agent-plugins/edit/main/website/source/:path",
      text: "Editar esta página"
    }
  }
};
