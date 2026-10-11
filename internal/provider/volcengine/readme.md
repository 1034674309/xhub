# VolcEngine Ark Seedance registration

[简体中文](readme_cn.md)

## Overview

This package declares VolcEngine as a supplier and registers Ark native asynchronous content generation. Administrators still provide the base URL, Bearer credential, and deployment prices. Two Seedance models are catalog conveniences.

### Dependencies and tests

It uses provider registration and shared Seedance billing. Tests verify supplier/model registration, prefix removal, paths, and binding without live requests.

## External interfaces

There is no exported API or handler. Init makes provider.Match recognize POST /api/v3/contents/generations/tasks, GET the collection for list, and GET /api/v3/contents/generations/tasks/{id}; dataplane native forwarding performs authentication, pinning, and settlement.

## Core implementation

ark_contents_generation uses endpoint bypass:ark-video, protocol ark, family video, Bearer auth, model field model, task field id, and strips volcengine/. Create, list, and get are separate whitelist actions. SeedanceBilling accepts only explicit successful terminal usage. Stored model IDs map to official Ark names; pricing is left to deployments.

## Source responsibilities and entry points

- [seedance.go](seedance.go): supplier, transport, three actions, two model bindings, auth/prefix/task metadata, and billing.

Verification: go test ./internal/provider/volcengine ./internal/provider -count=1. Live account access, quota, schema, and prices remain external.
