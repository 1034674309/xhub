/** 发布配置回归：读取真实 YAML 并执行内嵌 shell；外部发布使用临时夹具。 */
const { test } = require('node:test');
const assert = require('node:assert/strict');
const { readFileSync, mkdtempSync, mkdirSync, writeFileSync, rmSync, existsSync } = require('node:fs');
const { tmpdir } = require('node:os');
const { join } = require('node:path');
const { spawnSync } = require('node:child_process');
const yaml = require('../frontend/node_modules/js-yaml');
const workflow = yaml.load(readFileSync(join(__dirname, '../.github/workflows/release.yml'), 'utf8'));
const publish = workflow.jobs.publish;

/** 按名称返回发布步骤；参数为名称，缺失时抛出断言；供测试调用，无副作用。 */
function step(name) {
  const result = publish.steps.find(item => item.name === name);
  assert.ok(result, name);
  return result;
}

/** 配置准备覆盖干净检出、已有配置和缺失示例；执行真实工作流 shell，验证生成内容、保留原配置及失败退出，finally 删除临时目录。 */
test('verification prepares configs/config.yaml before Go tests without overwriting existing configuration', () => {
  const steps = workflow.jobs.verify.steps;
  const prepareIndex = steps.findIndex(item => item.name === 'Prepare test configuration');
  const goIndex = steps.findIndex(item => /go test/.test(item.run || ''));
  assert.ok(prepareIndex >= 0 && prepareIndex < goIndex, '配置必须在 Go 测试前准备');
  const prepare = steps[prepareIndex];
  assert.equal(prepare.shell, 'bash');
  const example = readFileSync(join(__dirname, '../configs/config.example.yaml'), 'utf8');
  for (const scenario of ['clean checkout', 'existing configuration', 'missing example']) {
    const directory = mkdtempSync(join(tmpdir(), 'xhub-release-config-'));
    try {
      mkdirSync(join(directory, 'configs'));
      if (scenario !== 'missing example') {
        writeFileSync(join(directory, 'configs/config.example.yaml'), example);
      }
      const target = join(directory, 'configs/config.yaml');
      if (scenario === 'existing configuration') writeFileSync(target, 'model_list: []\n# preserved configuration\n');
      const result = spawnSync('bash', ['-c', prepare.run], { cwd: directory, encoding: 'utf8' });
      if (scenario === 'missing example') {
        assert.notEqual(result.status, 0, '缺失示例时必须报告准备失败');
        assert.equal(existsSync(target), false, '失败时不得留下空配置');
      } else {
        assert.equal(result.status, 0, scenario + ': ' + result.stderr);
        assert.equal(readFileSync(target, 'utf8'), scenario === 'clean checkout' ? example : 'model_list: []\n# preserved configuration\n', scenario);
      }
    } finally { rmSync(directory, { recursive: true, force: true }); }
  }
});

/** 正常流程验证发布依赖真实测试且 PR 没有写权限；只读配置，无需清理。 */
test('publication requires regression and browser verification', () => {
  assert.deepEqual(workflow.on.push, { branches: ['main'], tags: ['v*'] });
  assert.equal(publish.needs, 'verify');
  assert.match(publish.if, /github.event_name != 'pull_request'/);
  assert.deepEqual(workflow.permissions, { contents: 'read' });
  assert.deepEqual(publish.permissions, { contents: 'write', packages: 'write' });
  const commands = workflow.jobs.verify.steps.map(item => item.run || '').join('\n');
  assert.match(commands, /go test/);
  assert.match(commands, /bash scripts\/regression.sh -v/);
  assert.match(commands, /scenarios\/keys\/rotation-secret.test.ts/);
  assert.ok(workflow.jobs.verify.steps.some(item => item.run === 'docker compose down -v' && item.if === 'always()'));
});

/** 边界覆盖正式版、预发布、非法及注入 ref；执行真实 shell，finally 清理临时输出。 */
test('version validation accepts valid versions and rejects malformed refs', () => {
  const directory = mkdtempSync(join(tmpdir(), 'xhub-release-version-'));
  try {
    for (const [version, accepted] of [
      ['v1.2.3', true], ['v0.0.0', true], ['v2.0.0-rc.1', true],
      ['v2.0.0-beta.0', true], ['v2.0.0-alpha.1', true],
      ['v01.2.3', false], ['v1.2', false], ['v1.2.3-rc.01', false],
      ['v1.2.3;touch injected', false], ['v1.2.3-unknown.1', false],
    ]) {
      const result = spawnSync('bash', ['-c', step('Validate version').run], {
        cwd: directory, encoding: 'utf8',
        env: { ...process.env, REF_TYPE: 'tag', REF_NAME: version, GITHUB_REPOSITORY: 'owner/xhub', GITHUB_OUTPUT: join(directory, 'output') },
      });
      assert.equal(result.status === 0, accepted, version + ': ' + result.stderr);
    }
  } finally { rmSync(directory, { recursive: true, force: true }); }
});

/** 镜像契约验证两个镜像、预发布 latest 隔离及 standalone 入口；只读无需清理。 */
test('both images preserve tags and standalone layout', () => {
  const builds = publish.steps.filter(item => item.uses === 'docker/build-push-action@v6');
  assert.equal(builds.length, 2);
  assert.ok(builds.every(item => item.with.push && item.with.platforms === 'linux/amd64'));
  const metadata = publish.steps.filter(item => item.uses === 'docker/metadata-action@v5');
  assert.equal(metadata.length, 2);
  assert.ok(metadata.every(item => item.with.flavor.includes("!contains(github.ref_name, '-')")));
  assert.match(step('Prepare runtime artifacts').run, /standalone\/frontend\/server.js/);
  assert.match(readFileSync(join(__dirname, '../deploy/Dockerfile.release'), 'utf8'), /\/app\/frontend\/server.js/);
  assert.equal(step('Publish GitHub Release').if, "github.ref_type == 'tag'");
});

/** 发布 shell 覆盖初次发布、重跑、预发布和上传失败；gh 夹具只记录命令，finally 清理。 */
test('release reruns are repeatable and upload failures propagate', () => {
  const directory = mkdtempSync(join(tmpdir(), 'xhub-release-publish-'));
  try {
    writeFileSync(join(directory, 'gh'), '#!/bin/bash\nprintf "%s\\n" "$*" >> "$CALLS"\ncase "$2" in view) exit "$EXISTS" ;; upload) exit "$UPLOAD_STATUS" ;; esac\n', { mode: 0o755 });
    for (const [version, exists, uploadStatus] of [['v1.2.3', '1', '0'], ['v1.2.3', '0', '0'], ['v2.0.0-rc.1', '1', '0'], ['v1.2.3', '0', '1']]) {
      const calls = join(directory, 'calls');
      writeFileSync(calls, '');
      const result = spawnSync('bash', ['-c', step('Publish GitHub Release').run], {
        cwd: directory, encoding: 'utf8', env: { ...process.env, PATH: directory + ':' + process.env.PATH,
          VERSION: version, REPOSITORY: 'owner/xhub', GATEWAY_DIGEST: 'sha256:gateway', CONSOLE_DIGEST: 'sha256:console',
          CALLS: calls, EXISTS: exists, UPLOAD_STATUS: uploadStatus },
      });
      assert.equal(result.status, Number(uploadStatus), result.stderr);
      const log = readFileSync(calls, 'utf8');
      assert.equal(log.includes('release create'), exists === '1');
      if (version.includes('-')) assert.match(log, /--prerelease --latest=false/);
      assert.match(log, /release upload .* docker-images.txt --clobber/);
      assert.equal(readFileSync(join(directory, 'docker-images.txt'), 'utf8'), 'ghcr.io/owner/xhub-gateway@sha256:gateway\nghcr.io/owner/xhub-console@sha256:console\n');
    }
  } finally { rmSync(directory, { recursive: true, force: true }); }
});
