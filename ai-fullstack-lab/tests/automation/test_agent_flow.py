import importlib.util
import io
import json
from pathlib import Path
import tempfile
import subprocess
import sys
import unittest
from unittest.mock import patch

MODULE_PATH = Path(__file__).resolve().parents[2] / "scripts/agent_flow.py"
spec = importlib.util.spec_from_file_location("agent_flow", MODULE_PATH)
flow = importlib.util.module_from_spec(spec)
spec.loader.exec_module(flow)


class AgentFlowTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name).resolve()
        (self.root / ".automation/roles").mkdir(parents=True)
        for stage in flow.STAGES:
            (self.root / ".automation/roles" / (stage + ".md")).write_text(stage)
        (self.root / "task.md").write_text("实现功能并验证")
        self.root_patch = patch.object(flow, "ROOT", self.root)
        self.root_patch.start()
        self.addCleanup(self.root_patch.stop)

    def execute(self, fake, *extra):
        with patch("sys.argv", ["agent_flow", "--task", str(self.root / "task.md"), *extra]), patch.object(flow, "run_process", side_effect=fake), patch("sys.stdout", new_callable=io.StringIO), patch("sys.stderr", new_callable=io.StringIO):
            return flow.main()

    def fake_stage(self, command, **kwargs):
        if command[0] == "codex":
            output = Path(command[command.index("--output-last-message") + 1])
            output.write_text(json.dumps({"status":"completed", "summary":"stage complete", "risks":[]}))
            if output.stem == "architect":
                (output.parent / "design.md").write_text("acceptance criteria")
        return 0

    def manifest(self):
        return json.loads(next((self.root / ".automation/runs").glob("*/manifest.json")).read_text())

    def test_preview_does_not_call_model_or_write(self):
        def fail(*args, **kwargs):
            self.fail("preview executed subprocess")
        self.assertEqual(self.execute(fail), 0)
        self.assertFalse((self.root / ".automation/runs").exists())

    def test_success_runs_checks_and_readonly_review(self):
        calls = []
        def fake(command, **kwargs):
            calls.append(command)
            return self.fake_stage(command, **kwargs)
        self.assertEqual(self.execute(fake, "--execute"), 0)
        self.assertEqual(self.manifest()["status"], "completed")
        self.assertIn(["npm", "run", "check"], calls)
        self.assertIn(["npm", "run", "test:e2e"], calls)
        self.assertEqual(calls[-1][calls[-1].index("--sandbox") + 1], "read-only")
        self.assertFalse((self.root / ".automation/flow.lock").exists())

    def test_failed_quality_gate_stops_before_review(self):
        stages = []
        def fake(command, **kwargs):
            if command[0] == "npm":
                return 1
            stages.append(Path(command[command.index("--output-last-message") + 1]).stem)
            return self.fake_stage(command, **kwargs)
        self.assertEqual(self.execute(fake, "--execute"), 1)
        self.assertNotIn("reviewer", stages)
        self.assertEqual(self.manifest()["stages"]["verify"], "blocked")
        self.assertFalse((self.root / ".automation/flow.lock").exists())

    def test_blocked_model_stops_pipeline(self):
        def fake(command, **kwargs):
            output = Path(command[command.index("--output-last-message") + 1])
            output.write_text(json.dumps({"status":"blocked", "summary":"missing requirement", "risks":[]}))
            return 0
        self.assertEqual(self.execute(fake, "--execute"), 1)
        self.assertEqual(self.manifest()["stages"]["contract"], "pending")

    def test_existing_lock_is_preserved(self):
        lock = self.root / ".automation/flow.lock"
        lock.write_text("another process")
        def fail(*args, **kwargs):
            self.fail("concurrent runner invoked subprocess")
        self.assertEqual(self.execute(fail, "--execute"), 1)
        self.assertEqual(lock.read_text(), "another process")

    def test_timeout_terminates_hung_process(self):
        with tempfile.TemporaryFile(mode="w+") as log:
            with self.assertRaises(subprocess.TimeoutExpired):
                flow.run_process([sys.executable, "-c", "import time; time.sleep(30)"],
                                 timeout=0.1, stdout=log, stderr=log)

    def test_malformed_result_is_rejected(self):
        for result in ({}, {"status":"completed", "summary":"ok", "risks":"none"}, {"status":"invalid", "summary":"ok", "risks":[]}):
            with self.assertRaises(ValueError):
                flow.validate_result(result)


if __name__ == "__main__":
    unittest.main()
