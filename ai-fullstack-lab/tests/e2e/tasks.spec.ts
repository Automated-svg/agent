import { test, expect } from "@playwright/test";

test("通过 Next.js 代理完成任务全生命周期，并在刷新后保留", async ({ page, request }) => {
  const title = `契约驱动联调 ${Date.now()}`;
  await page.goto("/");
  await page.getByLabel("任务标题").fill(title);
  await page.getByRole("button", { name: "添加任务" }).click();
  const card = page.getByRole("article").filter({ hasText: title });
  await expect(card).toBeVisible();
  await page.getByLabel(`${title} 状态`).selectOption("doing");
  await expect(page.getByRole("region", { name: "进行中", exact: true }).getByText(title)).toBeVisible();
  await page.reload();
  await expect(page.getByLabel(`${title} 状态`)).toHaveValue("doing");
  await page.getByLabel(`${title} 状态`).selectOption("done");
  await expect(page.getByRole("region", { name: "已完成", exact: true }).getByText(title)).toBeVisible();
  await page.getByRole("button", { name: `删除 ${title}` }).click();
  await expect(card).toHaveCount(0);
  const invalid = await request.post("/api/v1/tasks", { data: { title: "" } });
  expect(invalid.status()).toBe(400);
  expect(await invalid.json()).toMatchObject({ code: "bad_request" });
});

test("API 不可用时向用户显示错误", async ({ page }) => {
  await page.route("**/api/v1/tasks", route => route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ code: "unavailable", message: "Unavailable" }) }));
  await page.goto("/");
  await expect(page.getByRole("region", { name: "任务工作区" }).getByRole("alert")).toContainText("无法加载任务");
});

async function clearTask(request: import("@playwright/test").APIRequestContext, title: string) {
  const response = await request.get("/api/v1/tasks");
  expect(response.ok()).toBeTruthy();
  const tasks: { id: string; title: string }[] = await response.json();
  for (const task of tasks.filter(item => item.title === title)) {
    expect((await request.delete(`/api/v1/tasks/${task.id}`)).status()).toBe(204);
  }
}

test("加载失败后无需刷新页面即可重新加载真实任务", async ({ page, request }) => {
  const title = `恢复读取 ${Date.now()}`;
  expect((await request.post("/api/v1/tasks", { data: { title } })).status()).toBe(201);
  let unavailable = true;
  await page.route("**/api/v1/tasks", async route => {
    if (unavailable && route.request().method() === "GET") {
      await route.fulfill({ status: 503, json: { code: "unavailable", message: "Unavailable" } });
    } else await route.continue();
  });
  try {
    await page.goto("/");
    await expect(page.getByRole("region", { name: "任务工作区" }).getByRole("alert")).toBeVisible();
    await expect(page.getByLabel("任务标题")).toBeDisabled();
    await expect(page.getByRole("button", { name: "添加任务" })).toBeDisabled();
    unavailable = false;
    await page.getByRole("button", { name: "重新加载任务" }).click();
    await expect(page.getByRole("article").filter({ hasText: title })).toBeVisible();
    await expect(page.getByRole("region", { name: "任务工作区" }).getByRole("alert")).toHaveCount(0);
  } finally { await clearTask(request, title); }
});

test("保存成功无需再次读取列表，读取故障不误报写入失败", async ({ page, request }) => {
  const title = `可靠保存 ${Date.now()}`;
  await page.goto("/");
  await expect(page.getByRole("region", { name: "待开始", exact: true })).toBeVisible();
  let unexpectedReads = 0;
  await page.route("**/api/v1/tasks", async route => {
    if (route.request().method() === "GET") {
      unexpectedReads++;
      await route.fulfill({ status: 503, json: { code: "unavailable", message: "Unavailable" } });
    } else await route.continue();
  });
  try {
    await page.getByLabel("任务标题").fill(title);
    await page.getByRole("button", { name: "添加任务" }).click();
    await expect(page.getByRole("article").filter({ hasText: title })).toBeVisible();
    await expect(page.getByLabel("任务标题")).toBeEmpty();
    await expect(page.getByRole("region", { name: "任务工作区" }).getByRole("alert")).toHaveCount(0);
    expect(unexpectedReads).toBe(0);
    const persisted: { title: string }[] = await (await request.get("/api/v1/tasks")).json();
    expect(persisted.some(task => task.title === title)).toBeTruthy();
  } finally { await clearTask(request, title); }
});

test("写入失败保留草稿并显示后端错误", async ({ page }) => {
  const title = "失败时保留的草稿";
  await page.route("**/api/v1/tasks", async route => {
    if (route.request().method() === "POST") {
      await route.fulfill({ status: 500, json: { code: "internal_error", message: "服务暂时无法保存任务" } });
    } else await route.continue();
  });
  await page.goto("/");
  await page.getByLabel("任务标题").fill(title);
  await page.getByRole("button", { name: "添加任务" }).click();
  await expect(page.getByRole("region", { name: "任务工作区" }).getByRole("alert")).toHaveText("服务暂时无法保存任务");
  await expect(page.getByLabel("任务标题")).toHaveValue(title);
  await expect(page.getByRole("article").filter({ hasText: title })).toHaveCount(0);
});

test("提交期间锁定草稿并拦截重复提交", async ({ page, request }) => {
  const title = `单次提交 ${Date.now()}`;
  let release!: () => void;
  const gate = new Promise<void>(resolve => { release = resolve; });
  let writes = 0;
  await page.route("**/api/v1/tasks", async route => {
    if (route.request().method() === "POST") { writes++; await gate; }
    await route.continue();
  });
  try {
    await page.goto("/");
    await page.getByLabel("任务标题").fill(title);
    await page.getByRole("button", { name: "添加任务" }).click();
    await expect(page.getByLabel("任务标题")).toBeDisabled();
    await expect(page.getByRole("button", { name: "添加任务" })).toBeDisabled();
    await page.locator("form").evaluate(form => {
      form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
      form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    });
    release();
    await expect(page.getByRole("article").filter({ hasText: title })).toHaveCount(1);
    await expect(page.getByLabel("任务标题")).toBeEnabled();
    expect(writes).toBe(1);
    const persisted: { title: string }[] = await (await request.get("/api/v1/tasks")).json();
    expect(persisted.filter(task => task.title === title)).toHaveLength(1);
  } finally { release(); await clearTask(request, title); }
});
