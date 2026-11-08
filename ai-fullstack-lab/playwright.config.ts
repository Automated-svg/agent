import { defineConfig } from "@playwright/test";
import { resolve } from "node:path";
import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
// 每次使用独立存储文件，避免覆盖开发数据；临时数据由操作系统回收。
const dataFile = resolve(mkdtempSync(resolve(tmpdir(), "agent-lab-e2e-")), "tasks.json");
export default defineConfig({
  testDir: "./tests/e2e",
  fullyParallel: false,
  workers: 1,
  retries: process.env.CI ? 1 : 0,
  use: { baseURL: "http://127.0.0.1:3100", trace: "retain-on-failure" },
  webServer: {
    command: "npm run dev",
    url: "http://127.0.0.1:3100",
    reuseExistingServer: false,
    timeout: 120_000,
    env: { WEB_PORT: "3100", API_PORT: "8181", DATA_FILE: dataFile, NEXT_TELEMETRY_DISABLED: "1" },
  },
});
