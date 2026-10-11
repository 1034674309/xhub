"""统一 E2E 命令参数；把现成网关、共享基线和私有环境分发给各自生命周期。"""
import argparse
import time

from . import support as module
from .dataset import Dataset
from .runner_runtime import ROOT, E2E_CATEGORIES
from .runner_shared import shared_main
from .runner_isolated import run_isolated


def run_existing(args):
    """用途：为后台/浏览器回归复用已启动的隔离网关；参数为 CLI，返回退出码；只建数或验收，不拥有服务与数据库清理权，失败保存报告。"""
    dataset = Dataset(args.gateway, args.directory, module.load_manifest())
    try:
        dataset.seed()
        if not args.seed_only:
            dataset.verify()
    except Exception as error:
        dataset.report.update(status="failed", error=type(error).__name__ + ": " + str(error))
        dataset.save()
        raise SystemExit("真实数据集失败，查看私有运行目录 report.json") from error
    print("真实数据集完成，报告: " + str(dataset.directory / "report.json"))
    return 0


def main():
    """用途：一条命令完成真实数据验收；参数来自 CLI/环境，返回退出码；默认清理 schema/Redis，keep-data 仅保留明确创建的基线。"""
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--gateway", help="复用已启动的隔离网关")
    parser.add_argument("--seed-only", action="store_true", help="仅向已启动网关建数，不调用模型")
    parser.add_argument("--keep-data", action="store_true")
    parser.add_argument("--api-only", action="store_true", help="只运行后台真实请求验收，报告明确浏览器未执行")
    parser.add_argument("--resume", action="store_true", help="复用同一目录已保存基线和81请求证据，续跑新增场景与浏览器")
    parser.add_argument("--directory", default=str(ROOT / ".e2e" / ("real-dataset-" + time.strftime("%Y%m%d-%H%M%S"))))
    parser.add_argument("--shared-target", action="store_true", help="直连并保留用户指定的 xhub/public 与 Redis DB 1")
    parser.add_argument("--phase", choices=("seed", "verify"), help="共享目标分为构建数据和验收数据两个阶段")
    parser.add_argument("--with-regression", action="store_true", help="验收阶段追加后台 regression 套件")
    parser.add_argument("--category", action="append", choices=E2E_CATEGORIES, help="只执行所选分类，可重复指定；必须使用共享 verify 基线")
    parser.add_argument("--list-categories", action="store_true", help="只列出分类，不读取凭据、不启动服务")
    args = parser.parse_args()
    if args.list_categories:
        print("E2E categories (make e2e <category> [<category> ...]):\n" + "\n".join(E2E_CATEGORIES))
        return 0
    if args.gateway:
        if args.shared_target or args.phase or args.category or args.with_regression or args.resume or args.keep_data:
            parser.error("--gateway 不可与 runner 服务生命周期参数混用")
        return run_existing(args)
    if args.seed_only:
        parser.error("--seed-only 必须提供 --gateway")
    if args.category:
        if not args.shared_target or args.phase != "verify":
            parser.error("--category 只能与 --shared-target --phase verify 一起使用")
        if "regression" in args.category and not args.with_regression:
            parser.error("regression 分类必须同时指定 --with-regression")
        args.category = list(dict.fromkeys(args.category))
    if args.shared_target:
        if not args.phase:
            parser.error("--shared-target 必须指定 --phase seed 或 verify")
        return shared_main(args, module.load_manifest())
    if args.phase or args.with_regression:
        parser.error("--phase/--with-regression 只能与 --shared-target 一起使用")
    if args.resume and not args.keep_data:
        parser.error("续跑必须同时指定 --keep-data，避免删除已有验收基线")
    return run_isolated(args)
