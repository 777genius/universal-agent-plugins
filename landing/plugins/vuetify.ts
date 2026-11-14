import "vuetify/styles";
import { createVuetify } from "vuetify";
import { aliases, mdi } from "vuetify/iconsets/mdi-svg";

export default defineNuxtPlugin({
  name: "vuetify",
  setup(nuxtApp) {
    const vuetify = createVuetify({
      icons: {
        defaultSet: "mdi",
        aliases,
        sets: { mdi }
      },
      theme: {
        defaultTheme: "dark",
        themes: {
          light: {
            colors: {
              primary: "#00f0ff",
              secondary: "#ff00ff",
              background: "#f0f2f5",
              surface: "#ffffff"
            }
          },
          dark: {
            colors: {
              primary: "#00f0ff",
              secondary: "#ff00ff",
              background: "#0a0a0f",
              surface: "#12121a"
            }
          }
        }
      }
    });

    nuxtApp.vueApp.use(vuetify);
    nuxtApp.provide("vuetifyTheme", vuetify.theme);
  }
});
