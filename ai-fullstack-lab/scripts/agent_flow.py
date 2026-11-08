#!/usr/bin/env python3
"""串行 Codex 开发流水线：默认预览；--execute 才调用模型和修改项目。"""
import argparse
import datetime
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import uuid

ROOT = Path(__file__).resolve().parents[1]
STAGES = ["architect", "contract", "backend", "frontend", "verify", "reviewer"]
SCHEMA = ROOT / ".automation/result.schema.json"


def run_process(command, *, timeout, stdout, stderr, input_text=None):
    process = subprocess.Popen(command, cwd=ROOT, stdin=subprocess.PIPE if input_text else subprocess.DEVNULL,
                               stdout=stdout, stderr=stderr, text=True, start_new_session=True)
    try:
        process.communicate(input=input_text, timeout=timeout)
    except (subprocess.TimeoutExpired, KeyboardInterrupt):
        os.killpg(process.pid, signal.SIGTERM)
        try:
            process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            os.killpg(process.pid, signal.SIGKILL)
            process.wait()
        raise
    return process.returncode


def validate_result(result):
    if not isinstance(result, dict) or set(result) != {"status", "summary", "risks"}:
        raise ValueError("Agent 结果字段不符合 Schema")
    if result["status"] not in ("completed", "blocked") or not isinstance(result["summary"], str):
        raise ValueError("Agent status/summary 无效")
    if not isinstance(result["risks"], list) or not all(isinstance(risk, str) for risk in result["risks"]):
        raise ValueError("Agent risks 无效")
    if result["status"] != "completed":
        raise RuntimeError("Agent blocked: " + result["summary"])


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--task", required=True, type=Path)
    parser.add_argument("--execute", action="store_true", help="执行模型调用、修改代码并运行校验")
    parser.add_argument("--model", help="可选；默认继承本机配置")
    parser.add_argument("--timeout", type=int, default=1200, help="每阶段超时秒数")
    args = parser.parse_args()
    task_path = args.task.resolve()
    try:
        task_path.relative_to(ROOT)
        task = task_path.read_text()
    except (ValueError, OSError) as error:
        parser.error("需求文件必须位于项目中且可读取：" + str(error))
    if not task.strip() or args.timeout <= 0:
        parser.error("需求不能为空，timeout 必须大于零")
    if not args.execute:
        print(json.dumps({"mode": "preview", "task": str(task_path.relative_to(ROOT)), "stages": STAGES,
                          "checks": ["npm run check", "npm run test:e2e"],
                          "note": "未调用模型、未写入文件；加 --execute 执行。"}, ensure_ascii=False, indent=2))
        return 0
    stamp = datetime.datetime.now(datetime.timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    run_dir = ROOT / ".automation/runs" / (stamp + "-" + uuid.uuid4().hex[:8])
    run_dir.mkdir(parents=True)
    (run_dir / "request.md").write_text(task)
    state = {"status": "running", "task": str(task_path.relative_to(ROOT)),
             "stages": {stage: "pending" for stage in STAGES}}
    def save():
        (run_dir / "manifest.json").write_text(json.dumps(state, ensure_ascii=False, indent=2))
    save()
    stage = STAGES[0]
    lock = ROOT / ".automation/flow.lock"
    lock_fd = None
    try:
        # 防止两个编排器同时修改同一项目。进程崩溃后的残留锁需人工检查后移除。
        lock_fd = os.open(lock, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)
        os.write(lock_fd, str(os.getpid()).encode())
        summaries = []
        for stage in STAGES:
            state["stages"][stage] = "in_progress"
            save()
            print("执行阶段:", stage, flush=True)
            if stage == "verify":
                with (run_dir / "verify.log").open("w") as log:
                    for command in (["npm", "run", "check"], ["npm", "run", "test:e2e"]):
                        log.write("$ " + " ".join(command) + "\n"); log.flush()
                        code = run_process(command, timeout=args.timeout, stdout=log, stderr=subprocess.STDOUT)
                        if code != 0:
                            raise RuntimeError("质量门禁失败，见 verify.log；保留现场，不自动反复修改")
            else:
                role = (ROOT / ".automation/roles" / (stage + ".md")).read_text()
                output = run_dir / (stage + ".json")
                prompt = ("在当前项目完成以下阶段。读取 AGENTS.md。仅在当前子项目内工作。\n" + role +
                          "\n需求：\n" + task + "\n本次产物目录：" + str(run_dir.relative_to(ROOT)) +
                          "\n前序摘要：\n" + "\n".join(summaries) +
                          "\n读取此目录中的 design.md（architect 阶段由你创建），以及已存在的 verify.log。" +
                          "\n不要更改编排脚本、角色指令、检查命令或 Schema 来使任务通过；不要 git commit、push 或部署。")
                command = ["codex", "exec", "--cd", str(ROOT), "--sandbox",
                           "read-only" if stage == "reviewer" else "workspace-write",
                           "--output-schema", str(SCHEMA), "--output-last-message", str(output), "--json"]
                if args.model:
                    command += ["--model", args.model]
                command += ["-"]
                with (run_dir / (stage + ".events.jsonl")).open("w") as events, (run_dir / (stage + ".stderr.log")).open("w") as errors:
                    code = run_process(command, timeout=args.timeout, stdout=events, stderr=errors, input_text=prompt)
                if code != 0:
                    raise RuntimeError("Codex 退出码 " + str(code) + "；检查该阶段日志")
                result = json.loads(output.read_text())
                validate_result(result)
                if stage == "architect" and not (run_dir / "design.md").is_file():
                    raise RuntimeError("缺少 design.md")
                summaries.append(stage + ": " + result["summary"])
            state["stages"][stage] = "completed"
            save()
        state["status"] = "completed"
        save()
        print("已完成；请审查 diff 和日志，再决定是否提交。", run_dir)
        return 0
    except (Exception, KeyboardInterrupt) as error:
        state["status"] = "blocked"
        state["stages"][stage] = "blocked"
        state["error"] = str(error) or "用户中断"
        save()
        print("已停止：", state["error"], "产物：", run_dir, file=sys.stderr)
        return 1
    finally:
        if lock_fd is not None:
            os.close(lock_fd)
            lock.unlink(missing_ok=True)


if __name__ == "__main__":
    sys.exit(main())
