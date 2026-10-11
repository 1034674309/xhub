# HTTP response, validation, and module contracts

## Overview

The httpx package centralizes three gateway boundaries: JSON and protocol-specific error responses, numeric budget/rate request validation, and feature-module route registration. It does not authenticate, authorize, persist values, or mount routes by itself. Gateway handlers validate decoded JSON and write responses through it; feature packages bind modules that the gateway route registry later mounts.

## External interfaces

### Responses and call identity

- `CallID() string` generates 16 random bytes and returns 32 hexadecimal characters. Random-source errors are ignored, so an entropy-source failure can yield a weak all-zero/partial identifier; callers use it for correlation, not authorization.
- `SetCallID(w, id)` sets `x-litellm-call-id` and overwrites an existing value.
- `WriteJSON(w, status, value)` ensures call-ID and `x-litellm-version: xhub-dev` headers, preserves an existing Content-Type, writes the status, then JSON-encodes with a trailing newline. Encoding errors are discarded after headers have been committed.
- `WriteError` delegates to `WriteTypedError` with no path. `WriteTypedError` always sets JSON content type, correlation/version headers, and `Retry-After: 1` for 429. It selects Anthropic shape for non-chat/non-thread message paths, native Gemini shape for generate/stream-generate/count-token paths, and OpenAI shape otherwise. Google RPC names map 400/401/403/404/429/502/503 and other 5xx explicitly; other statuses become INVALID_ARGUMENT.

### Budget and rate fields

- `ValidateBudgetFields(body, fields...)` accepts absent, null, and empty-string values as shared/unset; accepts finite non-negative float64 values or numeric strings; and returns a field-specific error for other types, parse failures, NaN, infinity, or negatives.
- `ValidateRateFields(body)` applies that validation to rpm_limit and tpm_limit, then requires an integer no greater than 2,147,483,647.
- `RateValue(raw)` parses float64 or string input. `OptionalRate` distinguishes absent fields (nil outer pointer) from explicit clear (non-nil outer pointer containing nil) and numeric update. `Rate` collapses absence and clear to nil for create operations. Parsing helpers assume validation already succeeded; callers must validate first.

### Module registration

`Registrar.Handle` registers method-and-path patterns. `Module` supplies a stable name and a mount operation. `Bind(name, mount)` creates a module even for an empty name; the process registry rejects invalid/duplicate names. Bound Mount does nothing when either callback or registrar is nil, otherwise it invokes the callback synchronously.

## Core implementation

Response writers commit headers before encoding, so later failures cannot change status. Path matching lowercases and uses substring rules, making the supplied registered request path part of the protocol contract. Validation first handles JSON-decoder float64 values and numeric strings, then performs finiteness/range checks before converting to int. Module binding stores only name and closure, avoiding a dependency from feature packages to the gateway process.

### Test dependencies

`httpx_test.go` verifies protocol envelopes and header preservation; `budget_test.go` verifies budget input failures and clears; `rates_test.go` verifies rate fractions, bounds, and pointer semantics. Run `go test ./internal/util -count=1`; no external service is required.

## Source responsibilities and entry points

- `httpx.go` owns call IDs, common response headers, JSON output, protocol detection, and OpenAI/Anthropic/Gemini error envelopes.
- `budget.go` owns budget/rate validation and conversion, including patch-field three-state semantics.
- `module.go` defines Registrar, Module, Bind, and the private bound adapter used by all feature mount files.
- `httpx_test.go`, `budget_test.go`, and `rates_test.go` verify the three production areas.
