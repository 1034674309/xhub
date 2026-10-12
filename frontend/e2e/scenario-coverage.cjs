const fs = require('node:fs');
const path = require('node:path');
/** 用途：按独立业务清单核验实际结果；参数为清单与结果，返回分母、通过数及未通过条目。
 * reporter 与单元测试调用；缺失、失败、跳过、重复绑定均不计通过，不读取业务数据。 */
function evaluateScenarios(catalog, results) {
  const ids = new Set();
  const files = new Set();
  for (const item of catalog) {
    if (!item.id || !item.file || !item.title || ids.has(item.id)) throw new Error('业务清单缺少字段或 ID 重复');
    // 一个业务情况绑定一个独立文件，重复文件会再次串联情况并破坏独立运行结果。
    if (!item.file.endsWith('.test.ts') || files.has(item.file)) throw new Error('业务清单必须绑定独立 .test.ts，文件重复或后缀错误');
    ids.add(item.id);
    files.add(item.file);
  }
  if (!catalog.length) throw new Error('业务清单不能为空');
  const rows = catalog.map(item => {
    const matches = results.filter(result => result.file === item.file && (!item.title || result.title === item.title));
    const state = matches.length !== 1 ? (matches.length ? 'ambiguous' : 'missing') : matches[0].status;
    return {...item, state};
  });
  const unregistered = results.filter(result => !catalog.some(item => item.file === result.file && item.title === result.title));
  return {unregistered, total: rows.length, passed: rows.filter(row => row.state === 'passed').length,
    unmet: rows.filter(row => row.state !== 'passed'), scenarios: rows};
}
/** 用途：保存场景覆盖结果；参数为清单与 Playwright 测试，返回报告；只写当前运行目录。
 * 完整运行调用，不用本次发现的测试生成分母；未知场景需先审查并登记，重试后通过仍单列不计完整通过。 */
function reportCoverage(catalog, tests, output) {
  const root = path.resolve(__dirname, '..');
  const results = tests.map(test => {
    // 注册函数位于共享 helper 时 location 指向 helper；文件 suite 才是可单独运行的 .test.ts。
    let suite = test.parent;
    while (suite && suite.type !== 'file') suite = suite.parent;
    const file = suite?.location?.file || test.location.file;
    return {file: path.relative(root, file).replaceAll(path.sep, '/'), title: test.title,
      status: test.results.length === 1 ? test.results[0].status : 'retried'};
  });
  const report = evaluateScenarios(catalog, results);
  fs.mkdirSync(path.dirname(output), {recursive: true});
  fs.writeFileSync(output, JSON.stringify(report, null, 2) + '\n');
  return report;
}
module.exports = {evaluateScenarios, reportCoverage};
