import { spawn } from "node:child_process";
import { fileURLToPath } from "node:url";
import { resolve } from "node:path";
import { parsePort, assertPortsAvailable, waitForHealth } from "./dev-runtime.mjs";
const root = fileURLToPath(new URL("../", import.meta.url));
const webPort = process.env.WEB_PORT ?? "3000";
const apiPort = process.env.API_PORT ?? "8080";
const apiOrigin = `http://127.0.0.1:${apiPort}`;
const processes = [];
let stopping = false;
const shutdown = new AbortController();
function stop(code = 0) {
  if (stopping) return;
  stopping = true;
  shutdown.abort();
  // 每个服务独立进程组，连同 go run / npm 创建的子进程一起结束。
  for (const child of processes) {
    if (child.pid) { try { process.kill(-child.pid, "SIGTERM"); } catch {} }
  }
  // 忽略TERM的服务在宽限期后强制退出；正常退出无需等待此定时器。
  const escalation = setTimeout(() => {
    for (const child of processes) {
      if (child.pid) { try { process.kill(-child.pid, "SIGKILL"); } catch {} }
    }
  }, 2500);
  escalation.unref();
  process.exitCode = code;
}
function start(command, args, cwd, env) {
  const child = spawn(command, args, { cwd, env: { ...process.env, ...env }, detached: true, stdio: "inherit" });
  processes.push(child);
  child.on("error", error => { console.error(error); stop(1); });
  child.on("exit", code => { if (!stopping) stop(code || 1); });
  return child;
}
process.on("SIGINT", () => stop());
process.on("SIGTERM", () => stop());
try {
  await assertPortsAvailable([parsePort("WEB_PORT", webPort), parsePort("API_PORT", apiPort)]);
  if (stopping) throw new Error("启动已取消");
  // 使用预构建二进制，便于可靠地关闭服务进程。
  const { mkdirSync } = await import("node:fs");
  const { spawnSync } = await import("node:child_process");
  mkdirSync(resolve(root, ".var"), { recursive: true });
  const built = spawnSync("go", ["build", "-o", resolve(root, ".var/api"), "./cmd/server"], { cwd: resolve(root, "apps/api"), stdio: "inherit" });
  if (built.error || built.status !== 0) throw new Error("Go 构建失败");
  const api = start(resolve(root, ".var/api"), [], root, { ADDR: `127.0.0.1:${apiPort}`, DATA_FILE: process.env.DATA_FILE ?? resolve(root, ".var/tasks.json") });
  await waitForHealth(apiOrigin, { signal: shutdown.signal, hasExited: () => api.exitCode !== null || api.signalCode !== null });
  start("npm", ["run", process.env.PRODUCTION === "1" ? "start" : "dev", "-w", "apps/web", "--", "--port", webPort], root, { API_ORIGIN: apiOrigin });
  console.log(`前端 http://127.0.0.1:${webPort}  API ${apiOrigin}`);
} catch (error) { if (!stopping) { console.error(error); stop(1); } }
