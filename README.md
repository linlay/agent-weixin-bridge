# agent-weixin-bridge

Go 1.26 single-account Weixin bridge for `agent-platform-runner`.

## Capabilities

- QR-code login against the Weixin upstream API
- Long-poll message ingestion with `get_updates_buf` persistence
- Streaming bridge from Weixin text messages to runner `/api/query` Event Model v2
- Incremental message flush back to Weixin
- Typing indicator keepalive while the runner is generating
- Persistent UUID `chatId` mapping per Weixin user for runner chat continuity

## Current Scope

- Single Weixin account bridge
- Fixed `RUNNER_AGENT_KEY` routing to one runner agent
- Text-only bridge between Weixin and runner
- No Weixin-side support yet for runner frontend tools, `submit`, `steer`, or `interrupt`

## Run

```bash
go run ./cmd/bridge
```

Management API defaults to `:8091`.
The bridge automatically loads `.env` from the current working directory when starting.

## Browser Workflow

1. Start the bridge:

```bash
go run ./cmd/bridge
```

2. Open the built-in console in your browser:

```text
http://127.0.0.1:8091/
```

3. Click `生成二维码`.
4. Scan the QR code with Weixin and confirm on your phone.
5. Wait until the page shows `confirmed` / `已连接`.
6. Click `开始轮询` to start receiving inbound messages.

The built-in page now renders QR codes from `login/start.qrCodeUrl` directly in the browser:

- data URLs and base64 image payloads are shown as images
- direct image URLs are loaded as images
- page-style QR URLs (such as `https://liteapp.weixin.qq.com/q/...`) are converted into a local QR code on the page

The bridge QR-image endpoint remains available for compatibility and debugging of image-based QR payloads, but the browser workflow no longer depends on it.

The built-in page also shows:

- bridge health status
- current Weixin account status
- polling status
- recent inbound / outbound timestamps
- last error
- raw JSON responses for `login/start`, `login/status`, and `account`

## Environment

See `.env.example`.

`RUNNER_AGENT_KEY` is required. The bridge always sends Weixin messages to that configured runner agent instead of relying on the runner default agent.
Shell environment variables take precedence over values in `.env`, so `.env` acts as a local default for development.

## Management Endpoints

- `GET /healthz`
- `GET /`
- `POST /api/weixin/login/start`
- `GET /api/weixin/login/qrcode?sessionKey=...`
- `GET /api/weixin/login/status?sessionKey=...`
- `GET /api/weixin/account`
- `POST /api/weixin/poll/start`
- `POST /api/weixin/poll/stop`

## API Debug With curl

Set the base URL first:

```bash
BASE="http://127.0.0.1:8091"
```

Health check:

```bash
curl -sS "$BASE/healthz"
```

Start a Weixin login session:

```bash
curl -sS -X POST "$BASE/api/weixin/login/start"
```

Fetch the QR image through the bridge when the upstream returns actual image content:

```bash
SESSION_KEY="replace-with-session-key"
curl -sS "$BASE/api/weixin/login/qrcode?sessionKey=$SESSION_KEY" --output weixin-qrcode.png
```

Check login status:

```bash
SESSION_KEY="replace-with-session-key"
curl -sS "$BASE/api/weixin/login/status?sessionKey=$SESSION_KEY"
```

Check account status:

```bash
curl -sS "$BASE/api/weixin/account"
```

Start polling:

```bash
curl -sS -X POST "$BASE/api/weixin/poll/start"
```

Stop polling:

```bash
curl -sS -X POST "$BASE/api/weixin/poll/stop"
```

Recommended usage:

- use `http://127.0.0.1:8091/` for daily login and operations
- use `curl` for debugging, automation, and API verification
# agent-weixin-bridge
