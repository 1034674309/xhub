"""启动脚本隔离回归；假工具不访问 Docker 或网络，临时目录自动清理。"""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


class RunIsolationTests(unittest.TestCase):
    """验证运行目录、独立 schema 和清理错误传播。"""

    def execute(self, root, name, fail=False):
        """创建假命令并运行真实脚本；参数为临时根、运行名和故障开关，返回结果和目录，调用方回收临时文件。"""
        directory = root / name
        directory.mkdir()
        binaries = root / (name + '-bin')
        binaries.mkdir()
        programs = {
            'docker': ['#!/bin/bash', 'echo "$*" >> "$MOCK_SQL_LOG"',
                       'if [[ "$*" == *"DROP SCHEMA"* && "$MOCK_FAIL_DROP" == 1 ]]; then exit 42; fi'],
            'python3': ['#!/bin/bash', 'exit 0'],
        }
        for command, lines in programs.items():
            executable = binaries / command
            executable.write_text(chr(10).join(lines) + chr(10))
            executable.chmod(0o755)
        gateway = directory / 'xhub'
        gateway.write_text(chr(10).join(['#!/bin/bash', 'exit 0', '']))
        gateway.chmod(0o755)
        sweep = directory / 'livesweep'
        sweep.write_text(chr(10).join(['#!/bin/bash', 'exit 0', '']))
        sweep.chmod(0o755)
        env = {**os.environ, 'PATH': str(binaries) + os.pathsep + os.environ['PATH'],
               'E2E_RUN_DIR': str(directory), 'E2E_PREBUILT': '1', 'E2E_ENTERPRISE_FIXTURE': '0',
               'MOCK_SQL_LOG': str(directory / 'sql.log'), 'MOCK_FAIL_DROP': '1' if fail else '0'}
        result = subprocess.run(['bash', str(Path(__file__).with_name('start-gateway.sh'))],
                                env=env, text=True, capture_output=True, timeout=10)
        return result, directory

    def test_distinct_runs(self):
        """两轮必须创建不同 schema 且只删除本轮；前置假命令，临时目录自动清理，无业务数据。"""
        with tempfile.TemporaryDirectory() as temporary:
            first, a = self.execute(Path(temporary), 'first')
            second, b = self.execute(Path(temporary), 'second')
            self.assertEqual(first.returncode, 0, first.stderr)
            self.assertEqual(second.returncode, 0, second.stderr)
            schemas = [(directory / 'schema.txt').read_text().strip() for directory in (a, b)]
            self.assertNotEqual(*schemas)
            for directory, own, other in [(a, schemas[0], schemas[1]), (b, schemas[1], schemas[0])]:
                operations = (directory / 'sql.log').read_text()
                self.assertIn('CREATE SCHEMA ' + own, operations)
                self.assertIn('DROP SCHEMA IF EXISTS ' + own + ' CASCADE', operations)
                self.assertNotIn(other, operations)

    def test_cleanup_failure(self):
        """DROP 失败必须非零退出并保留证据，禁止吞错；假数据库无副作用，临时目录自动清理。"""
        with tempfile.TemporaryDirectory() as temporary:
            result, directory = self.execute(Path(temporary), 'failed', True)
            self.assertNotEqual(result.returncode, 0)
            self.assertTrue((directory / 'schema.txt').exists())
            self.assertTrue((directory / 'cleanup.log').exists())
