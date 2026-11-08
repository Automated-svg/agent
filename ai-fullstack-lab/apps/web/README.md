# Next.js 任务看板子工程

使用Next.js App Router、React与TypeScript的任务界面。页面位于src/app，类型化API调用封装在 [src/lib/api.ts](src/lib/api.ts)，通过同源/api/v1代理访问Go服务。

[完整工程指南](../../README.md) · [本目录开发规则](AGENTS.md)

## 工作区使用

依赖由上级工程根的npm workspace管理，在工程根执行npm ci。单独开发本应用可从工程根执行：

```bash
npm run dev -w apps/web
npm run typecheck -w apps/web
npm run lint -w apps/web
npm run build -w apps/web
```

单独启动页面不会自动启动Go后端。API_ORIGIN需指向可用服务，默认127.0.0.1:8080。统一联调优先使用根npm run dev，自动协调两端端口。

## 类型与修改约定

src/lib/api.generated.d.ts由根OpenAPI契约生成，不能手改。新增业务需先按根规则更新契约与生成物，再实现页面和调用；Next.js相关实现遵守本目录AGENTS.md及实际安装版本的本地文档。

本轮仅核对README、package.json与目录入口，没有修改页面、安装依赖或运行Next.js。质量门禁与Playwright联调从工程根执行，结果参考原验证记录。
