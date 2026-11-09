"use client";

import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import Link from "next/link";
import { api, type Task, type TaskStatus } from "@/lib/api";

const columns: { status: TaskStatus; label: string; number: string }[] = [
  { status: "todo", label: "待开始", number: "01" },
  { status: "doing", label: "进行中", number: "02" },
  { status: "done", label: "已完成", number: "03" },
];

export default function Home() {
  const [tasks, setTasks] = useState<Task[]>([]);
  const [title, setTitle] = useState("");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState("");

  const writing = useRef(false);
  const loadVersion = useRef(0);

  const load = useCallback((signal?: AbortSignal) => {
    const version = ++loadVersion.current;
    return api.GET("/api/v1/tasks", { signal }).then(({ data, error }) => {
      if (signal?.aborted || version !== loadVersion.current) return;
      if (error || !data) throw new Error("无法加载任务，请检查 API 服务");
      setTasks(data);
      setLoaded(true);
      setError("");
    }).catch((cause: unknown) => {
      if (!signal?.aborted && version === loadVersion.current) {
        setError(cause instanceof Error && cause.message === "无法加载任务，请检查 API 服务"
          ? cause.message : "连接失败，请稍后重试");
      }
    }).finally(() => {
      if (!signal?.aborted && version === loadVersion.current) setLoading(false);
    });
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    void load(controller.signal);
    return () => { controller.abort(); };
  }, [load]);

  function retry() {
    if (busy || loading) return;
    setLoading(true);
    setError("");
    void load();
  }

  async function mutate(action: () => Promise<void>) {
    if (writing.current || loading || !loaded) return;
    writing.current = true;
    setBusy(true);
    setError("");
    try { await action(); }
    catch (cause) { setError(cause instanceof Error ? cause.message : "操作失败，请稍后重试"); }
    finally { writing.current = false; setBusy(false); }
  }

  async function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!title.trim()) return;
    await mutate(async () => {
      const { data, error } = await api.POST("/api/v1/tasks", { body: { title: title.trim() } });
      if (error || !data) throw new Error(error?.message ?? "无法保存任务，请稍后重试");
      setTasks(current => [...current, data]);
      setTitle("");
    });
  }

  async function update(task: Task, status: TaskStatus) {
    await mutate(async () => {
      const { data, error } = await api.PATCH("/api/v1/tasks/{id}", { params: { path: { id: task.id } }, body: { status } });
      if (error || !data) throw new Error(error?.message ?? "无法更新任务，请稍后重试");
      setTasks(current => current.map(item => item.id === data.id ? data : item));
    });
  }

  async function remove(task: Task) {
    await mutate(async () => {
      const { error } = await api.DELETE("/api/v1/tasks/{id}", { params: { path: { id: task.id } } });
      if (error) throw new Error(error.message);
      setTasks(current => current.filter(item => item.id !== task.id));
    });
  }

  const interactionDisabled = busy || loading || !loaded;

  return <main>
    <nav><Link href="/" className="brand"><span className="brand-icon">a.</span> AGENT LAB</Link><span className="nav-label">WORKSPACE / 001</span></nav>
    <header><div className="eyebrow"><span className="dot" /> FROM IDEA TO SHIPPED</div>
      <h1>把想法，<br /><span>推进到完成。</span></h1>
      <p>每一个小任务，都是下一次交付的起点。</p>
      <div className="metrics"><strong>{tasks.length}<small>全部任务</small></strong><strong>{tasks.filter(t => t.status === "done").length}<small>已完成</small></strong></div>
    </header>
    <section className="workspace" aria-label="任务工作区" aria-busy={loading || busy}>
      <div className="section-heading"><h2>项目任务 <span>PROJECT BOARD</span></h2><span className="live"><span className="dot" /> 本地工作区</span></div>
      <form onSubmit={create}><label className="sr-only" htmlFor="title">任务标题</label>
        <span className="plus" aria-hidden="true">＋</span><input id="title" placeholder="下一步，你想完成什么？" value={title} maxLength={120} disabled={interactionDisabled} onChange={e => setTitle(e.target.value)} required />
        <button type="submit" disabled={interactionDisabled || !title.trim()}>添加任务 <span aria-hidden="true">↗</span></button>
      </form>
      {error && <div className="error"><p role="alert">{error}</p><button type="button" className="retry" disabled={busy || loading} onClick={retry}>重新加载任务</button></div>}
      {loading ? <p role="status" className="loading">正在加载工作区…</p> : <div className="board">
        {columns.map(column => <section className={`column ${column.status}`} key={column.status} aria-label={column.label}>
          <h3><span className="column-number">{column.number}</span>{column.label}<span className="count">{tasks.filter(t => t.status === column.status).length}</span></h3>
          <div className="cards">{tasks.filter(t => t.status === column.status).map(task => <article key={task.id}>
            <div className="card-top"><span className="tag">TASK</span><button className="delete" disabled={interactionDisabled} onClick={() => remove(task)} aria-label={`删除 ${task.title}`}>×</button></div>
            <h4>{task.title}</h4><div className="card-bottom"><time dateTime={task.createdAt}>{new Date(task.createdAt).toLocaleDateString("zh-CN")}</time>
              <select aria-label={`${task.title} 状态`} value={task.status} disabled={interactionDisabled} onChange={e => update(task, e.target.value as TaskStatus)}>{columns.map(c => <option key={c.status} value={c.status}>{c.label}</option>)}</select>
            </div>
          </article>)}{!tasks.some(t => t.status === column.status) && <div className="empty"><span aria-hidden="true">{column.status === "done" ? "✓" : "○"}</span><p>{column.status === "done" ? "期待你的第一份成果" : "给下一步留一点空间"}</p></div>}</div>
        </section>)}
      </div>}
    </section><footer><span>小步前进，持续交付。</span><span>AGENT LAB © {new Date().getFullYear()}</span></footer>
  </main>;
}
