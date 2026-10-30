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

export const arLocaleConfig = {
  label: "العربية",
  lang: "ar",
  link: "/ar/",
  title: "وثائق Agent Plugins",
  description: "الوثائق العامة لـ Agent Plugins: استخدام الإضافات وبناؤها.",
  dir: "rtl",
  themeConfig: {
    skipToContentLabel: "انتقل إلى المحتوى",
    darkModeSwitchTitle: "التبديل إلى المظهر الداكن",
    lightModeSwitchTitle: "التبديل إلى المظهر الفاتح",
    outlineTitle: "في هذه الصفحة",
    lastUpdatedText: "آخر تحديث",
    returnToTopLabel: "العودة إلى الأعلى",
    sidebarMenuLabel: "القائمة",
    darkModeSwitchLabel: "المظهر",
    docFooter: {
      prev: "الصفحة السابقة",
      next: "الصفحة التالية"
    },
    footer: {
      message: "وثائق عامة لمؤلفي الإضافات والقائمين على تكاملها.",
      copyright: "مرخّص بموجب Apache-2.0"
    },
    nav: [
      ...journeyNav("ar"),
      { text: "البدء السريع", link: "/ar/guide/quickstart" },
      { text: "API 0.1.65 · EN", link: "/en/api/cli/prepared-authoring-v2-agentplugins-author" }
    ],
    sidebar: readSidebar("sidebars.ar.json"),
    editLink: {
      pattern: "https://github.com/777genius/universal-agent-plugins/edit/main/website/source/:path",
      text: "تعديل هذه الصفحة"
    }
  }
};
