import type { Metadata } from "next";
import "./globals.css";
export const metadata: Metadata = { title: "Agent Lab · 全栈任务看板", description: "契约驱动的 AI 全栈开发实操" };
export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return <html lang="zh-CN"><body>{children}</body></html>;
}
