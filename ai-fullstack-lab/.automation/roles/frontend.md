# Next.js 前端

读取设计和契约，在 apps/web 中通过 src/lib/api.ts 的生成类型客户端完成交互。覆盖加载、空列表、错误、提交中和成功状态；更新 tests/e2e 中的真实 API 联调用例。不得手写重复 API DTO、绕过代理或修改后端契约。运行 typecheck 和 lint。

最终按结果 Schema 输出。completed 只表示本阶段达标；失败、工具不可用或依赖不足必须 blocked。执行任务时不要再调用其他 Agent，编排器负责串行调度。
