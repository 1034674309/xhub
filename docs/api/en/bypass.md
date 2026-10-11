# Native protocol Bypass

Choose an endpoint to read authentication, parameters, examples and responses, then open its online runner. Each endpoint has its own Markdown source. Only registered paths and permitted deployments can be called.

## Native endpoints

| Method | Endpoint | Path |
| --- | --- | --- |
| POST | [Bypass Anthropic Messages](native-anthropic.md) | /bypass/anthropic/v1/messages |
| POST | [Bypass Vertex generateContent](native-vertex.md) | /bypass/vertex/v1/models/YOUR_MODEL_NAME:generateContent |
| POST | [Bypass Gemini generateContent](native-gemini.md) | /bypass/gemini/v1beta/models/YOUR_MODEL_NAME:generateContent |
| POST | [Bypass OpenAI image generation](native-images.md) | /bypass/openai/v1/images/generations |
| POST | [Bypass OpenAI image edits](native-image-edits.md) | /bypass/openai/v1/images/edits |
| POST | [Bypass OpenAI Responses](native-responses.md) | /bypass/openai/v1/responses |
| POST | [Bypass OpenAI Chat Completions](native-chat.md) | /bypass/openai/v1/chat/completions |

## Capability and protocol selection

Bypass is a transport mechanism, not a model family. Select a deployment supporting the registered protocol.

| Capability | Guide |
| --- | --- |
| Chat | [OpenAI](chat.md), [Anthropic](messages.md), [Gemini](gemini.md) |
| Images | [Generation](images.md), [Edits](image-edits.md) |
| Video | [OpenAI](videos.md), [Content generation](ark.md), [Seedance Fal](fal-seedance.md) |
