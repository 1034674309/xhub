# Runtime logging and secret redaction

## Overview

The logx package is the process-wide diagnostic logging boundary. It adds one of four textual levels to standard-library log output and redacts common credential and URL forms after formatting. Gateway, data plane, IAM, routing, catalog, store, and provider code use it. These lines are operational diagnostics; durable audit records and spend logs are separate paths.

## External interfaces

`Trace`, `Debug`, `Info`, and `Error` accept a fmt-style format string and arguments and return nothing. Trace marks detailed execution, Debug records diagnostic decisions, Info records normal outcomes, and Error records failures without terminating the process. Each call formats, redacts, and sends one line to Go's standard logger with a lowercase level prefix. There is no HTTP response, returned error, level filter, logger injection, or persistence API.

## Core implementation

The private writer applies three filters. Case-insensitive Bearer values become asterisks; tokens matching the `sk-` family become asterisks; HTTP and HTTPS URLs are parsed and reduced to their hostname, dropping scheme, path, query, user information, and port. If parsing cannot recover a hostname, the whole match is hidden. Common trailing punctuation is excluded while parsing.

Redaction is a pattern-based last defense, not a complete secret classifier. Arbitrary credentials, prompts, environment values, or non-HTTP connection strings can remain visible, so callers must log identifiers and stages rather than raw secrets or payloads. Formatting precedes redaction; standard logger settings control timestamps and destination.

### Test dependencies

`files_test.go` checks the expected logging surface and bearer, key, mixed-case, and URL redaction. Run `go test ./internal/util -count=1`; it reads repository source files but requires no service.

## Source responsibilities and entry points

- `log.go` is the only production file. Public entry points are Trace, Debug, Info, and Error; private writing and redaction centralize formatting and masking.
- `files_test.go` enforces level availability and the current redaction boundary.
