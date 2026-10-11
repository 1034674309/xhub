# Quota enforcement

[简体中文](../zh-CN/quota-chain.md) · [Developer documentation](README.md)

Amount/RPM/TPM allocate organization → team → person → key. Projects add ceilings, not pools. Numeric limits reserve; null shares remaining capacity; zero disables. Validate writes transactionally, authorization, over-allocation, service-account ownership, inference rejection, and historical records. Routing inheritance differs from quota inheritance.

See [testing](testing.md) for isolation, commands, evidence, and external boundaries.
