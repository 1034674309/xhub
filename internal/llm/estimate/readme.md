# Local token and cost estimation helpers

[简体中文](readme_cn.md)

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

Use these functions for admission estimates and utility endpoints. Settle completed calls using reported vendor usage and catalog rate snapshots.

| Entry point or resource | Usage and result |
| --- | --- |
| CountTokens(model, prompt, messages) | Return a count, counting model, and error |
| TokenCost(prompt, completion, inputRate, outputRate) | Multiply token counts by per-token rates |
| NormalizeServiceTier(value, isString) | Normalize recognized service tiers |

### Integration demo

Save the code as `demo/main.go` inside the repository and run `go run ./demo` from the repository root. Remove the demo directory after use.

```go
package main

import (
	"fmt"
	"github.com/sunqirui1987/xhub/internal/llm/estimate"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	input, output := estimate.TokenCost(100, 20, 0.000001, 0.000002)
	fmt.Printf("%.6f %.6f\n", input, output)
	tier, known := estimate.NormalizeServiceTier("auto", true)
	fmt.Printf("%q %t\n", tier, known)
}
```

Prints 0.000100 0.000040 and "" false. auto is not an observed billing tier.

### Errors, state, and side effects

Estimates are not supplier billing facts. Rate units must match the input quantities; preserve unknown tier status.

## Overview

This package provides deterministic helpers for previewing token counts, token-based cost, discounts, margins, and service tiers. Rates are dollars per token. Results support admission, UI estimates, and LiteLLM-compatible token-count responses; they are not authoritative provider settlement evidence.

### Dependencies and tests

It uses the configured tokenizer library and logging. Tests cover discount/margin precedence, fixed additions, token arithmetic, tier mapping, and invalid boundaries. No network is used.

## External interfaces

There is no HTTP registration. Gateway token-count and pricing code calls these functions. ApplyDiscount returns final cost, percent, and amount; provider-specific entries override global and invalid/missing values leave cost unchanged. ApplyMargin returns final, percent, fixed, and total margin with the same precedence. TokenCost returns input and output components and clamps negative token counts to zero.

MapTrafficType maps Gemini traffic classes to billing tiers and reports whether the value is known/standard. NormalizeServiceTier accepts valid settled tiers and rejects missing/non-string/auto as a final tier. CountTokens(model, prompt, messages) returns count, tokenizer model, and error. OpenAISupportedParams returns a fresh supported-key list, varying response_format and user by model/catalog context. ModelUsedForCount removes one provider prefix from the deployment model.

## Core implementation

Discounts apply a bounded percentage to base cost. Margins combine percentage and fixed components after choosing provider-specific then global configuration. Token cost multiplies normalized counts by per-token rates without rounding away components. Traffic normalization keeps unknown values distinguishable from standard billing and prevents auto from being recorded as a settled tier.

CountTokens follows LiteLLM/OpenAI chat accounting: encode prompt or message fields, add per-message framing, include names and structured content through stable text conversion, then add assistant priming. It chooses a compatible tokenizer and returns an error when encoding cannot be established. It cannot observe hidden reasoning, provider retokenization, binary media, or future tokenizer changes.

## Source responsibilities and entry points

- [cost.go](cost.go): Margin, discount/margin selection and arithmetic, TokenCost, Gemini traffic mapping, and service-tier normalization.
- [tokencount.go](tokencount.go): CountTokens chat accounting, OpenAISupportedParams, and ModelUsedForCount provider-prefix normalization.

Verification: go test ./internal/llm/estimate -count=1. Live provider tokenization, multimodal units, hidden tokens, and final invoices remain external.
