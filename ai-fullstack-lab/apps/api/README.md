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

本轮只核对入口、Go模块和工程约定，未执行数据库写入、Go测试或服务启动。此前验收结果见根docs/validation.md。
