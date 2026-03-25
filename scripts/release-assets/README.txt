agent-weixin-bridge release bundle
================================

1. Copy `.env.example` to `.env`.
2. Adjust `.env` as needed.
   - Keep `RUNNER_BASE_URL=http://agent-platform-runner:8080` when bridge and runner share the `zenmind-network` docker network.
   - If runner stays on the host machine instead, switch to `http://host.docker.internal:11949`.
3. Make sure the external Docker network `zenmind-network` already exists.
4. Start with `./start.sh`. It will load the offline image automatically and create `HOST_STATE_DIR` when missing.
5. Stop with `./stop.sh`.

Bundle contents:
- `images/agent-weixin-bridge.tar`: offline Docker image
- `compose.release.yml`: deployment compose file
- `.env.example`: deployment variables
- no precreated `runtime/`: host runtime directories come from `.env`, and `HOST_STATE_DIR` is auto-created by `./start.sh`

Persistent state:
- `credential.json`: saved Weixin login credential
- `status.json`: current bridge status
- user context data used to preserve chat continuity

If you delete `HOST_STATE_DIR`, the bridge will lose its saved Weixin login and local session state.
