"""Hermetic initialization CLI checks; RPC calls are mocked, no DB is touched."""
import os
import pathlib
import shutil
import subprocess
import tempfile
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[2]
BASH = os.environ.get("NEWBEE_TEST_BASH") or shutil.which("bash")


class InitDatabasesTest(unittest.TestCase):
    def setUp(self):
        task_logs = ROOT / "logs" / "clone-deploy" / "tests"
        task_logs.mkdir(parents=True, exist_ok=True)
        self.directory = tempfile.TemporaryDirectory(dir=task_logs)
        self.addCleanup(self.directory.cleanup)
        self.tmp = pathlib.Path(self.directory.name)
        self.mock = self.tmp / "grpcurl"
        self.mock.write_text(
            '#!/usr/bin/env bash\n'
            'printf "%s\\n" "$*" >> "$NEWBEE_TEST_CALLS"\n'
            '[[ "$*" != *"${NEWBEE_TEST_FAILURE:-__never__}"* ]] || exit 1\n'
            'echo "{}"\n', encoding="utf-8", newline="\n"
        )
        self.mock.chmod(0o755)
        self.calls = self.tmp / "calls"
        self.env = {**os.environ, "GRPCURL": self.mock.as_posix(),
                    "NEWBEE_TEST_CALLS": self.calls.as_posix()}

    def run_cli(self, *args, **env):
        return subprocess.run([BASH, ROOT.joinpath("init-databases.sh").as_posix(), *args],
                              env={**self.env, **env}, capture_output=True, text=True,
                              encoding="utf-8", cwd=ROOT, timeout=10)

    def test_all_services_finish_and_use_explicit_proto(self):
        result = self.run_cli()
        self.assertEqual(result.returncode, 0, result.stderr)
        calls = self.calls.read_text().splitlines()
        self.assertEqual(len(calls), 5)
        self.assertTrue(all("-proto " in call for call in calls))
        self.assertIn("总计=5 成功=5 失败=0", result.stdout)
        self.assertIn("job.Job/initDatabase", calls[-1])

    def test_multiple_service_selection_keeps_dependency_order(self):
        result = self.run_cli("-s", "cmdb", "-s", "core", "-s", "cmdb")
        self.assertEqual(result.returncode, 0, result.stderr)
        calls = self.calls.read_text().splitlines()
        self.assertEqual(len(calls), 2)
        self.assertIn("core.Core/initDatabase", calls[0])
        self.assertIn("cmdb.Cmdb/initDatabase", calls[1])

    def test_failed_rpc_is_not_counted_as_skipped(self):
        result = self.run_cli(NEWBEE_TEST_FAILURE="cmdb.Cmdb/")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("总计=5 成功=4 失败=1", result.stdout)

    def test_core_failure_stops_dependent_initialization(self):
        result = self.run_cli(NEWBEE_TEST_FAILURE="core.Core/")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(len(self.calls.read_text().splitlines()), 1)

    def test_dry_run_does_not_call_grpcurl(self):
        result = self.run_cli("--dry-run", "-s", "core", GRPCURL="missing-grpcurl")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertFalse(self.calls.exists())
        self.assertIn("core.Core/initDatabase", result.stdout)

    def test_custom_endpoint_and_timeout(self):
        result = self.run_cli("-s", "job", NEWBEE_JOB_RPC="127.0.0.1:19105",
                              NEWBEE_INIT_TIMEOUT="120")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("-max-time 120", self.calls.read_text())
        self.assertIn("127.0.0.1:19105", self.calls.read_text())

    def test_invalid_input_is_rejected_without_rpc_calls(self):
        for args, env in [(("-s",), {}), (("-s", "unknown"), {}),
                          (("--unknown",), {}), ((), {"NEWBEE_INIT_TIMEOUT": "0"})]:
            with self.subTest(args=args, env=env):
                result = self.run_cli(*args, **env)
                self.assertEqual(result.returncode, 2)
                self.assertFalse(self.calls.exists())


if __name__ == "__main__":
    unittest.main()
