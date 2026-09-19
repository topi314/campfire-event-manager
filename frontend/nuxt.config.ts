export default defineNuxtConfig({
  ssr: false,
  compatibilityDate: "2025-01-01",
  css: ["leaflet/dist/leaflet.css", "~/assets/css/main.css"],
  runtimeConfig: {
    public: {
      apiBase: process.env.NUXT_PUBLIC_API_BASE || "",
    },
  },
  nitro: {
    preset: "static",
  },
  routeRules: {
    "/api/**": { proxy: "http://127.0.0.1:8080/api/**" },
    "/auth/**": { proxy: "http://127.0.0.1:8080/auth/**" },
    "/logout": { proxy: "http://127.0.0.1:8080/logout" },
  },
  app: {
    head: {
      title: "Campfire Event Manager",
      meta: [{ name: "viewport", content: "width=device-width, initial-scale=1" }],
    },
  },
  vite: {
    server: {
      proxy: {
        "/api": { target: "http://127.0.0.1:8080", changeOrigin: true },
        "/auth": { target: "http://127.0.0.1:8080", changeOrigin: true },
        "/logout": { target: "http://127.0.0.1:8080", changeOrigin: true },
      },
    },
    optimizeDeps: {
      include: ["leaflet"],
    },
  },
});
