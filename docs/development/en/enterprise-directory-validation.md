# Enterprise directory validation record

This is an implementation record, not a production-provider certification. [简体中文](../zh-CN/enterprise-directory-validation.md) documents current verification entries.

## Local coverage

| Layer | Coverage |
| --- | --- |
| Backend | Isolated PostgreSQL, SCIM lifecycle/PATCH rollback, TLS OIDC discovery/JWKS/token validation, Kodo SDK requests and archive outbox |
| Components | Directory, SSO and archive forms; persistence, errors and both languages |
| Browser | Organization/team selection, generated addresses, saved configuration, secret clearing, disabled state and retry |

No real enterprise tenant or Qiniu account was used. Local protocol fixtures do not establish external interoperability. See [current configuration boundaries](enterprise-identity-log-archive-design.md).
