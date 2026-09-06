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
  label: "简体中文",
  lang: "zh-CN",
  link: "/zh/",
  themeConfig: {
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
      copyright: "MIT 许可"
    },
    nav: [
      { text: "使用插件", link: "/zh/use/" },
      { text: "构建插件", link: "/zh/build/" },
      { text: "指南 · v1", link: "/zh/guide/" },
      { text: "概念 · v1", link: "/zh/concepts/" },
      { text: "参考 · v1", link: "/zh/reference/" },
      { text: "API · v1", link: "/zh/api/" },
      { text: "发布", link: "/zh/releases/" }
    ],
    sidebar: readSidebar("sidebars.zh.json"),
    editLink: {
      pattern: "https://github.com/777genius/universal-agent-plugins/edit/main/website/source/:path",
      text: "编辑此页"
    }
  }
};
