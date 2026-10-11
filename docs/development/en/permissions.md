# Authorization and visibility

The gateway authorizes every request using current identity and membership. UI visibility is not the security boundary.

## Example

~~~bash
go test ./internal/authz -count=1
go test ./internal/gateway -run 'TestVisibilityChain|TestPermission' -count=1
~~~

## Contract

| Scope | Rule |
| --- | --- |
| Platform / organization / team roles | Distinct management roles, not interchangeable |
| List queries | Restrict in the query layer; client filters only narrow scope |
| Personal key | Owned by the person; team attribution must be valid |
| Service key | Uses its declared service scope, not another member's ownership |
| Model access | Narrow along the identity chain |
| Routing template | Cannot expand model scope or bypass budget checks |
| History | Keep identity at call time after membership/key changes |

Treat missing scope separately from global scope. Read failures cannot become an allow decision.

## Source and verification

[internal/authz/decide.go](../../../internal/authz/decide.go)

~~~bash
go test ./internal/authz -count=1
~~~

[Test prerequisites](testing.md) · [Next: quota-chain](quota-chain.md)
