# Qiniu Fal and content-generation catalog

[简体中文](readme_cn.md)

## Overview

This package registers Qiniu-compatible Fal queue transports and the Qiniu /v3 content-generation transport. Connections are administrator supplied through custom/OpenAI-compatible deployments; registration creates no account and reads no credential. Model rows keep upstream IDs, pricing IDs, sources, transport IDs, and billing dimensions separate.

### Dependencies and tests

It uses provider registration and shared Fal/Seedance billing. Tests cover exact paths, auth declarations, prefix handling, measured units, success boundaries, variants, and incomplete-context pricing blocks.

## External interfaces

There is no exported function or local router. Fal exposes exact POST /queue/{model-path}, GET /queue/{group}/requests/{request_id}, and GET .../status actions. Content generation exposes POST /v3/contents/generations/tasks and GET /v3/contents/generations/tasks/{id}, with no list action. provider.Match dispatches them to dataplane native handling.

## Core implementation

registerFal creates one transport per queue family, two shared follow-up actions, and one create action per concrete model path. It uses Authorization: Key, request_id, strips qiniu/, rewrites queue URLs, and binds FalBilling to resolution/audio/variant rules. Init explicitly enumerates Seedance/Dreamina, Kling, Vidu, Veo, and MiniMax variants, so no wildcard expands the whitelist.

The content transport uses Bearer auth, task field id, prefix stripping, and SeedanceBilling. Four bytedance Seedance models carry distinct marketplace pricing IDs and sources. Settlement later requires explicit terminal success and positive measured usage.

## Source responsibilities and entry points

- [fal.go](fal.go): falEndpoint, registerFal, concrete queue actions/models, catalog rows, Key auth, URL rewriting, and Fal billing bindings.
- [seedance.go](seedance.go): qiniu_contents_generation create/get, four models, Bearer auth, prefix handling, and Seedance billing.

Verification: go test ./internal/provider/qiniu ./internal/provider -count=1. Availability, quota, duration, and price freshness remain external.
