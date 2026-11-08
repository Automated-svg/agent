---
name: api-contract-sync
description: 同步本工程 OpenAPI 契约、Go 服务端和 TypeScript 客户端类型。适用于接口新增、字段变更与契约漂移修复。
---

读取 contracts/openapi.yaml、apps/api/oapi-codegen.yaml、scripts/contracts.mjs。
先检查调用方与旧持久化数据；定义字段默认值、可选性、错误码与兼容策略。
只修改 OpenAPI 源文件；执行 npm run generate；不要手工编辑 generated.go / api.generated.d.ts。
在 Go 中实现生成接口，在 Next.js 中使用类型客户端。生成类型不代替运行时验证。
执行 npm run contracts:check、npm run test:api 和前端 typecheck；跨端行为修改时追加 E2E。
如果只同步契约而不实现接口，明确列出待实现项，不声称功能已交付。
