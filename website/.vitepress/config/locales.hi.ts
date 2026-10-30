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

export const hiLocaleConfig = {
  label: "हिन्दी",
  lang: "hi-IN",
  link: "/hi/",
  title: "Agent Plugins दस्तावेज़",
  description: "Agent Plugins के सार्वजनिक दस्तावेज़: प्लगइन इस्तेमाल करें और बनाएँ।",
  themeConfig: {
    skipToContentLabel: "सामग्री पर जाएँ",
    darkModeSwitchTitle: "गहरे रंग की थीम चुनें",
    lightModeSwitchTitle: "हल्के रंग की थीम चुनें",
    outlineTitle: "इस पेज पर",
    lastUpdatedText: "अद्यतन",
    returnToTopLabel: "ऊपर लौटें",
    sidebarMenuLabel: "मेन्यू",
    darkModeSwitchLabel: "रूप-रंग",
    docFooter: {
      prev: "पिछला पेज",
      next: "अगला पेज"
    },
    footer: {
      message: "प्लगइन लेखकों और इंटीग्रेटर के लिए सार्वजनिक दस्तावेज़।",
      copyright: "Apache-2.0 लाइसेंस के अंतर्गत"
    },
    nav: [
      ...journeyNav("hi"),
      { text: "त्वरित शुरुआत", link: "/hi/guide/quickstart" },
      { text: "API 0.1.65 · EN", link: "/en/api/cli/prepared-authoring-v2-agentplugins-author" }
    ],
    sidebar: readSidebar("sidebars.hi.json"),
    editLink: {
      pattern: "https://github.com/777genius/universal-agent-plugins/edit/main/website/source/:path",
      text: "यह पेज संपादित करें"
    }
  }
};
