import { defineConfig, devices } from "@playwright/test";

// Testa o build de produção da interface com a ponte MyGo simulada (e2e/bridge.ts).
// O app desktop em si não roda aqui: falta o runtime nativo.
export default defineConfig({
  testDir: "e2e",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: 0,
  reporter: process.env.CI ? [["github"], ["list"]] : "list",
  // Os testes conferem os textos em português; o de idioma troca para inglês.
  use: { baseURL: "http://127.0.0.1:4173", trace: "retain-on-failure", locale: "pt-BR" },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: {
    command: "npm run build:web && npx vite preview --host 127.0.0.1 --port 4173 --strictPort",
    url: "http://127.0.0.1:4173",
    reuseExistingServer: false,
    timeout: 120_000,
  },
});
