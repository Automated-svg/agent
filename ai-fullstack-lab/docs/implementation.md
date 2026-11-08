# 实操：从启动到新功能交付

## 一、验证现有全栈路径

按 README 启动服务，在页面添加“完成第一轮契约联调”，将状态切换到进行中、已完成，刷新后仍保留，再删除任务。

也可直接请求后端；将创建响应里的 id 代入后续命令：

```bash
curl -s http://127.0.0.1:8080/api/v1/health
curl -s -X POST http://127.0.0.1:8080/api/v1/tasks   -H 'Content-Type: application/json' -d '{"title":"完成契约联调"}'
curl -s http://127.0.0.1:8080/api/v1/tasks
curl -s -X PATCH http://127.0.0.1:8080/api/v1/tasks/REPLACE_ID   -H 'Content-Type: application/json' -d '{"status":"done"}'
curl -i -X DELETE http://127.0.0.1:8080/api/v1/tasks/REPLACE_ID
```

浏览器请求走 3000 端口的 `/api/v1/...`，通过 Next rewrite 到 Go。用 3000 端口执行同样的 curl 即可验证代理链路。当前提供健康检查、列表、创建、状态更新和删除；不存在单条任务 GET 接口。

## 二、用 Agent 实现优先级

需求在 `specs/examples/task-priority.md`，它尚未实现，留作完整实践；不应先把功能写好再假装 Agent 开发。

1. 确认基线测试通过，把需求改成你的业务要求，写清输入、界面行为、异常与兼容性。
2. 在独立分支保存当前基线，运行 `npm run agent:plan` 预览。
3. `npm run agent:run` 开始真实模型调用。观察终端阶段与 `.automation/runs/` 日志。
4. 检查 design.md 是否包含旧文件数据缺字段时的默认值；检查契约是否保持旧创建/更新请求兼容。
5. 验收时应看到契约、Go 实现、前端、E2E 与文档同时更新，且两条门禁命令均通过。
6. 审查 diff、日志、UI，再自行提交或发布。

若手工实践：先改契约并生成；再实现 Go 默认值与历史数据兼容；再通过生成客户端接入选择器和卡片标签；最后写“创建默认值/非法输入/更新/旧数据恢复”的 API 测试与浏览器用例。请求类型里可选的新字段不应让旧客户端失效。

## 三、常用命令

| 命令 | 行为 |
|---|---|
| npm run dev | 构建 Go、启动 Go 与 Next.js、处理退出信号 |
| npm run generate | 重生成两端 API 代码 |
| npm run contracts:check | 临时生成并比较，检测契约漂移 |
| npm run check | 确定性质量门禁和生产构建 |
| npm run test:e2e | 独立端口、独立文件的真实联调 |
| python3 -m unittest discover -s tests/automation -v | 编排器控制逻辑测试，无真实模型调用 |
| npm run agent:plan | 只预览 |
| npm run agent:run | 执行真实 AI 开发流水线 |

单独启动后端：进入 `apps/api`，运行 `go run ./cmd/server`。默认存储文件相对当前目录，因此与根目录编排启动的数据文件不同；要共享时显式设置 DATA_FILE 的绝对路径。

## 四、容器启动

需要自行安装 Docker Engine / Docker Desktop 与 Compose：

```bash
docker compose up --build
```

前端 http://localhost:3000 ，API 只在 Compose 网络内访问，数据放在 `task-data` 卷。`docker compose down` 保留数据卷；无需删除卷。镜像配置提供生产构建路径，本次环境没有 Docker，未实际构建验证。

生产方式为 Node.js 承载 Next.js，服务端 rewrite 代理 Go；不是静态导出。Compose 中 API_ORIGIN 在前端镜像构建时设置为 `http://api:8080`，改变部署地址时重新构建前端。真实上线应补鉴权、TLS、数据库、备份和平台密钥配置。

## 五、CI 接入

所有文件均放在用户指定的新子目录内，因此当前父仓库的 GitHub Actions **不会扫描子目录里的 workflow**。

把 `ai-fullstack-lab` 作为独立仓库根目录时，`.github/workflows/ci.yml` 可直接工作；它使用根目录 package-lock.json 与 apps/api/go.mod。若保留父仓库结构，则将 workflow 放到父仓库 `.github/workflows/`，并设置每个 run 的 working-directory 为 ai-fullstack-lab、缓存/产物路径加上该前缀。

CI 只执行确定性检查、浏览器测试和编排器单元测试，不调用模型。需要 CI 自动开发时，应另建由维护者触发的受控任务，并把模型调用与运行仓库代码的密钥环境隔离。

## 六、排错

- 端口占用：停止自己的旧进程或使用 WEB_PORT / API_PORT；E2E 不复用已有服务器。
- 页面无法加载：先请求 API 健康检查，再检查 Next 代理配置与两端终端日志。
- 契约漂移：运行 generate，审查生成 diff，再 check；不要手改生成文件。
- Go 文件损坏：先备份 `.var/tasks.json`，检查 JSON；服务不会静默清空坏数据。
- 浏览器未安装：运行 Playwright install；网络受限时按失败日志安装对应浏览器版本。
- Agent blocked：看阶段 stderr 和 manifest；明确修复后重新运行，不把 blocked 当作通过。
