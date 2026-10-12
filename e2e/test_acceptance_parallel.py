"""独立媒体并行调度与失败检查点测试。"""
import threading
import tempfile
import unittest
from unittest import mock
from acceptance import Dataset, load_manifest
from acceptance.media import MediaCases


class ParallelTests(unittest.TestCase):
    """用途：通过同步屏障证明同时执行，使用临时目录验证报告与失败保留，不调用供应商。"""

    def test_parallel_overlap_and_failure_preserve_other_results(self):
        """目的：两用例必须并行到达屏障，失败不能丢失另一结果；前置临时数据集，验证独立目录、汇总与真实失败清单，目录自动清理。"""
        for failed in (False, True):
            with self.subTest(failed=failed), tempfile.TemporaryDirectory() as directory:
                dataset = Dataset('http://127.0.0.1:4000', directory, load_manifest(), logger=lambda _: None)
                dataset.acceptance_keys = lambda: []
                dataset.start_checklist(False, ['image', 'ark-video'])
                dataset.media_jobs = 2
                barrier = threading.Barrier(2)
                directories = []

                def verify(worker, key):
                    """用途：模拟独立媒体协议结果；参数为工作上下文和密钥，无返回，屏障超时证明未并行；只写临时证据。"""
                    category = next(iter(worker.selected_categories))
                    directories.append(worker.directory)
                    with worker.case(category):
                        barrier.wait(timeout=3)
                        worker.report['media_calls'] = [{'model': category, 'call_id': category}]
                        if failed and category == 'image':
                            raise RuntimeError('image failed')
                        worker.save()

                with mock.patch.object(MediaCases, 'verify_media_models', verify):
                    if failed:
                        with self.assertRaisesRegex(RuntimeError, 'image failed'):
                            dataset.verify_media_models_parallel({})
                    else:
                        dataset.verify_media_models_parallel({})
                self.assertEqual(len(set(directories)), 2)
                self.assertEqual({row['model'] for row in dataset.report['media_calls']}, {'image', 'ark-video'})
                self.assertEqual(dataset.checklist.rows[1]['status'], 'passed')
                self.assertEqual(dataset.checklist.rows[0]['status'], 'failed' if failed else 'passed')
