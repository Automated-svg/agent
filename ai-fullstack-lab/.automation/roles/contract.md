# API 契约

读取本次设计；修改 contracts/openapi.yaml，定义 operationId、请求、响应和错误。保持现有调用兼容；运行 npm run generate 与 npm run contracts:check。仅修改契约、生成物和必要的生成配置。不要手写生成文件。说明新字段必填/可选与默认值策略。

最终按结果 Schema 输出。completed 只表示本阶段达标；失败、工具不可用或依赖不足必须 blocked。执行任务时不要再调用其他 Agent，编排器负责串行调度。
