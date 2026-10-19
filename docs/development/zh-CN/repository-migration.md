# 七牛仓库迁移说明

项目的代码、后续 Issue 和发布流程使用 [qiniu/xhub](https://github.com/qiniu/xhub)。本次从旧仓库 `main` 的 `cc7912c8927b378e25fa7ea5e3fcd7976326218f` 初始化，保留全部可达提交及 `main`、`dev` 分支；旧仓库当时没有标签或 Release。

## Issue 对照

手动复制保留正文、勾选进度、历史评论及开关状态。新对象由迁移账号创建；原作者、原创建时间与原评论链接写在正文中，编号以此表为准。旧 PR、评审、Actions 运行记录仍在旧仓库，Git 提交历史已保留。

| 旧 Issue | 七牛 Issue | 迁移时状态 |
| --- | --- | --- |
| [3](https://github.com/sunqirui1987/xhub/issues/3) | [1](https://github.com/qiniu/xhub/issues/1) | open |
| [6](https://github.com/sunqirui1987/xhub/issues/6) | [2](https://github.com/qiniu/xhub/issues/2) | closed |
| [7](https://github.com/sunqirui1987/xhub/issues/7) | [3](https://github.com/qiniu/xhub/issues/3) | open |
| [8](https://github.com/sunqirui1987/xhub/issues/8) | [4](https://github.com/qiniu/xhub/issues/4) | open |
| [9](https://github.com/sunqirui1987/xhub/issues/9) | [5](https://github.com/qiniu/xhub/issues/5) | closed |
| [15](https://github.com/sunqirui1987/xhub/issues/15) | [6](https://github.com/qiniu/xhub/issues/6) | open |
| [16](https://github.com/sunqirui1987/xhub/issues/16) | [7](https://github.com/qiniu/xhub/issues/7) | open |
| [17](https://github.com/sunqirui1987/xhub/issues/17) | [8](https://github.com/qiniu/xhub/issues/8) | closed |
| [20](https://github.com/sunqirui1987/xhub/issues/20) | [9](https://github.com/qiniu/xhub/issues/9) | closed |

## 发布与服务器

- Actions 工作流随代码迁移，镜像路径按仓库名生成：`ghcr.io/qiniu/xhub-gateway` 和 `ghcr.io/qiniu/xhub-console`。镜像发布必须先通过单元、后台 regression 和浏览器 E2E。
- Pages 来源使用 GitHub Actions，预期官网为 <https://qiniu.github.io/xhub/>；实际发布状态以新仓库 Actions 为准。
- 示例配置和部署脚本已纳入 Git；生产配置、供应商凭据、数据库、Redis 和账号数据仍保存在服务器，部署时原位保留，备份位于服务器私有目录。运行报告和测试凭据不会发布到仓库。
- 原有 Go module 标识 `github.com/sunqirui1987/xhub` 保持兼容，暂不更改内部 import 路径；这不会影响在新仓库构建、发布或运行。后续如需改变公共 module 标识，应单独评估下游引用。

## 开发地址

现有 checkout 可将旧远程保留为 `legacy`，主要远程指向新仓库：

~~~bash
git remote set-url origin git@github.com:qiniu/xhub.git
git fetch origin
~~~

更新前检查本地未提交文件。服务器更新使用经提交校验的镜像 digest，不直接跟随可变的 `main` 标签。
