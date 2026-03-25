#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
RELEASE_ASSETS_DIR="$SCRIPT_DIR/release-assets"

die() { echo "[release] $*" >&2; exit 1; }

VERSION="${VERSION:-$(cat "$REPO_ROOT/VERSION" 2>/dev/null || echo "dev")}"
[[ "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "VERSION must match vX.Y.Z (got: $VERSION)"

if [[ -z "${ARCH:-}" ]]; then
  case "$(uname -m)" in
    x86_64|amd64) ARCH=amd64 ;;
    arm64|aarch64) ARCH=arm64 ;;
    *) die "cannot detect ARCH from $(uname -m); pass ARCH=amd64|arm64" ;;
  esac
fi

PLATFORM="linux/$ARCH"
IMAGE_REF="agent-weixin-bridge:$VERSION"
BUNDLE_NAME="agent-weixin-bridge-${VERSION}-linux-${ARCH}"
BUNDLE_TAR="$REPO_ROOT/dist/release/${BUNDLE_NAME}.tar.gz"
RELEASE_BASE_IMAGE_LOCAL="${RELEASE_BASE_IMAGE_LOCAL:-}"
RELEASE_BASE_IMAGE="${RELEASE_BASE_IMAGE:-debian:bookworm-slim}"
BASE_IMAGE="$RELEASE_BASE_IMAGE"

if [[ -n "$RELEASE_BASE_IMAGE_LOCAL" ]]; then
  BASE_IMAGE="$RELEASE_BASE_IMAGE_LOCAL"
fi

echo "[release] VERSION=$VERSION ARCH=$ARCH PLATFORM=$PLATFORM"
if [[ -n "$RELEASE_BASE_IMAGE_LOCAL" ]]; then
  echo "[release] BASE_IMAGE_LOCAL=$RELEASE_BASE_IMAGE_LOCAL"
else
  echo "[release] BASE_IMAGE=$RELEASE_BASE_IMAGE"
fi

command -v go >/dev/null 2>&1 || die "go is required"
command -v docker >/dev/null 2>&1 || die "docker is required"

if [[ -n "$RELEASE_BASE_IMAGE_LOCAL" ]]; then
  if ! docker image inspect "$RELEASE_BASE_IMAGE_LOCAL" >/dev/null 2>&1; then
    die "local base image not found: $RELEASE_BASE_IMAGE_LOCAL. Pull a reachable base image first, then tag it to $RELEASE_BASE_IMAGE_LOCAL before rerunning make release"
  fi
fi

TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/agent-weixin-bridge-release.XXXXXX")"
trap 'rm -rf "$TMP_DIR"' EXIT

IMAGES_DIR="$TMP_DIR/images"
CONTEXT_DIR="$TMP_DIR/context"
mkdir -p "$IMAGES_DIR" "$CONTEXT_DIR"

CA_CERT_FILE=""
for candidate in /etc/ssl/cert.pem /etc/ssl/certs/ca-certificates.crt; do
  if [[ -f "$candidate" ]]; then
    CA_CERT_FILE="$candidate"
    break
  fi
done
[[ -n "$CA_CERT_FILE" ]] || die "could not find a host CA bundle (checked /etc/ssl/cert.pem and /etc/ssl/certs/ca-certificates.crt)"

echo "[release] building bridge binary on host..."
(
  cd "$REPO_ROOT"
  CGO_ENABLED=0 GOOS=linux GOARCH="$ARCH" go build -trimpath -ldflags="-s -w" -o "$CONTEXT_DIR/bridge" ./cmd/bridge
)

cp "$CA_CERT_FILE" "$CONTEXT_DIR/ca-certificates.crt"

echo "[release] building image..."
docker buildx build \
  --platform "$PLATFORM" \
  --file "$RELEASE_ASSETS_DIR/Dockerfile.release" \
  --tag "$IMAGE_REF" \
  --build-arg "BASE_IMAGE=$BASE_IMAGE" \
  --output "type=docker,dest=$IMAGES_DIR/agent-weixin-bridge.tar" \
  "$CONTEXT_DIR"

BUNDLE_ROOT="$TMP_DIR/agent-weixin-bridge"
mkdir -p "$BUNDLE_ROOT/images"

cp "$RELEASE_ASSETS_DIR/compose.release.yml" "$BUNDLE_ROOT/compose.release.yml"
cp "$RELEASE_ASSETS_DIR/start.sh" "$BUNDLE_ROOT/start.sh"
cp "$RELEASE_ASSETS_DIR/stop.sh" "$BUNDLE_ROOT/stop.sh"
cp "$RELEASE_ASSETS_DIR/README.txt" "$BUNDLE_ROOT/README.txt"
cp "$RELEASE_ASSETS_DIR/.env.example" "$BUNDLE_ROOT/.env.example"
cp "$IMAGES_DIR/agent-weixin-bridge.tar" "$BUNDLE_ROOT/images/"

sed -i.bak "s/^BRIDGE_VERSION=.*/BRIDGE_VERSION=$VERSION/" "$BUNDLE_ROOT/.env.example"
rm -f "$BUNDLE_ROOT/.env.example.bak"

chmod +x "$BUNDLE_ROOT/start.sh" "$BUNDLE_ROOT/stop.sh"

mkdir -p "$(dirname "$BUNDLE_TAR")"
tar -czf "$BUNDLE_TAR" -C "$TMP_DIR" agent-weixin-bridge

echo "[release] done: $BUNDLE_TAR"
