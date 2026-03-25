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

## Environment

See [`.env.example`](/Users/linlay/Project/zenmind/agent-weixin-bridge/.env.example).

Key variables:

- `BRIDGE_HTTP_ADDR`: bridge management API listen address. Default `:11958`.
- `RUNNER_BASE_URL`: external runner base URL. Keep it independent from the bridge port.
- `STATE_DIR`: bridge state directory used by the Go process.
- `HOST_STATE_DIR`: docker compose host mount for Weixin state files such as `credential.json`, `status.json`, and user context files.
- `AUTO_START_POLL`: when `true`, restart will resume polling if the saved Weixin credential is still valid.

`RUNNER_AGENT_KEY` is required. Shell environment variables take precedence over values in `.env`.

## Local Run

Start the bridge:

```bash
go run ./cmd/bridge
```

Management API now defaults to `:11958`.

Open the built-in console:

```text
http://127.0.0.1:11958/
```

Daily browser workflow:

1. Start the bridge.
2. Open `http://127.0.0.1:11958/`.
3. Click `生成二维码`.
4. Scan the QR code with Weixin and confirm on your phone.
5. Wait until the page shows `confirmed` / `已连接`.
6. Polling starts automatically when `AUTO_START_POLL=true`; otherwise click `开始轮询`.

The built-in page renders QR codes directly from `login/start.qrCodeUrl`:

- data URLs and base64 payloads render as images
- direct image URLs render as images
- page-style QR URLs such as `https://liteapp.weixin.qq.com/q/...` render as a locally generated QR code

## Container Deployment

Build and start:

```bash
docker compose up -d --build
```

Stop:

```bash
docker compose down
```

Before running `docker compose`, set these values in [`.env`](/Users/linlay/Project/zenmind/agent-weixin-bridge/.env):

- `BRIDGE_HTTP_ADDR=:11958`
- `HOST_STATE_DIR=./runtime/weixin-state`
- `RUNNER_BASE_URL=http://host.docker.internal:11949` when the runner is running on the host machine

`compose.yml` mounts `${HOST_STATE_DIR}` into `/app/var/state` and forces `STATE_DIR=/app/var/state` inside the container. This keeps Weixin session files on the host, so after the first successful login:

- `credential.json` persists on the host
- container restart can reuse the saved credential
- re-scan is only needed if the Weixin credential itself has expired

`compose.yml` also exposes the bridge on:

```text
http://127.0.0.1:11958/
```

For Linux hosts, `compose.yml` includes `host.docker.internal:host-gateway` so the container can still reach a host-side runner.

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
BASE="http://127.0.0.1:11958"
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
