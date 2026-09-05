#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
if [[ -f .env ]]; then set -a; source .env; set +a; fi
if [[ -f .env.deploy ]]; then set -a; source .env.deploy; set +a; fi
: "${PORTAINER_URL:?missing PORTAINER_URL}"
: "${PORTAINER_API_KEY:?missing PORTAINER_API_KEY}"
: "${ENDPOINT_ID:?missing ENDPOINT_ID}"
export RELEASE_ID="${RELEASE_ID:-$(date -u +%Y%m%dT%H%M%SZ)}"
export APP_NAME="${APP_NAME:-ai-tutor}" EXTERNAL_PORT="${EXTERNAL_PORT:-8104}"
export PUBLIC_BACKEND_URL="${PUBLIC_BACKEND_URL:-https://toko-api.tarcloud.win}"
export ALLOWED_ORIGINS="${ALLOWED_ORIGINS:-https://ai-tutor-sooty-two.vercel.app}"
export IMAGE_TAG="toko-loop:${RELEASE_ID}"
env -u GOROOT go test -timeout 90s ./...
env -u GOROOT go vet ./...
docker build --platform "${PLATFORM:-linux/amd64}" -t "$IMAGE_TAG" .
artifact="$(mktemp -t toko-image).tar"
trap 'rm -f "$artifact"' EXIT
docker save -o "$artifact" "$IMAGE_TAG"
python3 scripts/deploy_portainer.py "$artifact"
