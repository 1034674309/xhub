export const docsHelpZhCN = {
  "help-setup": {
    title: "安装与首次配置",
    description: "部署到第一次成功请求",
    heading1: "部署到第一次成功请求",
    heading2: "操作步骤",
    heading3: "检查与边界",
    body1:
      "先按仓库安装指南部署网关、控制台与存储，确认控制台和网关均可访问。登录后添加供应商凭证，再创建模型部署；空模型目录通常意味着还没有部署，而不是安装失败。",
    body2:
      "在模型目录确认公开模型名与支持的接口，在调试台选择模型发送英文示例。返回成功后创建最小权限的 XHub 虚拟密钥，再接入业务应用。不要把供应商密钥发给业务使用者。",
    body3:
      "网关地址使用调试台显示的 Base URL，保留反向代理前缀，不追加 /v1。接口请求使用相应协议完整路径。升级前备份数据库和部署配置；安装与升级细节见仓库帮助指南。",
  },
  "help-models": {
    title: "供应商与模型部署",
    description: "NewAPI 配置接入、Suno 新协议开发与真实调用验收",
    heading1: "先判断：页面配置还是开发新协议",
    heading2: "NewAPI：连接、部署、密钥、调用",
    heading3: "Suno：当前未内置，必须实现音乐任务协议",
    body1:
      "供应商名称不是协议。NewAPI 实际提供 OpenAI Chat 时，在 XHub 中创建 custom_openai 连接并把部署绑定到已登记的 bypass_openai_chat，无需新建 provider 目录。若只是认证头或固定路径不同，增加已知协议的 Transport；若请求、响应、任务状态都是新结构，则要开发新协议。价格、Logo 和认证表单存在不代表可调用。",
    body2:
      "在模型供应商页面添加 company-newapi，在供应商表单填写服务地址和 NewAPI 密钥。再在模型表单填写公开名 company-chat、账户实际可用的上游型号，选择 company-newapi 和 OpenAI Chat 上游协议，确认价格并保存部署。地址和认证只在模型供应商管理，部署不重复填写。刷新核对后，创建授权 company-chat 的 XHub 虚拟密钥并调用 /v1/chat/completions；业务只持有 XHub 密钥。全部在页面完成，不编辑 model_list。",
    body3:
      "当前没有 Suno 执行器或音乐端点。先确定实际接的是官方授权 API、NewAPI 插件还是第三方中转，取得创建/查询路径、认证、任务 ID、终态和收费单位。自有音乐 JSON 不能冒充 audio/speech，也不能靠 custom_llm_provider=suno 接通。实现后必须验证创建、原身份轮询、失败、多首结果与重复查询唯一结算。",
    detail: `## 第一步：保存模型供应商连接

打开“模型供应商 → 添加提供商”。本例接入 Chat，供应商类型选择 Custom Openai，提供商名称填 company-newapi，在这里填写 API Base 和 API Key。普通 Chat 模型的“供应商目录 ID”留空；它是专用目录标识，不是连接名称。

![中文模型供应商连接表单](/assets/provider-guide/provider-form-zh.jpg)

图中的地址和已遮挡的密钥都是虚构演示值，不代表已验证 NewAPI 账户。请先保存自己的连接。

## 第二步：添加模型部署

打开“模型管理 → 添加模型”。第 1 步选择已保存的供应商后，第 2 步才可填写上游模型 ID、对外模型名称并选择上游协议；第 3 步选择价格目录或填写实际合同费率。保存后刷新核对。

![中文模型部署表单](/assets/provider-guide/deployment-form-zh.jpg)

图中是尚未选择供应商的初始表单。模型区域置灰表示需要先选供应商，不是让你重复填地址、密钥或编辑 model_list。

## 开发新供应商的修改顺序

1. internal/provider/contract/type.go：确认 Action、AuthConfig、TaskID、TaskBilling 能否表达协议。
2. internal/provider/capability/endpoints.go：新媒体协议登记公开端点及兼容关系。
3. internal/provider/<supplier>/：注册 Transport 的协议、鉴权和操作白名单；参考 qiniu/seedance.go。需要专用型号时用 RegisterModel 绑定目录与 TransportID。
4. internal/provider/all/all.go：导入新包，重建并重启。只建目录不会加载。
5. internal/dataplane/execution/official.go、transport 与 gateway：检查入口分派、原生载荷、任务归属与错误；注册不等于所有执行逻辑已具备。
6. internal/provider/measurement、internal/catalog：成功事实与费率分开，缺失维度保留待核价；复杂音乐计价需要扩展。
7. 模型配置、调试台和双语 API：增加参数、查询及结果展示，补单元、真实后台 regression 和浏览器 E2E。

## 保存成功后还要验证什么

- 目录失败可以手动填已确认型号，目录成功不等于推理成功。先非流式，再流式、工具和业务参数。
- 上游 API 根可保留 /proxy/v1，不能填完整操作 URL。改目录路径不改变推理协议。
- 正常部署引用模型供应商，不重复保存地址和密钥。历史数据若残留 api_base，供应商地址更新可能不生效；这是旧数据排查边界，不是推荐配置方式。
- 连接测试可能计费，但不经过完整额度、路由和用量链；必须再用虚拟密钥请求并核对日志与价格快照。
- 创建超时不能证明未受理，不要盲目重发收费任务。当前没有通用后台补查、预算预占或 Suno 回调接收器。

完整双语页面操作、curl 调用和开发文件清单见仓库 docs/development/zh-CN/provider-catalog.md（website 的“接入与集成”目录）。`,
  },
  "help-workspace": {
    title: "组织、团队与访问管理",
    description: "建立使用边界",
    heading1: "建立使用边界",
    heading2: "操作步骤",
    heading3: "检查与边界",
    body1:
      "管理员先创建组织与团队，再邀请或添加成员并分配角色。给团队配置可用模型和预算，项目用于业务归属。成员只应获得完成工作所需的管理权限。",
    body2:
      "创建个人或服务虚拟密钥时确认所属团队、模型范围、有效期和额度。模型范围不能超过上级授权。额度分配、共享余额与不限额的区别见计费说明及额度管理指南。",
    body3:
      "成员离职时撤销相关密钥、移除成员关系，并检查服务集成是否仍使用个人凭据。历史用量用于审计，不应为清理权限而删除费用记录。",
  },
  "help-playground": {
    title: "调试台与业务接入",
    description: "先调试，再集成",
    heading1: "先调试，再集成",
    heading2: "操作步骤",
    heading3: "检查与边界",
    body1:
      "在调试台选择有权限的公开模型与匹配接口，填写提示词或协议正文后执行。流式响应、多模态和文件参数以部署实际能力为准。使用模型详情中的接口能力判断支持范围。",
    body2:
      "成功后复制请求示例，把业务密钥作为环境变量保存，使用相同网关 Base URL。工具接入分类提供 Codex、Claude Code、OpenCode、Cursor、聊天客户端和 SDK 配置；示例提示词保持英文。",
    body3:
      "API 文档可直接在线运行：填写密钥、路径和 JSON，点击运行后查看 HTTP 状态与正文。文件请求勾选 multipart 并选择文件；任务查询切换 GET 并填写任务路径。真实请求可能计费，不会自动重试。",
  },
  "help-routing": {
    title: "路由配置与回退",
    description: "控制部署选择",
    heading1: "控制部署选择",
    heading2: "操作步骤",
    heading3: "检查与边界",
    body1:
      "先为公开模型添加可用部署，再配置路由策略或模板。按业务需求选择权重、负载、延迟或费用策略；配置前确认每个候选部署具备相同请求能力。",
    body2:
      "路由模板可沿密钥、团队、组织继承，清除绑定后恢复继承。模板决定在已有权限内如何选择部署，不扩大模型访问范围。回退、超时、重试和冷却需一起检查。",
    body3:
      "跨模型回退只发生在响应字节输出前；请求设置 disable_fallbacks:true 可关闭。固定续传、原生异步任务创建和 Responses background 请求不会换模型重放。任务结果不明时保留任务 ID 与 x-litellm-call-id，不要盲目重发。",
  },
  "help-guardrails": {
    title: "护栏配置与验证",
    description: "检查、拦截与脱敏",
    heading1: "检查、拦截与脱敏",
    heading2: "操作步骤",
    heading3: "检查与边界",
    body1:
      "在护栏页面配置内置规则或自定义 XGo 检查，先保存草稿并调试正常与违规输入。规则覆盖当前实现的调用前文本检查；不要假定所有媒体内容或输出均已扫描。",
    body2:
      "核对规则执行阶段、作用范围与动作后再启用。拦截应阻止请求进入上游，脱敏应让上游接收替换后的正文，告警应保留可观察记录。调试成功后仍需在真实业务调用中验证绑定。",
    body3:
      "外部审核不可达时检查适配器配置与失败策略，并查看护栏日志。排查时避免把包含真实密钥或个人信息的输入复制到公开工单。",
  },
  "help-logs": {
    title: "用量、费用与调用排查",
    description: "用证据核对一次请求",
    heading1: "用证据核对一次请求",
    heading2: "操作步骤",
    heading3: "检查与边界",
    body1:
      "从调用日志查看模型、部署、状态、耗时和调用 ID；再进入费用明细核对输入输出用量与当次价格快照。不要用今天的价格重新推算历史请求。",
    body2:
      "按用户、密钥、团队或项目筛选费用，确认归属和时间范围。缓存输入是输入用量的一部分，图片、音频和视频使用各自计价单位；计算公式及示例见计费分类。",
    body3:
      "异步任务使用相同身份与凭据查询状态和结果，完成后的费用记录可能晚于创建响应。遇到差异时保留任务 ID、x-litellm-call-id、状态与计价快照；超时或连接中断不能证明供应商未执行或未计费，日志也不等同于供应商完整账单。",
  },
  "help-troubleshooting": {
    title: "常见问题与故障排查",
    description: "按响应定位问题",
    heading1: "按响应定位问题",
    heading2: "操作步骤",
    heading3: "检查与边界",
    body1:
      "401：检查是否使用 XHub 虚拟密钥、是否过期或被撤销。403：检查模型范围、团队关系和角色。400：核对 JSON、必填字段、公开模型名与接口协议；先保留返回正文。",
    body2:
      "429：检查 RPM、TPM、额度及上游限流。5xx：使用调用 ID 定位部署、上游错误和超时。浏览器网络失败但 curl 成功时检查网关地址、HTTPS 混合内容和跨域允许来源。",
    body3:
      "GET /models 是匿名部署发现，GET /v1/models 会应用调用方 XHub 密钥权限。工具连接失败时使用调试台显示的网关根地址，不追加 /v1，并核对工具自行拼接的路径。保留 x-litellm-call-id，媒体任务需区分创建、轮询与下载。",
  },
};
