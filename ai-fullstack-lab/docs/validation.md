# 验证记录

日期：2026-11-08。环境：macOS arm64、Node 24.14.1、npm 11.11.0、Go 1.25.11、Python 3.9.6、Codex CLI 0.167.0。

## 已实际通过

| 检查 | 结果 |
|---|---|
| npm run check | 通过：契约一致、TypeScript、ESLint、Go race 测试、go vet、Next.js 生产构建 |
| npm run test:e2e | 2 条通过；真实浏览器 → Next rewrite → Go → 临时持久化文件 |
| Python 编排器测试 | 7 项通过：预览无副作用、串行成功、只读审查、门禁阻断、Agent blocked、并发锁、格式检查及超时终止 |
| npm run agent:plan | 通过；输出完整阶段，不调用模型、不写执行产物 |
| Skill Creator quick_validate.py | 3 个 Skill 均通过；校验器依赖安装在项目 .var/skill-check 虚拟环境 |
| YAML / TOML / JS 语法 | 通过；包括 OpenAPI、Compose、CI、自定义角色、Skill 元数据和脚本 |
| 契约漂移探针 | 故意修改生成类型时正确失败；确认检查不覆盖 Go 或 TS 源文件，探针恢复 |
| npm 安装审计 | 局部依赖覆盖后为 0 vulnerabilities；这是本次安装时的审计结果 |
| 进程清理 | E2E gracefulShutdown 结束后，确认测试 API / Next / 开发编排进程均已退出 |

Go 测试覆盖正常 CRUD、重新打开存储、非法字段、缺字段、非法状态、标题边界、存储失败不改内存、并发创建、超大请求、不支持的 Content-Type；HTTP 用例同时依据 OpenAPI 验证响应。

浏览器第一条用例调用真实后端，完成创建、doing、刷新、done、删除，并验证非法请求。第二条仅拦截请求返回 503，用于验证错误提示，未以 mock 代替真实联调用例。

## 验证中修正的问题

- 健康检查响应从内联枚举改为命名 Health Schema，避免生成器漏掉枚举类型。
- 移除生成器配置中的固定 output，让脚本明确控制目标，契约检查真正只生成到临时目录。
- 将请求格式错误统一映射到契约声明的 400 JSON 响应。
- 错误提示断言限定到任务工作区，避免匹配 Next.js 自带 route announcer。
- Playwright 退出时先发送 SIGTERM，使开发脚本有机会清理分离进程组。
- 修复开发工具间接依赖 minimatch 的已知问题；覆盖理由见架构文档。

## 未验证与使用边界

没有执行真实模型开发：编排器控制流程通过测试，CLI 参数已按本机帮助核对；模型认证、额度、生成质量与项目配置的信任状态需要运行 --execute 后验证。不会声称本次已完成 AI 开发整条模型链路。

没有 Docker 工具，镜像与 Compose 未执行构建或启动。未发布到任何环境。JSON 存储适合单进程本地示例，没有登录和多租户能力。

CI 文件位于用户要求的新子目录中；作为独立仓库根目录时生效。保留父仓库结构时需按实施文档接入父仓库的 workflow 目录。

Next.js dev 自动生成 apps/web/AGENTS.md 和更新 next-env.d.ts；保留框架生成的指导与类型入口。所有运行日志、数据与校验虚拟环境均被 Git 忽略。
