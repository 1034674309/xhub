.PHONY: test run ui tidy testdata e2e e2e-list e2e-offline e2e-all e2e-model-endpoints regression regression-live

# make 的第二个词本来是另一个目标；仅在 e2e 后把白名单内的词解释为分类。
E2E_CATEGORIES := entities gpt-1 gpt-2 gpt-3 gpt-4 glm image ark-video fal-video kling-video inheritance chat-bills permissions xgo limits model-lifecycle guard-lifecycle global-guards profile-isolation cache-weight Affinity fallback media-logs database browser business-browser regression
ifneq ($(filter e2e,$(MAKECMDGOALS)),)
E2E_SELECTED := $(filter-out e2e,$(MAKECMDGOALS))
ifneq ($(filter-out $(E2E_CATEGORIES),$(E2E_SELECTED)),)
$(error Unknown E2E category: $(filter-out $(E2E_CATEGORIES),$(E2E_SELECTED)); run make e2e-list)
endif
.PHONY: $(E2E_SELECTED)
$(E2E_SELECTED):
	@:
endif

test:
	go test ./...

# End-to-end regression suite: real gateway, real PostgreSQL, real budgets,
# guardrails and bypass. Uses a fake provider, so it is free to run repeatedly.
regression:
	./scripts/regression.sh -v

# The same suite with the live vendor calls enabled. Spends real money and needs
# XHUB_REGRESSION_<ID>_KEY in the environment.
regression-live:
	python3 scripts/with-live-vendors.py bash scripts/regression.sh -v --live

# 全量 live 回归，终端按用例实时打印。密钥从 FENNO_AI_API_KEY / QINIU_API_KEY 读取。
regression-live-log:
	bash scripts/regression-live-log.sh

run:
	go run ./cmd/gateway -config configs/config.yaml -addr :4000

ui:
	cd frontend && npm install && npm run dev

tidy:
	go mod tidy

# 自启空闲端口临时网关：清空共享 xhub 和 Redis DB 1 后建数，结束关闭网关并保留数据，不调用模型。
testdata:
	python3 scripts/e2e.py --shared-target --phase seed --directory .e2e/real-acceptance-current

# 默认媒体并发 4（E2E_JOBS=1 可串行）；复用 E2E_GATEWAY（默认 localhost:4000）和现有前端；不启动网关、不改写供应商地址；隔离 regression 另行执行。
e2e:
	python3 scripts/e2e.py --shared-target --phase verify $(foreach category,$(E2E_SELECTED),--category $(category)) --directory .e2e/real-acceptance-current

# 显示每类验收目的、使用示例、共享基线前提、费用边界与报告位置；只显示帮助，无服务或数据副作用。
e2e-list:
	@python3 scripts/e2e.py --list-categories

# 离线浏览器入口使用独立 schema、本地协议夹具和自动清理，不复用共享验收数据或调用付费模型。
# E2E_TESTS 可选指定 Playwright 用例路径；为空时执行完整离线套件。
e2e-offline:
	bash scripts/e2e.sh $(E2E_TESTS)

# 原有全套隔离验收保留为显式入口。
e2e-all:
	bash scripts/e2e-all.sh

e2e-model-endpoints:
	bash scripts/e2e-model-endpoints.sh
