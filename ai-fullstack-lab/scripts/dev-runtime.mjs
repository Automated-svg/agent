import { createServer } from "node:net";
import { setTimeout as delay } from "node:timers/promises";

export function parsePort(name, value) {
  if (!/^\d+$/.test(String(value)) || Number(value) < 1 || Number(value) > 65535) {
    throw new Error(`${name} 必须是 1–65535 的整数`);
  }
  return Number(value);
}

// 探测期间保留监听，再统一释放；实际启动仍可能受到其他进程竞争。
export async function assertPortsAvailable(ports) {
  if (new Set(ports).size !== ports.length) throw new Error("WEB_PORT 与 API_PORT 不可相同");
  const listeners = [];
  try {
    for (const port of ports) {
      const server = createServer();
      listeners.push(server);
      await new Promise((resolve, reject) => {
        server.once("error", error => reject(new Error(`端口 ${port} 无法使用：${error.code}`, { cause: error })));
        server.listen(port, "127.0.0.1", resolve);
      });
    }
  } finally {
    await Promise.all(listeners.filter(server => server.listening).map(server => new Promise((resolve, reject) => {
      server.close(error => error ? reject(error) : resolve());
    })));
  }
}

export async function waitForHealth(origin, { timeoutMs = 10000, signal, hasExited = () => false } = {}) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    signal?.throwIfAborted();
    if (hasExited()) throw new Error("API 进程已退出，请检查启动日志");
    const probe = AbortSignal.timeout(Math.max(1, Math.min(500, deadline - Date.now())));
    try {
      const response = await fetch(`${origin}/api/v1/health`, {
        signal: signal ? AbortSignal.any([signal, probe]) : probe,
      });
      if (response.ok && (await response.json()).status === "ok") {
        signal?.throwIfAborted();
        if (hasExited()) throw new Error("API 进程已退出，请检查启动日志");
        return;
      }
      await response.body?.cancel();
    } catch {
      signal?.throwIfAborted();
    }
    if (Date.now() < deadline) await delay(Math.min(100, deadline - Date.now()), undefined, { signal });
  }
  throw new Error(`API 就绪超时（${timeoutMs}ms），请检查服务和端口`);
}
