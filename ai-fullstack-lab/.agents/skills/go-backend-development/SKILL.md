---
name: go-backend-development
description: 在 AI Fullstack Lab 中开发和修复 Go 后端接口、业务逻辑、文件持久化及后端测试。适用于 apps/api 的后端任务；Rust 实现和前端页面不在此 Skill 范围内。
---

# Go 后端开发

从项目根目录执行命令。先读 `AGENTS.md`、用户需求及本次设计，再根据变更读取以下入口：

| 入口 | 职责 |
|---|---|
| `contracts/openapi.yaml` | 请求、响应、状态码和字段约束 |
| `apps/api/internal/api/generated.go` | 生成的类型、路由与 `StrictServerInterface`，不手工修改 |
| `apps/api/internal/tasks/handler.go` | 严格接口实现、业务校验与错误映射 |
| `apps/api/internal/tasks/store.go` | 文件存储、并发保护与持久化 |
| `apps/api/internal/tasks/handler_test.go` | HTTP 行为、响应契约及存储测试 |
| `apps/api/cmd/server/main.go` | 配置、服务超时、启动与关闭 |

## 契约与任务边界

先判断现有契约是否能表达需求。仅修改内部实现时保持 API 不变。

独立开发任务中，接口变更应在用户授权范围内按 `$api-contract-sync` 完成：先改契约，再运行 `npm run generate`，再实现生成接口。不得通过手写 DTO 或路由绕过生成流程。

由流水线 `backend` 阶段调用时，契约归 `contract` 阶段所有。若设计与契约不匹配，说明差异并返回 `blocked`，不要自行修改契约或生成物。需要前端适配的事项交给主流程，不扩展后端阶段的目录所有权。

## 业务实现

沿用现有 `net/http`、严格接口和任务模块结构。处理器负责 HTTP 业务规则与错误映射，Store 负责数据读写；仅在业务复杂度确有需要时拆出服务或存储接口。

OpenAPI 校验处理字段类型、必填项及枚举；处理器补充规范无法表达的业务规则，例如去除标题空白后的非空检查。返回契约声明的状态码和 JSON 错误结构，不向客户端暴露文件路径或内部错误细节。

新增字段时分别考虑旧请求和旧文件：可选字段的默认值、历史 JSON 缺字段时的读取行为、更新请求是否覆盖旧值。不要把“能够反序列化”当作已经完成旧数据兼容。

## 持久化约束

- 保持“复制快照 → 写临时文件 → sync / close → rename → 替换内存”的顺序；写入失败不能污染内存状态。
- 读操作返回独立结果；若新增 map、slice 或指针字段，评估现有浅复制是否仍然足够。
- 保留并发锁，避免读取与写入竞争；该实现只支持单进程，不把 mutex 描述成跨进程锁。
- 存储损坏或读取失败应明确返回错误，不静默清空数据。测试使用 `t.TempDir()`，不读写开发数据文件。

仅当用户要求接入数据库时引入数据库依赖、迁移和相应集成测试；否则继续复用文件存储。

## 验证与交接

对本次修改的 Go 文件运行 `gofmt`。增加能验证新增行为的测试，按变更选择正常请求、非法输入、未找到、旧数据恢复、写入失败或并发场景；复用现有响应契约校验辅助方法。

```bash
npm run contracts:check
npm run test:api
```

`test:api` 已包含 `go test -race ./...` 和 `go vet ./...`。跨端需求由主流程继续执行 `npm run check` 与 `npm run test:e2e`；不要声称只通过后端测试就已完成前后端联调。

交付实现文件、API / 数据兼容说明、实际检查结果和待交接事项。工具或依赖不足时标明未验证内容；流水线阶段按 `.automation/result.schema.json` 返回结果，不自行启动其他 Agent 或重新运行整条流水线。
