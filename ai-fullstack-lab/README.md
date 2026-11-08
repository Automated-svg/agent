# AI Fullstack Lab

可直接运行的 AI 自动化全栈开发实操工程：**Next.js + Go + OpenAPI + Codex 工作流**。

用任务看板跑通创建、列表、状态更新、删除和持久化；用“任务优先级”需求演示 Agent 从设计到验收的完整流程。默认选择 Go，Rust 迁移方案见架构文档。

## 5 分钟启动

前置：Node.js 22+（本次使用 24）、Go 1.25+、Python 3.9+。使用 AI 流水线时额外需要已登录的 Codex CLI；普通开发和测试不需要模型或 API Key。

```bash
cd /Users/guilinzhou/Desktop/chovy/agent/ai-fullstack-lab
npm ci
npm run dev
```

打开 http://127.0.0.1:3000 。后端默认 http://127.0.0.1:8080 ，数据保存在 `.var/tasks.json`。Ctrl+C 同时关闭两端。首次安装依赖和 Go 模块需要网络。

端口冲突时：`WEB_PORT=3200 API_PORT=8282 npm run dev`。`.env.example` 是变量说明；根目录 `.env` 不会被编排器自动加载，命令前设置环境变量即可。`npm run dev` 自动协调两端地址，单独启动 Next.js 时使用 `API_ORIGIN`。

## 先验证，再让 Agent 开发

```bash
npm run check
npx playwright install chromium
npm run test:e2e
python3 -m unittest discover -s tests/automation -v
```

`check` 包含契约漂移、TS、ESLint、Go race 测试、go vet、Next.js 构建；E2E 启动真实两端，在 3100 / 8181 端口和临时数据上测试。浏览器安装在 Linux CI 使用 `npx playwright install --with-deps chromium`。

## 让 AI 实际完成一个需求

先预览，不调用模型、不修改文件：

```bash
npm run agent:plan
```

确认需求文件后执行（使用本机 Codex 登录，会消耗模型额度并修改本工程）：

```bash
npm run agent:run
# 或替换自己的需求文件
python3 scripts/agent_flow.py --task specs/examples/task-priority.md --execute
```

执行前建议提交当前基线，或在独立分支/工作树运行。默认继承本机模型，支持 `--model YOUR_MODEL_ID`，无需把密钥写入仓库。每阶段默认 20 分钟超时；错误立即停止，所有阶段日志、设计与结果保存在 `.automation/runs/<run-id>/`。不会自动 commit、push、部署。

也可以在本目录启动 `codex`，输入：

```text
使用 $fullstack-feature，实现 specs/examples/task-priority.md 并完成验收。
```

仓库提供 3 个 Skill、5 个角色配置。CLI 串行编排直接加载角色指令，不依赖子 Agent 功能；交互式自定义角色取决于本机 Codex 版本与项目配置的信任设置。详见工作流文档。

## 导航

| 入口 | 用途 |
|---|---|
| [架构与选型](docs/architecture.md) | 代码结构、对接机制、Go / Rust、数据演进 |
| [Agent 工作流](docs/agent-workflow.md) | 阶段边界、角色、Skill、失败恢复 |
| [实操手册](docs/implementation.md) | 手工/API 实操、优先级需求、容器与 CI |
| [验证记录](docs/validation.md) | 实际运行结果与已知限制 |
| [API 契约](contracts/openapi.yaml) | 前后端唯一接口定义 |
| [项目规则](AGENTS.md) | Agent 持久指令与质量门禁 |

## 交付边界

这是本地开发与 AI 工程工作流基线；数据采用单后端进程 JSON 文件，未加入用户认证或租户隔离。多实例和业务上线前应按架构文档接入数据库、认证、备份与部署环境。完整 AI 模型调用需你显式运行，搭建期间仅验证编排器的预览与失败控制，不代替真实模型端到端执行。
