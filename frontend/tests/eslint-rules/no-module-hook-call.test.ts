import { RuleTester } from "eslint";
import rule from "../../scripts/eslint-rules/no-module-hook-call.mjs";

/** 目的：防止模块顶层以及大写配置数组的回调调用 Hook；前置真实 ESLint 解析；验证合法组件、Hook、纯配置及非法工具函数；纯静态检查无需数据清理。 */
const ruleTester = new RuleTester({
  languageOptions: { ecmaVersion: "latest", sourceType: "module", parserOptions: { ecmaFeatures: { jsx: true } } },
});
ruleTester.run("no-module-hook-call", rule as never, {
  valid: [
    "const PRICE_FIELDS = categories.flatMap(category => category.fields);",
    "function Table() { const t = useT(); return options.map(option => t(option)); }",
    "const Table = () => { const t = useT(); return <div>{t('title')}</div>; };",
    "export default function () { const t = useT(); return null; }",
    "const Table = React.forwardRef((props, ref) => { useT(); return null; });",
    "const Table = memo(() => { useT(); return null; });",
    "function useLabels() { return useT(); }",
    "const useLabels = () => useT();",
    "function Table() { return React.useContext(Context); }",
  ],
  invalid: [
    { code: "const t = useT();", errors: [{ messageId: "moduleHook" }] },
    { code: "const ACTION_OPTIONS = values.map(value => { const t = useT(); return value; });", errors: [{ messageId: "moduleHook" }] },
    { code: "const PRICE_FIELDS = categories.flatMap(category => { const t = useT(); return category.fields; });", errors: [{ messageId: "moduleHook" }] },
    { code: "function formatLabel() { return useT(); }", errors: [{ messageId: "moduleHook" }] },
    { code: "const value = React.useContext(Context);", errors: [{ messageId: "moduleHook" }] },
  ],
});
