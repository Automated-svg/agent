# AI Fullstack Lab

本目录是独立工程边界。回答默认中文；仅修改用户要求的子项目文件。

## 架构与入口

- 前端：apps/web，Next.js App Router + TypeScript。
- 后端：apps/api，Go net/http，JSON 文件持久化，单进程部署。
- 唯一 API 契约：contracts/openapi.yaml。
- 业务与启动说明：README.md、docs/architecture.md。
- Agent 阶段职责：.automation/roles/*.md；工作流：docs/agent-workflow.md。

## 开发规则

修改 API 时先改契约，再 npm run generate；禁止直接编辑生成文件。
前端使用 apps/web/src/lib/api.ts 的类型客户端，同源 /api/v1 代理。
后端通过生成的 StrictServerInterface 实现；新增行为补充失败场景和契约响应测试。
JSON 存储写入成功后才更新内存；不支持多进程共享同一个文件。
不要修改检查命令、弱化测试或关闭类型检查来通过门禁。

## 验收命令

npm run check
npm run test:e2e

首次准备：npm ci；npx playwright install chromium。
未运行的检查明确说明原因，不编造通过证据。

## Agent 配合

日常任务可直接使用 $fullstack-feature。编排器按角色串行运行，无需子 Agent。
只有用户明确要求并行协作时，才在契约确认后委派 frontend / backend；
前端仅写 apps/web、tests/e2e，后端仅写 apps/api，contract 独占 contracts 和生成物。
不要由角色再次委派其他 Agent。工作流输出仅代表检查完成，不自动提交或发布。
