"""组合业务场景的验收上下文；没有第二个命令行入口。"""
from .runtime import RuntimeCases
from .catalog import CatalogCases
from .seed import SeedCases
from .billing import BillingCases
from .chat import ChatCases
from .routing import RoutingCases
from .affinity import AffinityCases
from .media import MediaCases
from .permissions import PermissionsCases
from .guards import GuardsCases
from .limits import LimitsCases
from .suite import SuiteCases


class Dataset(RuntimeCases, CatalogCases, SeedCases, BillingCases, ChatCases, RoutingCases, AffinityCases, MediaCases, PermissionsCases, GuardsCases, LimitsCases, SuiteCases):
    """用途：向 runner 提供统一状态和按分类执行的场景；初始化参数见 RuntimeCases，各模块显式导入、无动态绑定。"""
