"""执行真实回归脚本，验证控制台输出与完整日志契约。"""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


class RegressionOutputTests(unittest.TestCase):
    """临时命令夹具隔离数据库与供应商，测试结束删除全部数据。"""

    def test_output_modes_and_failure(self):
        """前置临时 Go 与数据库命令；覆盖静默、详细及失败，验证退出码、日志和独立目录，自动清理。"""
        with tempfile.TemporaryDirectory(prefix='xhub-output-') as directory:
            root = Path(directory)
            (root / 'scripts').mkdir()
            shutil.copy(Path(__file__).with_name('regression.sh'), root / 'scripts/regression.sh')
            (root / 'bin').mkdir()
            commands = {
                'psql': '#!/bin/bash\nexit 0\n',
                'go': '#!/bin/bash\necho "=== RUN   TestExample"\nfor ((i=0;i<10000;i++)); do echo "trace gateway diagnostic"; done\necho "stderr diagnostic" >&2\necho "--- $RESULT: TestExample (0.01s)"\nexit "$STATUS"\n',
            }
            for name, content in commands.items():
                path = root / 'bin' / name
                path.write_text(content)
                path.chmod(0o755)
            for verbose, status in [(False, 0), (True, 0), (False, 7), (True, 7)]:
                with self.subTest(verbose=verbose, status=status):
                    env = {**os.environ, 'PATH': str(root / 'bin') + ':' + os.environ['PATH'], 'STATUS': str(status), 'RESULT': 'FAIL' if status else 'PASS'}
                    result = subprocess.run(['bash', str(root / 'scripts/regression.sh'), *(['-v'] if verbose else [])], env=env, capture_output=True, text=True)
                    output = result.stdout + result.stderr
                    self.assertEqual(result.returncode, status, output)
                    self.assertNotIn('gateway diagnostic', output)
                    self.assertNotIn('stderr diagnostic', output)
                    self.assertEqual('=== RUN' in output, verbose)
                    self.assertLess(len(output), 1000)
                    log = Path(next(line.split(': ', 1)[1] for line in output.splitlines() if line.startswith('full regression log: ')))
                    self.assertEqual(log.read_text().count('trace gateway diagnostic'), 10000)
                    self.assertIn('stderr diagnostic', log.read_text())
                    self.assertIn('backend regression failed' if status else 'backend regression passed', output)
            self.assertEqual(len(list((root / '.e2e/runs').iterdir())), 4)


if __name__ == '__main__':
    unittest.main()
