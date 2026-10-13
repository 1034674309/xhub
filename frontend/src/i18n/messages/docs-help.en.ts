export const docsHelpEn = {
  "help-setup": {
    title: "Installation and first configuration",
    description: "From deployment to your first request",
    heading1: "From deployment to your first request",
    heading2: "Steps",
    heading3: "Checks and limits",
    body1:
      "Follow the repository installation guide to deploy the gateway, console and storage. Check both console and gateway access. Sign in, add provider credentials, then create a deployment. An empty model catalog normally means no deployment has been created.",
    body2:
      "Confirm the public model name and supported endpoints in the catalog. Test an English prompt in Playground. Once successful, create a minimally scoped XHub virtual key for your application. Keep provider credentials private.",
    body3:
      "Use the Base URL shown in Playground, keeping any reverse proxy prefix and without appending /v1. Requests use the complete endpoint path for the chosen protocol. Back up the database and deployment configuration before upgrading.",
  },
  "help-models": {
    title: "Providers and model deployments",
    description: "Configure NewAPI, develop Suno protocols and verify real calls",
    heading1: "Choose UI configuration or protocol development",
    heading2: "NewAPI: connection, deployment, key and call",
    heading3: "Suno: no built-in music task executor",
    body1:
      "Provider names are not protocols. If NewAPI implements OpenAI Chat, create a custom_openai credential in XHub and bind the deployment to the registered bypass_openai_chat; no provider directory is needed. Different authentication or paths need a Transport for the existing protocol; new request, response and task structures need protocol development. Prices, logos and credential forms do not establish execution support.",
    body2:
      "Add company-newapi on the Model providers page and enter its service address and NewAPI key in the provider form. In the model form, enter public name company-chat and an account-authorized upstream model, select company-newapi and OpenAI Chat, confirm pricing and save the deployment. Manage address and authentication only on the model provider; do not repeat them in deployments. Reload, create an XHub virtual key authorized for company-chat, and call /v1/chat/completions. Applications hold only XHub keys. Complete this in the UI without editing model_list.",
    body3:
      "There is no Suno executor or music endpoint today. Identify the actual authorized API, NewAPI plugin or third-party relay, then obtain creation/polling paths, authentication, task IDs, terminal states and billing units. Music JSON is not audio/speech, and custom_llm_provider=suno cannot install a protocol. Implement and test creation, original-identity polling, failures, multiple songs and deduplicated settlement.",
    detail: `## Step 1: save the provider connection

Open Model providers → Add provider. For this Chat example, choose Custom Openai, enter Provider name company-newapi, and fill API Base and API Key here. Leave Provider catalog ID blank for ordinary Chat models; it is a dedicated catalog identifier, not the connection name.

![English provider connection form](/assets/provider-guide/provider-form-en.jpg)

The address and masked key are fictional demonstration values. Save your own connection; this screenshot is not proof of a tested NewAPI account.

## Step 2: add the model deployment

Open Models + Endpoints → Add Model. Select the saved connection in step 1 to unlock the model section. Enter the upstream model ID and public name, choose the upstream protocol, then select catalog pricing or enter actual manual rates. Save and refresh to verify.

![English model deployment form](/assets/provider-guide/deployment-form-en.jpg)

This screenshot shows the initial unselected form. Disabled model fields mean you must select a provider first. Do not enter an address or key here or edit model_list.

## Source changes for a new provider

1. internal/provider/contract/type.go: check Action, AuthConfig, TaskID and TaskBilling against the real contract.
2. internal/provider/capability/endpoints.go: register a new media endpoint and compatibility.
3. internal/provider/<supplier>/: register Transport protocol, authentication and operation allowlist; see qiniu/seedance.go. Use RegisterModel for specialized catalog/TransportID bindings.
4. internal/provider/all/all.go: import the package, rebuild and restart. A directory alone is not loaded.
5. internal/dataplane/execution/official.go, transport and gateway: verify entry dispatch, native payloads, task ownership and errors; registration does not implement every execution behavior.
6. internal/provider/measurement and internal/catalog: separate successful facts from rates, preserve unpriced states and extend complex music pricing.
7. Model setup, Playground and bilingual API: add parameters, polling and results, plus unit tests, real backend regression and browser E2E.

## Checks after saving

- Discovery failures allow confirmed manual IDs; successful discovery does not prove inference. Verify non-streaming, streaming, tools and application parameters separately.
- API roots can retain /proxy/v1; do not enter a full operation URL. Discovery paths do not change inference protocols.
- Normal deployments reference model providers without repeating addresses or keys. Deployment-level api_base values take precedence; provider resolution fills empty addresses.
- Connection Test may incur charges and bypasses full quotas, routing and accounting. Make a virtual-key request and inspect usage and price snapshots.
- A creation timeout does not prove rejection. Do not blindly resend paid tasks. Generic background polling, budget reservation and Suno callbacks are not implemented.

The full bilingual UI guide, curl calls and source checklist are in docs/development/en/provider-catalog.md, under the website's Models & integration navigation.`,
  },
  "help-workspace": {
    title: "Organizations, teams and access",
    description: "Establish access boundaries",
    heading1: "Establish access boundaries",
    heading2: "Steps",
    heading3: "Checks and limits",
    body1:
      "Create organizations and teams, add members and assign roles. Configure team model access and budgets; use projects for business attribution. Grant only the management permissions members need.",
    body2:
      "Every API key belongs to a user; the team is derived from that user's membership. Check model scope, expiration and budget. Model scope cannot exceed parent permissions. See billing and quota guides for allocated limits, shared balances and unlimited settings.",
    body3:
      "On departure, revoke related keys, remove membership and check whether services still use personal credentials. Preserve historical usage for auditing.",
  },
  "help-playground": {
    title: "Playground and application integration",
    description: "Test before integrating",
    heading1: "Test before integrating",
    heading2: "Steps",
    heading3: "Checks and limits",
    body1:
      "Select an authorized public model and matching endpoint in Playground, then send a prompt or protocol body. Streaming, multimodal and file parameters depend on deployment capability.",
    body2:
      "After a successful test, copy the request example and store your application key in an environment variable. Use the same gateway Base URL. The tools category covers coding tools, chat clients and SDK setup.",
    body3:
      "API pages can run requests directly: enter a key, path and JSON, then inspect the HTTP status and response. Use multipart for files; select GET and the task path for polling. Real requests may incur costs and are not retried automatically.",
  },
  "help-routing": {
    title: "Routing and fallback",
    description: "Control deployment selection",
    heading1: "Control deployment selection",
    heading2: "Steps",
    heading3: "Checks and limits",
    body1:
      "Add usable deployments for a public model before selecting a routing strategy or template. Choose weights, load, latency or cost according to business needs. Confirm candidate deployments support the request.",
    body2:
      "Routing templates can inherit through key, team and organization scopes. Clearing a binding restores inheritance. Templates choose deployments within existing permissions and do not expand model access.",
    body3:
      "Cross-model fallback is available only before response bytes are emitted. Set disable_fallbacks:true to disable it. Pinned continuations, native asynchronous creation and Responses background requests are not replayed on another model. For unknown task outcomes, preserve the task ID and x-litellm-call-id instead of blindly resending.",
  },
  "help-guardrails": {
    title: "Guardrail configuration and validation",
    description: "Inspect, block and redact",
    heading1: "Inspect, block and redact",
    heading2: "Steps",
    heading3: "Checks and limits",
    body1:
      "Configure built-in rules or custom XGo checks on the guardrails page. Save a draft and test normal and violating inputs. Current rules cover pre-call text checks; do not assume every media input or output is scanned.",
    body2:
      "Check execution stage, scope and action before enabling a rule. Blocking stops upstream execution; redaction sends modified content; warning records the event. Verify bindings with a real application request.",
    body3:
      "When external moderation is unavailable, check adapter configuration and failure policy, then review guardrail logs. Keep real credentials and personal data out of public issue reports.",
  },
  "help-logs": {
    title: "Usage, costs and call diagnostics",
    description: "Review a request using evidence",
    heading1: "Review a request using evidence",
    heading2: "Steps",
    heading3: "Checks and limits",
    body1:
      "Inspect model, deployment, status, latency and call ID in call logs. Review usage and the recorded price snapshot in cost details. Do not recompute historical requests with current prices.",
    body2:
      "Filter costs by user, key, team or project and check attribution and time range. Cached input is part of input usage. Images, audio and video use their own pricing units; see billing formulas and examples.",
    body3:
      "Poll asynchronous tasks with the same identity and credentials. Completed task costs may appear after creation. Preserve task ID, x-litellm-call-id, status and pricing snapshot; a timeout or lost connection does not prove that the provider did no work or charged nothing, and logs are not a complete provider invoice.",
  },
  "help-troubleshooting": {
    title: "Common problems and troubleshooting",
    description: "Diagnose using the response",
    heading1: "Diagnose using the response",
    heading2: "Steps",
    heading3: "Checks and limits",
    body1:
      "401: check that the key is an active XHub virtual key. 403: inspect model scope, team membership and role. 400: check JSON, required fields, public model name and endpoint protocol; preserve the response body.",
    body2:
      "429: inspect RPM, TPM, quota and upstream rate limits. 5xx: use the call ID to locate deployment, upstream errors and timeouts. If curl works but browser requests fail, check gateway URL, HTTPS mixed content and CORS origins.",
    body3:
      "GET /models is anonymous deployment discovery; GET /v1/models applies the caller's XHub key permissions. For tool failures, use the Playground gateway root without appending /v1 and inspect paths added by the tool. Preserve x-litellm-call-id, and distinguish media creation, polling and download.",
  },
};
