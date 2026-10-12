"""验证浏览器入口独立库与构建；使用假工具运行真实入口，临时目录自动清理。"""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
ROOT = Path(__file__).resolve().parents[1]

class BrowserEntryTests(unittest.TestCase):
    """覆盖默认、专属、失败清理及非法数据库。"""
    def execute(self, database=None, fail=False):
        """参数为数据库与模拟失败，返回退出结果、SQL 与配置；临时项目不访问外部服务。"""
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            for name in ('scripts', 'frontend', 'bin'):
                (root / name).mkdir()
            for name in ('e2e.sh', 'browser-lock.sh'):
                shutil.copy(ROOT / 'scripts' / name, root / 'scripts' / name)
            (root / 'frontend/tsconfig.json').write_text(json.dumps({'include':['src/**/*.ts','.next/types/**/*.ts']}))
            programs = {'docker':'echo "$*" >> "$MOCK_SQL_LOG"', 'npm':'exit 0', 'go':'exit 0',
                        'npx':'mkdir -p "$E2E_RUN_DIR"\necho e2e_123_456 > "$E2E_RUN_DIR/schema.txt"\nexit "$MOCK_EXIT"'}
            for name, program in programs.items():
                target = root / 'bin' / name
                target.write_text('#!/bin/bash\n' + program + '\n')
                target.chmod(0o755)
            env = {k:v for k,v in os.environ.items() if not k.startswith('E2E_')}
            env.update(PATH=str(root/'bin')+os.pathsep+env['PATH'], E2E_RUN_DIR=str(root/'run'),
                       E2E_BUILD_DIR='.next-private', E2E_UI_PORT='53352', E2E_GW_PORT='54352', E2E_UP_PORT='54362',
                       MOCK_SQL_LOG=str(root/'sql.log'), MOCK_EXIT='17' if fail else '0')
            if database is not None:
                env['E2E_DATABASE']=database
            result=subprocess.run(['bash',str(root/'scripts/e2e.sh'),'example.test.ts'],env=env,capture_output=True,text=True,timeout=15)
            sql=(root/'sql.log').read_text() if (root/'sql.log').exists() else ''
            config=json.loads((root/'frontend/tsconfig.e2e.json').read_text()) if (root/'frontend/tsconfig.e2e.json').exists() else None
            self.assertFalse((root/'.e2e/browser.lock').exists())
            return result,sql,config

    def test_default_and_private(self):
        """默认和专属目标同库探测/清理且保留构建；假工具无业务数据，自动回收。"""
        for database in (None,'xhub_e2e_audit'):
            with self.subTest(database=database):
                result,sql,config=self.execute(database)
                self.assertEqual(result.returncode,0,result.stderr)
                self.assertEqual(len(sql.splitlines()),2)
                self.assertTrue(all('-d '+(database or 'xhub')+' ' in row for row in sql.splitlines()))
                self.assertIn('DROP SCHEMA IF EXISTS e2e_123_456 CASCADE',sql)
                self.assertEqual(config['include'],['src/**/*.ts','.next-private/types/**/*.ts'])

    def test_failed_browser_cleanup(self):
        """浏览器失败仍清理专属库并保留退出码；假数据库无数据，自动回收。"""
        result,sql,_=self.execute('xhub_e2e_audit',True)
        self.assertEqual(result.returncode,17)
        self.assertIn('-d xhub_e2e_audit',sql)
        self.assertIn('DROP SCHEMA',sql)

    def test_invalid_target(self):
        """非法名称在任何 SQL/构建前拒绝；边界不产生外部操作，临时目录自动回收。"""
        for database in ('../xhub','xhub;drop','Xhub'):
            with self.subTest(database=database):
                result,sql,config=self.execute(database)
                self.assertNotEqual(result.returncode,0)
                self.assertEqual(sql,'')
                self.assertIsNone(config)
