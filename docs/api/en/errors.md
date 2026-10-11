# Errors and troubleshooting

## Overview

Diagnose issues by status code and call ID.

### Request and permission errors

For 400, inspect JSON, model and parameters. For 401, check credentials. For 403, check role, model scope and ownership. Some unauthorized calls return 404 to hide existence.

### Rate limits and upstream failures

For 429, inspect Retry-After and rate settings. For 5xx, check dependencies, provider connectivity, deployments and logs. Use backoff and bounded retries.

### Keep diagnostic details

Record time, model, status and call ID. Remove keys, personal information and sensitive content before sharing. Also inspect streaming event errors.

[Chat request](chat.md) · [Native paths](bypass.md)
