import test from "node:test";
import assert from "node:assert/strict";
import { once } from "node:events";
import { mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { serve } from "../src/serve.mjs";

// 验证静态预览真实 HTTP 边界；使用独立临时目录与随机端口，覆盖成功、HEAD、子路径和缺页，finally 关闭并删除数据。
test("预览服务器支持项目子路径并返回真实文件与错误状态", async () => {
  const directory = await mkdtemp(path.join(tmpdir(), "xhub-website-"));
  await writeFile(path.join(directory, "index.html"), "<h1>XHub</h1>");
  await writeFile(path.join(directory, "guide.jpg"), Buffer.from([0xff, 0xd8, 0xff, 0xd9]));
  const server = serve({ directory, port: 0, mount: "/xhub/" });
  try {
    await once(server, "listening");
    const origin = `http://127.0.0.1:${server.address().port}`;
    const response = await fetch(`${origin}/xhub/`);
    assert.equal(response.status, 200);
    assert.match(response.headers.get("content-type"), /text\/html/);
    assert.equal(await response.text(), "<h1>XHub</h1>");
    const head = await fetch(`${origin}/xhub/index.html`, { method: "HEAD" });
    assert.equal(head.status, 200);
    assert.equal(await head.text(), "");
    // 实际 HTTP 返回截图格式及原始字节，防止原图链接被当成下载文件；临时文件在 finally 清理。
    const picture = await fetch(`${origin}/xhub/guide.jpg`);
    assert.equal(picture.status, 200);
    assert.equal(picture.headers.get("content-type"), "image/jpeg");
    assert.deepEqual(Buffer.from(await picture.arrayBuffer()), Buffer.from([0xff, 0xd8, 0xff, 0xd9]));
    const pictureHead = await fetch(`${origin}/xhub/guide.jpg`, { method: "HEAD" });
    assert.equal(pictureHead.headers.get("content-type"), "image/jpeg");
    assert.equal(await pictureHead.text(), "");
    assert.equal((await fetch(`${origin}/xhub/missing.jpg`)).status, 404);
    assert.equal((await fetch(`${origin}/`)).status, 404);
    assert.equal((await fetch(`${origin}/xhub/missing.html`)).status, 404);
    assert.equal((await fetch(`${origin}/xhub/`, { method: "POST" })).status, 404);
  } finally {
    await new Promise((resolve) => server.close(resolve));
    await rm(directory, { recursive: true, force: true });
  }
});

// 验证无效挂载参数被显式拒绝；无需打开端口，输入边界返回异常，无需清理文件。
test("预览服务器拒绝不完整挂载路径", () => {
  assert.throws(() => serve({ mount: "xhub" }), /挂载路径/);
  assert.throws(() => serve({ mount: "/xhub" }), /挂载路径/);
});
