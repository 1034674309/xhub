# requestlog: Content encoding, streamed-log reconstruction and secret redaction

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Responsibility

Content encoding, streamed-log reconstruction and secret redaction. No pricing or upstream calls; log reconstruction must not change responses; raw and billing usage remain comparable.

## Review order

encode.go: Encode → assembleLoggedResponse / redactHeaders → diagnostics.go: DiagnosticJSON

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

inputs are values/facts; gateway/spend.go owns lifecycle/persistence.

## Verification

Adjacent tests: encode_test.go, prompt_response_test.go

From repository root: go test ./internal/gateway/requestlog -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
