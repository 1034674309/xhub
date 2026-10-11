# Overflow-safe pagination

## Overview

The pagination package converts one-based page numbers into database offsets and safe in-memory slice bounds. Its guarantee is that invalid or extremely large input cannot wrap an integer and accidentally select the first page or produce an invalid interval. It has no configuration, state, I/O, or HTTP routes.

## External interfaces

- `Offset(page, size int) int` returns a non-negative database offset. A page at or below one, or a non-positive size, produces zero. If the multiplication would overflow int, it returns the maximum int, preserving an out-of-range position instead of wrapping toward zero. Gateway codecs and list handlers use it before database queries.
- `Bounds(total, page, size int) (start, end int)` returns a half-open interval for an in-memory list. Non-positive totals or sizes return an empty interval; pages below one become page one; a page beyond the data returns total for both bounds; otherwise the end is capped at total. Callers can safely slice when total equals the list length.

Neither function returns an error or mutates input. Callers still normalize public maximum page sizes and ensure total describes the list being sliced.

## Core implementation

Offset derives the largest int, then compares page minus one with max divided by size before multiplying. Bounds compares page minus one with total minus one divided by size; multiplication occurs only after that check proves it safe. It compares size with the remaining count before adding the end. These staged checks are the overflow boundary: no unchecked user-controlled multiplication occurs.

### Test dependencies

page_test.go covers ordinary pages, invalid values, out-of-range pages, and overflow-sized input. Run `go test ./internal/util -count=1`; no external service is required.

## Source responsibilities and entry points

- page.go is the complete production package. Offset is the SQL-pagination entry point; Bounds is the in-memory slicing entry point.
- page_test.go verifies normalization, half-open intervals, and overflow behavior.
