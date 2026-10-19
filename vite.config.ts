import { defineConfig } from "vite";

export default defineConfig({
  base: "./",
  server: { host: "127.0.0.1", port: 5173, strictPort: true },
  build: {
    target: "es2022",
    // A webview carrega do disco, sem rede: um bundle único de ~540 kB não atrasa nada.
    chunkSizeWarningLimit: 800,
    rolldownOptions: {
      // O Fluent marca os módulos com "use client" para o React Server Components;
      // num app só de cliente a diretiva não significa nada.
      onwarn(warning, warn) {
        if (warning.code === "MODULE_LEVEL_DIRECTIVE" && warning.message.includes('"use client"')) return;
        warn(warning);
      },
    },
  },
});
