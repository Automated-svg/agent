# AI Agent 实操工作流

## 怎么使用

交互式适合边做边澄清：在项目根目录打开 Codex，使用 `$fullstack-feature` 加需求。自动流水线适合需求明确：先 `npm run agent:plan` 检查阶段，再 `npm run agent:run` 执行示例。

编排器是本地 Python 程序，逐阶段调用真实 `codex exec`；不是写一个“假定 Agent 已实现”的演示。搭建时不主动调用付费模型；开发者显式运行 `--execute` 后才发起模型会话。

## 阶段与产物

| 阶段 | 输入 | 允许修改 | 成功证据 |
|---|---|---|---|
| architect | 需求、架构、现有代码 | 本次 run 的 design.md | 可测试验收条件、文件清单、兼容策略 |
| contract | design.md | OpenAPI、生成物、生成配置 | generate + contracts:check |
| backend | 设计、生成 Go 接口 | apps/api | Go race 测试、HTTP 响应契约 |
| frontend | 设计、生成 TS 类型 | apps/web、tests/e2e | 交互、类型与 lint、新 E2E |
| verify | 已实现代码 | 检查过程的构建/日志产物 | check + test:e2e 都退出 0 |
| reviewer | 代码、需求、设计、verify.log | 只读 | 无阻断问题才返回 completed |

当前串行实现中，backend 结束才进入 frontend；契约确认后两端本可并行，但串行更易复现，也避免生成文件与跨目录冲突。交互式需要并行时由用户明确要求，按 AGENTS.md 的目录所有权分配。

每个模型阶段输出相同 JSON：

```json
{"status":"completed","summary":"本阶段完成内容与验证证据","risks":[]}
```

结果必须通过本地结构检查；`blocked`、非零退出、缺少 design.md、结果 JSON 无效、超时或任何门禁失败均终止，不继续运行后续阶段。机器检查的退出码与日志是门禁，模型一句“通过”不能代替。一个阶段的 completed 也不表示可以发布。

## 日志与停止条件

`.automation/runs/<run-id>/` 保存 request.md、design.md、各阶段结果、事件、stderr、verify.log 和 manifest.json。manifest 逐阶段更新 pending → in_progress → completed，错误为 blocked。日志可能包含代码和业务内容，默认 Git 忽略。

每阶段默认 1200 秒，`--timeout 600` 可调整；超时或 Ctrl+C 结束该阶段整个进程组。每次执行用锁防止并发编排。进程强制终止后可能留下 `.automation/flow.lock`，确认对应进程已退出后再删除残留锁。

失败恢复：阅读 manifest 和对应日志 → 检查 diff → 修复环境或需求歧义 → 验证已有改动 → 重新执行完整流水线。当前不实现从阶段中途 resume、不自动重试模型、不自动撤销代码；防止把部分完成代码误当作无更改基线。重跑前应让需求明确描述已实现部分。

## Skill 与 Agent 是什么关系

- `AGENTS.md`：项目常驻规则与检查入口。
- `.agents/skills/fullstack-feature/`：跨端开发流程，触发于功能或跨端修复。
- `.agents/skills/go-backend-development/`：Go 接口、业务逻辑、持久化与后端测试，供独立后端任务和 backend 阶段复用。
- `.agents/skills/api-contract-sync/`：接口契约与生成代码同步。
- `.agents/skills/fullstack-verify/`：运行校验并诊断失败。
- `.automation/roles/*.md`：阶段指令的唯一职责来源，CLI 编排直接读取。
- `.codex/agents/*.toml`：交互式自定义 Agent 入口，指向同一阶段指令。

项目配置不固定模型，也不修改全局 Codex 配置或认证。CLI 流水线不要求发现自定义 Agent，也不递归调用子 Agent。自定义 Agent 配置按当前文档使用独立 TOML 文件；较旧 CLI 如果不支持，仍可使用串行编排与 Skill。

## 实际权限边界

编排器为实现阶段传入 workspace-write，reviewer 传入 read-only，保留本机 Codex 授权机制；不会使用绕过沙箱选项。网络或命令被环境阻止时，该阶段应报告 blocked，不能偷偷扩大权限。自动化环境应事先准备依赖与浏览器。

角色目录边界是指令约束，不是独立文件系统沙箱。尤其当前目录位于父 Git 仓库内时，Codex 实际可写范围可能包含父仓库；需要强隔离时把子目录导出为独立仓库，或在容器/专用工作树运行。不要把它当作可以安全执行不可信 Issue 的沙箱。

编排器不会自动 commit、push、发消息或发布；完成后开发者检查 diff 和测试证据。普通 CI 只跑确定性检查，不接收模型密钥，也不让 PR 内容自动驱动模型改代码。

## 官方依据

[项目 Skills 发现目录](https://learn.chatgpt.com/docs/build-skills)、[自定义 Agent TOML](https://learn.chatgpt.com/docs/agent-configuration/subagents)、[非交互模式与结构化结果](https://learn.chatgpt.com/docs/non-interactive-mode)。本机 Codex 0.167.0 的 exec 帮助已确认支持 --sandbox、--output-schema、--output-last-message 和 --json；模型与认证状态由执行环境决定。

## 统一门禁的检查范围

npm run check现在先执行test:tooling与test:automation，再运行原有契约一致性、TypeScript、ESLint、Go race/vet和生产构建。编排器的verify阶段继续调用同一个check与test:e2e，因此脚本与编排控制回归也成为每次交付的确定性检查；测试使用隔离目录和模拟阶段，不会在门禁里启动真实模型流水线。
