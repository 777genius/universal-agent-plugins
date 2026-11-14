import DefaultTheme from "vitepress/theme";
import type { Theme } from "vitepress";
import Layout from "./components/Layout.vue";
import LanguageGateway from "./components/LanguageGateway.vue";
import DocMetaCard from "./components/DocMetaCard.vue";
import MermaidDiagram from "./components/MermaidDiagram.vue";
import NotFoundLinks from "./components/NotFoundLinks.vue";
import "./styles/custom.css";

const theme: Theme = {
  ...DefaultTheme,
  Layout,
  enhanceApp({ app }) {
    app.component("LanguageGateway", LanguageGateway);
    app.component("DocMetaCard", DocMetaCard);
    app.component("MermaidDiagram", MermaidDiagram);
    app.component("NotFoundLinks", NotFoundLinks);
  }
};

export default theme;
