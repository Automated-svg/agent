---
name: fullstack-feature
description: 实现本工程跨前后端需求，协调契约、Go、Next.js 和自动化验收。适用于功能开发和跨端缺陷修复。
---

读取 AGENTS.md、docs/architecture.md、docs/agent-workflow.md 和用户需求。
将验收条件写清楚，识别会影响契约、旧数据和前端状态的变更。
涉及 API 时使用 $api-contract-sync 的流程先完成契约和生成物，再按依赖实现后端与前端。
前端复用 src/lib/api.ts；后端复用现有 StrictServerInterface 和 Store，不增加无需求的基础设施。
为新增行为增加合适的后端/端到端验证，执行 npm run check 与 npm run test:e2e。
遵循用户既有授权；仅当用户要求执行整条编排时运行 scripts/agent_flow.py --execute，不在当前流程内重复启动自身。
交付实际变更、验证结果、文件位置与未解决问题。
