"""验证文档中的离线入口确实存在、只调用隔离脚本，并传递用例与失败状态。"""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


class OfflineMakeTests(unittest.TestCase):
    """读取真实 Makefile，在临时目录替换外部执行边界；测试结束清理全部夹具。"""

    def test_offline_target_and_selected_cases(self):
        """前置真实 Makefile；正常/指定用例/空值均展开为离线脚本，不执行共享建数或供应商调用，无持久数据。"""
        root = Path(__file__).resolve().parents[1]
        for cases in ['', 'agent-native-protocol.test.ts agent-chat-bridge.test.ts']:
            with self.subTest(cases=cases):
                result = subprocess.run(['make', '-n', 'e2e-offline', 'E2E_TESTS=' + cases], cwd=root, text=True, capture_output=True)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(result.stdout.strip(), ('bash scripts/e2e.sh ' + cases).strip())
                self.assertNotIn('--shared-target', result.stdout)

    def test_offline_target_propagates_failure(self):
        """前置临时离线脚本与真实 Makefile；验证正常返回和失败传播、用例参数完整传入，自动清理目录。"""
        with tempfile.TemporaryDirectory(prefix='xhub-offline-make-') as directory:
            root = Path(directory)
            shutil.copy(Path(__file__).resolve().parents[1] / 'Makefile', root / 'Makefile')
            (root / 'scripts').mkdir()
            (root / 'scripts/e2e.sh').write_text('printf "%s\\n" "$@"\nexit "$OFFLINE_STATUS"\n')
            for status in [0, 7]:
                with self.subTest(status=status):
                    result = subprocess.run(['make', 'e2e-offline', 'E2E_TESTS=first.test.ts second.test.ts'], cwd=root, env={**os.environ, 'OFFLINE_STATUS': str(status)}, text=True, capture_output=True)
                    self.assertEqual(result.returncode == 0, status == 0, result.stderr)
                    self.assertIn('first.test.ts\nsecond.test.ts\n', result.stdout)


if __name__ == '__main__':
    unittest.main()
