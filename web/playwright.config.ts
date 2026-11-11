import { defineConfig } from "@playwright/test";
import { existsSync } from "node:fs";
export default defineConfig({
  testDir: "./tests",
  workers: 1,
  use: {
    headless: true,
    viewport: { width: 1440, height: 1000 },
    launchOptions: {
      executablePath: process.env.CHROMIUM_PATH ||
        (existsSync("/usr/bin/google-chrome") ? "/usr/bin/google-chrome" : undefined),
      args: ["--no-sandbox"],
    },
  },
});
