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

export const zhLocaleConfig = {
  title: "Agent Plugins · 文档",
  description: "Agent Plugins 的安装、管理与构建指南。",
  label: "简体中文",
  lang: "zh-CN",
  link: "/zh/",
  themeConfig: {
    langMenuLabel: "切换语言",
    lightModeSwitchTitle: "切换到浅色主题",
    darkModeSwitchTitle: "切换到深色主题",
    skipToContentLabel: "跳转到正文",
    outlineTitle: "本页内容",
    lastUpdatedText: "最近更新",
    returnToTopLabel: "回到顶部",
    sidebarMenuLabel: "菜单",
    darkModeSwitchLabel: "外观",
    docFooter: {
      prev: "上一页",
      next: "下一页"
    },
    footer: {
      message: "面向插件作者和集成者的公共文档。",
      copyright: "采用 Apache 2.0 许可证"
    },
    nav: [
      ...journeyNav("zh"),
      { text: "快速开始", link: "/zh/guide/quickstart" },
      { text: "API 0.1.65 · EN", link: "/en/api/cli/prepared-authoring-v2-agentplugins-author" }
    ],
    sidebar: readSidebar("sidebars.zh.json"),
    editLink: {
      pattern: "https://github.com/777genius/universal-agent-plugins/edit/main/website/source/:path",
      text: "编辑此页"
    }
  }
};
