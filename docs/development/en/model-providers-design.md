# Provider protocol design

[简体中文](../zh-CN/model-providers-design.md) · [Developer documentation](README.md)

Separate authentication, discovery, adaptation, capabilities, and native transport. llm.Build constructs without sending. Multipart/binary/WebSocket/tasks need dedicated implementations. Validate errors, terminal usage, units, and explicit zero. Simplified Bedrock authentication is not full SigV4.

See [testing](testing.md) for isolation, commands, evidence, and external boundaries.
