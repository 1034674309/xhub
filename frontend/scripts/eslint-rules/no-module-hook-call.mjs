/** 判断函数节点是否属于组件或自定义 Hook；参数为 ESTree 节点，返回布尔值；供模块级 Hook 检查使用，不执行产品代码。 */
function isRenderFunction(node) {
  const name = node.id?.name ?? (node.parent?.type === "VariableDeclarator" ? node.parent.id?.name : undefined);
  return typeof name === "string" && (/^use[A-Z0-9]/.test(name) || /^[A-Z]/.test(name));
}

/** 创建模块级 Hook 检查器；参数为 ESLint 上下文，返回调用表达式监听器；禁止模块初始化及其 map/flatMap 回调使用 Hook，错误通过 lint 报告，无运行时副作用。 */
function create(context) {
  return {
    CallExpression(node) {
      const callee = node.callee;
      const name = callee.type === "Identifier" ? callee.name : callee.type === "MemberExpression" && !callee.computed ? callee.property.name : "";
      if (!/^use[A-Z0-9]/.test(name)) return;
      // 官方规则会把 PRICE_FIELDS 等大写常量的回调误判为组件；继续沿祖先查找真正的渲染函数。
      for (let parent = node.parent; parent; parent = parent.parent) {
        if (["FunctionDeclaration", "FunctionExpression", "ArrowFunctionExpression"].includes(parent.type) && isRenderFunction(parent)) return;
      }
      context.report({ node, messageId: "moduleHook", data: { name } });
    },
  };
}

export default {
  meta: {
    type: "problem",
    docs: { description: "禁止在模块初始化及普通回调中调用 React Hook。" },
    schema: [],
    messages: { moduleHook: "{{name}} 必须在组件或自定义 Hook 内调用，不能在模块初始化或普通工具函数中调用。" },
  },
  create,
};
