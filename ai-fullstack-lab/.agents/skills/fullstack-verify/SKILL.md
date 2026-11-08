---
name: fullstack-verify
description: 验证本工程契约、后端、前端和真实 API 联调，定位并报告质量门禁失败。适用于交付验收和跨端回归。
---

读取需求验收条件和实际 diff，确认测试覆盖业务行为。
执行 npm run check；检查契约漂移、Go race 测试、vet、TypeScript、ESLint 和构建结果。
执行 npm run test:e2e；Playwright 使用独立端口 3100/8181 和临时存储，不连接用户开发数据。
失败时依据日志和 trace 定位；不删除断言、不换成 mock 成功响应来通过真实联调用例。
报告运行命令、通过/失败、关键文件行号与阻断项。没有工具或网络时记录未验证内容。
