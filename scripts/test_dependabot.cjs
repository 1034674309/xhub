const { test } = require('node:test');
const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const { join } = require('node:path');
const yaml = require('../frontend/node_modules/js-yaml');

/** 校验维护策略；参数为解析后的配置，返回无值，缺少目录、调度或分组时抛错；配置回归使用，纯读取无需清理。 */
function assertPolicy(config) {
  assert.equal(config.version, 2);
  assert.deepEqual(config.updates.map(item => `${item['package-ecosystem']}:${item.directory}`).sort(),
    ['gomod:/', 'npm:/frontend', 'npm:/website']);
  for (const entry of config.updates) {
    assert.deepEqual(entry.schedule, { interval: 'weekly', day: 'monday', time: '09:00', timezone: 'Asia/Shanghai' });
    assert.equal(entry['open-pull-requests-limit'], 3);
    assert.equal(entry.ignore, undefined, '不能通过忽略规则屏蔽安全修复');
    const groups = Object.values(entry.groups);
    for (const kind of ['version-updates', 'security-updates']) {
      const group = groups.find(item => (item['applies-to'] || 'version-updates') === kind);
      assert.ok(group, `缺少 ${kind} 分组`);
      assert.deepEqual(group.patterns, ['*']);
      assert.deepEqual(group['update-types'], ['minor', 'patch'], '主版本必须独立审核');
    }
  }
}

const config = yaml.load(readFileSync(join(__dirname, '../.github/dependabot.yml'), 'utf8'));

/** 验证真实配置覆盖三处依赖、小版本分组及即时安全组；前置仓库 YAML，结果满足维护策略，无外部请求和清理。 */
test('Dependabot uses weekly grouped updates and separate security groups', () => assertPolicy(config));

/** 验证缺失目录、错误调度、主版本混入和屏蔽更新均被拒绝；输入为独立副本，断言抛错，无持久副作用。 */
test('Dependabot policy rejects missing coverage and unsafe grouping', () => {
  const mutations = [
    value => value.updates.pop(),
    value => { value.updates[0].schedule.interval = 'daily'; },
    value => { Object.values(value.updates[0].groups)[0]['update-types'].push('major'); },
    value => { value.updates[0].ignore = [{ 'dependency-name': '*' }]; },
    value => { delete value.updates[0].groups['frontend-security']; },
  ];
  for (const mutate of mutations) {
    const candidate = structuredClone(config);
    mutate(candidate);
    assert.throws(() => assertPolicy(candidate));
  }
});
