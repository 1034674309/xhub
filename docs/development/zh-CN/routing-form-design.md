# 路由表单与 JSON 同步

## 改表单时保持这些映射

| 页面动作 | JSON 结果 |
| --- | --- |
| 继承默认权重 | 省略 allocations |
| 自定义权重 | deployment_id / weight 数组 |
| 切换非加权策略 | 移除 allocations |
| 清空某源回退 | 保留该源到 [] 的映射 |
| 删除整条回退 | 恢复继承 |
| 弹窗保存组 | 只更新模板草稿，提交后生效 |

非法 JSON 阻止保存及切回表单。0 不能转换成默认 1；空输入是错误，不自动变成 0。异步加载不能覆盖用户草稿。

~~~bash
cd frontend
npm run test:unit -- src/components/route_templates/templateForm.test.ts
~~~

[完整 JSON](routing.md) · [表单源码](../../../frontend/src/components/route_templates)
