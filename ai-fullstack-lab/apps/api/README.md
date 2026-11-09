# Go 任务 API 子工程

基于Go net/http与OpenAPI生成接口实现的任务服务，使用JSON文件持久化，入口为 [cmd/server/main.go](cmd/server/main.go)。完整开发和契约流程见[工程指南](../../README.md)。

## 目录与接口维护

`internal/tasks/handler.go` 负责业务HTTP处理，store.go负责单进程存储，handler_test.go覆盖服务行为。`internal/api/generated.go` 来自根contracts/openapi.yaml，禁止直接编辑生成物。

API变更先修改根契约，再从工程根执行npm run generate并对齐严格接口实现。服务只支持单进程管理自己的JSON数据文件，不适合多个实例共享同一文件写入。

## 单独运行与检查

在当前目录执行：

```bash
go run ./cmd/server
go test -race ./...
go vet ./...
```

默认监听127.0.0.1:8080，支持ADDR和DATA_FILE；单独运行时相对数据路径基于进程工作目录，需要与前端联调数据一致时显式设置绝对文件路径。统一启动两端优先在工程根运行npm run dev。

2026-11-09已通过Go race测试、go vet与真实HTTP/浏览器联调；存储与协议的最新验证见[验证记录](../../docs/validation.md)。测试使用独立临时数据，不连接生产数据库。

## 存储启动校验

Open加载快照时校验任务ID、标题、状态与创建时间，拒绝重复ID或无效任务；失败不会自动修复或覆盖用户文件。合法历史任务与空快照保持兼容。存储回归额外覆盖创建、更新、删除在rename阶段失败后的内存/磁盘回滚与临时文件清理。
