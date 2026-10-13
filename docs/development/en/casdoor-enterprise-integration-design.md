# Casdoor integration boundaries

The current enterprise identity interfaces are OIDC Code + PKCE and SCIM user/group provisioning. There is no Casdoor directory polling, dedicated QR login executor or department snapshot reconciliation.

Verify that the target instance supplies issuer, discovery, JWKS and token endpoints satisfying XHub validation, and provisions users through SCIM. OIDC binds provisioned identities; it does not create or merge accounts by email. Users must meet current directory field requirements. Match the stable identity claim to SCIM externalId and use the exact generated callback URL.

Group mappings target teams in the directory organization. Source deactivation or deletion must actually reach XHub and be verified; propagation cannot be assumed.

See [enterprise identity and archive](enterprise-identity-log-archive-design.md) and [verification](enterprise-directory-validation.md). Real Casdoor releases, permissions and provisioning behavior require target-instance validation.
