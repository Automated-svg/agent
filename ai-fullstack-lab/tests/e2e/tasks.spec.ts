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
  await expect(page.getByRole("alert")).toContainText("无法加载任务");
});
