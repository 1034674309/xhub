# dialogue: Unified dialogue conversion, continuation and adapted SSE

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Responsibility

Unified dialogue conversion, continuation and adapted SSE. Convert supported text and function tools; fail explicitly on unsupported capabilities, retaining native capabilities through bypass.

## Review order

dialogue.go: BuildDialogue, ResponsesRequestBody, ProbeDialogue → dialogue_stream.go: PipeDialogue

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

llm owns protocol values and official SDK types; provider declares transports; transport handles native forwarding.

## Verification

Adjacent tests: dialogue_probe_test.go, dialogue_stream_test.go, registered_operation_test.go, responses_continuation_test.go, sdk_stream_test.go

From repository root: go test ./internal/dataplane/dialogue -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
