# usage: Usage normalization, missing-count estimation and first-token timing

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

Extract usage with UsageFromDocument/UsageCounts. Use CompleteUsage only to fill missing statistics, preserving explicit zero values, then pass it to catalog for normalization and pricing.

| Entry point or resource | Usage and result |
| --- | --- |
| UsageCounts(usage) | Return input/output token counts |
| CompleteUsage / MergeUsage | Fill or merge maps, potentially mutating them |
| TtftMillis(duration) | Return optional milliseconds, with positive values at least 1ms |

### Integration demo

Save the code as `demo/main.go` inside the repository and run `go run ./demo` from the repository root. Remove the demo directory after use.

```go
package main

import (
	"fmt"
	"github.com/sunqirui1987/xhub/internal/dataplane/usage"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	input, output := usage.UsageCounts(map[string]any{"input_tokens": 100, "output_tokens": 20})
	fmt.Println(input, output)
}
```

Prints 100 20 without selecting rates or writing accounts.

### Errors, state, and side effects

Token estimates are neither guaranteed upper bounds nor measured usage. MergeUsage mutates its destination; copy first if the input must be retained.

## Responsibility

Usage normalization, missing-count estimation and first-token timing. Distinguish explicit zero from absence; estimates are not upstream measurements and do not define prices.

## Review order

usage.go: UsageCounts / UsageFromDocument / MergeUsage → CompleteUsage / TtftMillis

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

stream, dialogue and execution consume pure helpers; catalog and gateway/billing own prices.

## Verification

Adjacent tests: metrics_test.go, usage_stream_test.go

From repository root: go test ./internal/dataplane/usage -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
