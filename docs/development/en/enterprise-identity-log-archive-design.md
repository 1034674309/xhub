# Enterprise identity and Kodo archive

Directory synchronization, OIDC login and the Kodo storage adapter have local implementations. Real tenant/account acceptance remains separate. The [full design record](../zh-CN/enterprise-identity-log-archive-design.md) distinguishes current implementation from future targets.

## Connect in this order

1. Deploy a public HTTPS origin and a stable server master key of at least 32 characters.
2. In the administrator panel, create an organization-bound directory. Save the one-time SCIM credential; register the generated connection URL in the identity platform.
3. Provision test users and groups first. Map groups to teams in the same organization; do not infer priority from incoming group order.
4. Register an OIDC Code + PKCE application and the exact generated callback. Align the stable identity claim with SCIM externalId. Save disabled, then enable and test login.
5. Configure a private Kodo bucket, region, dedicated credentials and prefix. Save disabled; test upload and cleanup before enabling capture.

~~~text
SCIM user/group -> stable externalId -> OIDC identity -> effective team
committed billing -> versioned outbox -> JSONL.gz object + manifest
~~~

## Current boundaries

| Area | Boundary |
| --- | --- |
| Login | No unsynchronized account creation or email-based identity merge |
| Team | Manual override first, then valid current team, then ordered mapping |
| SCIM | Limited filters and attributes; no Bulk or nested groups |
| Archive | One billing event per object, metadata only, PostgreSQL remains authoritative |
| Planned | Rotation UI, reassignment preview, backfill, multi-line batches and forced private-bucket verification |

These pages do not change the enterprise implementation or claim real Entra/Okta/Qiniu verification. [Validation evidence](enterprise-directory-validation.md).
