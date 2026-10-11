# Fal queue tasks

## Behavior
Only registered Fal video paths are supported: Seedance, Kling, Vidu, Veo and MiniMax. Resolve returned status_url and response_url against the gateway root and query with the same XHub key used to create the task. Do not append /requests/ to the creation path.

## Query queues
| Family | Query prefix |
| --- | --- |
| Doubao Seedance 2.0 / 2.5 | /queue/bytedance/seedance-2.0/requests/ · /queue/bytedance/seedance-2.5/requests/ |
| Dreamina Seedance 2.0 / 2.5 | /queue/byteplus/seedance-2.0/requests/ · /queue/byteplus/seedance-2.5/requests/ |
| Kling | /queue/fal-ai/kling-video/requests/ |
| Vidu | /queue/fal-ai/vidu/requests/ |
| Veo 3.1 | /queue/fal-ai/veo3.1/requests/ |
| MiniMax H3 / H3 Max | /queue/minimax/h3/requests/ · /queue/minimax/h3-max/requests/ |

GET prefix + {request_id} retrieves results; GET prefix + {request_id}/status retrieves status. Fast/Mini share the version queue. When one path has multiple public aliases, add model to the creation body to select one; the gateway removes it before forwarding.

## Retries and settlement
Cancellation is not implemented and cancel_url is empty. Network errors and 5xx during creation are not automatically replayed, avoiding duplicate paid tasks; 429 follows routing policy. Forwarded webhooks do not trigger settlement: query terminal success with measured usage. Exact creation modes, fields and billing variants depend on deployment endpoint details; no arbitrary wildcard Fal proxy.
