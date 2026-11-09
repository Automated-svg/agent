import test from "node:test";
import assert from "node:assert/strict";
import { createServer } from "node:http";
import { spawn } from "node:child_process";
import { once } from "node:events";
import { fileURLToPath } from "node:url";
import { parsePort, assertPortsAvailable, waitForHealth } from "../../scripts/dev-runtime.mjs";

const root = fileURLToPath(new URL("../../", import.meta.url));
async function serve(t, handler) {
  const server = createServer(handler);
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  t.after(async () => { server.closeAllConnections(); await new Promise(resolve => server.close(resolve)); });
  return { server, port: server.address().port, url: `http://127.0.0.1:${server.address().port}` };
}
async function freePort() {
  const server = createServer();
  server.listen(0, "127.0.0.1"); await once(server, "listening");
  const port = server.address().port;
  await new Promise(resolve => server.close(resolve));
  return port;
}

test("端口配置拒绝非整数、零与超范围值", () => {
  assert.equal(parsePort("WEB_PORT", "3000"), 3000);
  assert.equal(parsePort("API_PORT", "65535"), 65535);
  for (const value of ["", "abc", "1.5", "-1", "0", "65536", "3000x"]) {
    assert.throws(() => parsePort("WEB_PORT", value), /WEB_PORT/);
  }
});

test("两个服务不能使用同一端口", async () => {
  await assert.rejects(assertPortsAvailable([3000, 3000]), /不可相同/);
});

test("发现已占用端口时释放先前的探测监听", async t => {
  const occupied = await serve(t, (_req, res) => res.end("existing"));
  const first = await freePort();
  await assert.rejects(assertPortsAvailable([first, occupied.port]), /无法使用/);
  await assertPortsAvailable([first]);
  assert.equal(await (await fetch(occupied.url)).text(), "existing");
});

test("健康检查等待真正的ok响应，可从暂时失败恢复", async t => {
  let requests = 0;
  const { url } = await serve(t, (req, res) => {
    assert.equal(req.url, "/api/v1/health");
    if (++requests === 1) { res.writeHead(503); res.end(); }
    else { res.setHeader("content-type", "application/json"); res.end('{"status":"ok"}'); }
  });
  await waitForHealth(url, { timeoutMs: 2000 });
  assert.equal(requests, 2);
});

test("HTTP 200但没有正确健康状态时不能判为就绪", async t => {
  const { url } = await serve(t, (_req, res) => res.end('{"status":"unavailable"}'));
  await assert.rejects(waitForHealth(url, { timeoutMs: 150 }), /就绪超时/);
});

test("挂起的健康响应受整体期限限制", async t => {
  const { url } = await serve(t, () => {});
  await assert.rejects(waitForHealth(url, { timeoutMs: 150 }), /就绪超时/);
});

test("中断启动会取消正在等待的健康请求", async t => {
  const controller = new AbortController();
  const { url } = await serve(t, () => controller.abort());
  await assert.rejects(waitForHealth(url, { signal: controller.signal, timeoutMs: 2000 }), { name: "AbortError" });
});

test("API子进程退出后不接受其他服务的健康响应", async () => {
  await assert.rejects(waitForHealth("http://127.0.0.1:1", { hasExited: () => true }), /进程已退出/);
});

test("开发CLI在端口冲突时停止，不构建或误连已有服务", { timeout: 5000 }, async t => {
  let requests = 0;
  const existing = await serve(t, (_req, res) => { requests++; res.end('{"status":"ok"}'); });
  const child = spawn(process.execPath, ["scripts/dev.mjs"], {
    cwd: root, env: { ...process.env, WEB_PORT: String(existing.port), API_PORT: "65534" },
    stdio: ["ignore", "pipe", "pipe"],
  });
  t.after(() => { if (child.exitCode === null) child.kill("SIGKILL"); });
  let output = "";
  child.stdout.on("data", data => { output += data; });
  child.stderr.on("data", data => { output += data; });
  const [code] = await once(child, "close");
  assert.equal(code, 1);
  assert.match(output, /端口.*无法使用/);
  assert.equal(requests, 0);
});
