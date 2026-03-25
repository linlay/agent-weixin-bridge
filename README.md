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

See `.env.example`.

Key variables:

- `BRIDGE_HTTP_ADDR`: local development listen address. Default `:11958`.
- `RUNNER_BASE_URL`: runner base URL. Recommended `http://agent-platform-runner:8080` when bridge and runner share `zenmind-network`.
- `RUNNER_AGENT_KEY`: required runner routing key.
- `RUNNER_BEARER_TOKEN`: optional bearer token for runner auth.
- `HOST_STATE_DIR`: docker/deploy host directory for persistent Weixin session and bridge state.
- `AUTO_START_POLL`: when `true`, bridge restart will resume polling if the saved Weixin credential is still valid.

Shell environment variables take precedence over values in `.env`.

## Local Development

Run tests:

```bash
make test
```

Start the bridge locally:

```bash
make run
```

This starts the management API on:

```text
http://127.0.0.1:11958/
```

If your runner is also local, set `RUNNER_BASE_URL=http://127.0.0.1:11949` in `.env`.

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

## Docker Compose

Create the shared network once:

```bash
docker network create zenmind-network
```

Build and start:

```bash
make docker-up
```

Stop:

```bash
make docker-down
```

The root `compose.yml` is for development or operations on the shared `zenmind-network`:

- bridge external address stays `http://127.0.0.1:11958/`
- container internal port is fixed at `8080`
- bridge joins external docker network `zenmind-network`
- bridge is exposed on that network as `agent-weixin-bridge`

Recommended `.env` for shared-network deployment:

- `RUNNER_BASE_URL=http://agent-platform-runner:8080`
- `HOST_STATE_DIR=./runtime/weixin-state`

If runner stays on the host machine instead of `zenmind-network`, switch to:

```text
RUNNER_BASE_URL=http://host.docker.internal:11949
```

`HOST_STATE_DIR` stores persistent bridge state on the host, including:

- `credential.json`
- `status.json`
- user context data used to preserve chat continuity

Deleting `HOST_STATE_DIR` is equivalent to clearing the saved Weixin login and local bridge session state.

## Release Bundle

Build a release bundle:

```bash
make release
```

This produces:

```text
dist/release/agent-weixin-bridge-vX.Y.Z-linux-<arch>.tar.gz
```

The release bundle follows the same deployment style as `agent-platform-runner`:

- versioned image tar
- `compose.release.yml`
- `.env.example`
- `start.sh`
- `stop.sh`
- `README.txt`

Deploying the bundle:

1. Create `zenmind-network` if it does not exist.
2. Extract the tarball.
3. Copy `.env.example` to `.env`.
4. Set `RUNNER_BASE_URL`, `RUNNER_AGENT_KEY`, `RUNNER_BEARER_TOKEN`, and `HOST_STATE_DIR`.
5. Run `./start.sh`.
6. Stop with `./stop.sh`.

The bundle auto-loads the offline Docker image and auto-creates `HOST_STATE_DIR` when missing.

Repository deployment and release bundle are both kept intentionally:

- Root `compose.yml`: for source-repo deployment or local integration work
- `make release` bundle: for versioned offline delivery and deployment-machine startup

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
