#!/usr/bin/env bash
set -euo pipefail

# ====== CONFIG ======
MSG_UPDATE="${MSG_UPDATE:-update}"
BRANCH="${BRANCH:-main}"
PLATFORM="${PLATFORM:-linux/amd64}"
GOOS_TARGET="${GOOS_TARGET:-linux}"
GOARCH_TARGET="${GOARCH_TARGET:-amd64}"

ROOT_DIR="$(pwd)"
GO_BINARY_NAME="goapp"
OUTPUT_TAR=""

cleanup() {
  echo "==> Cleanup local artifacts"
  rm -f "${ROOT_DIR}/${GO_BINARY_NAME}"
  if [[ -n "${OUTPUT_TAR}" ]]; then
    rm -f "${ROOT_DIR}/${OUTPUT_TAR}"
  fi
}
trap cleanup EXIT

# ====== Load .env optional ======
if [[ -f ".env" ]]; then
  set -a
  source .env
  set +a
fi

: "${PORTAINER_URL:?missing}"
: "${PORTAINER_API_KEY:?missing}"
: "${ENDPOINT_ID:?missing}"
: "${APP_NAME:?missing}"
: "${EXTERNAL_PORT:?missing}"
: "${CONFIG_YAML_B64:?missing}"

# ====== Validate tools ======
for cmd in git docker curl jq go; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "Missing required command: $cmd"
    exit 1
  fi
done

# ====== Git: commit + push ======
echo "==> git pull"
git pull origin "$BRANCH"

echo "==> git add/commit"
git add .
if git diff --cached --quiet; then
  echo "==> No staged changes. Skip commit."
else
  git commit -m "$MSG_UPDATE"
fi

echo "==> fetch tags"
git fetch --tags

LAST_TAG="$(git tag | sort -V | tail -n 1)"
if [[ -z "${LAST_TAG}" ]]; then
  LAST_TAG="v0.0.0"
fi

echo "==> latest tag: ${LAST_TAG}"

IFS='.' read -r MAJOR MINOR PATCH <<< "${LAST_TAG//v/}"
PATCH=$((PATCH + 1))
NEW_TAG="v${MAJOR}.${MINOR}.${PATCH}"

echo "==> new tag: ${NEW_TAG}"

echo "==> create tag"
git tag "${NEW_TAG}"

echo "==> push branch + tag"
git push origin "$BRANCH"
git push origin "${NEW_TAG}"

# ====== Sanitize Docker image name ======
IMAGE_TAG="${NEW_TAG}"

IMAGE_REPO="$(echo "${APP_NAME}" \
  | tr '[:upper:]' '[:lower:]' \
  | sed -E 's/[^a-z0-9._/-]+/-/g' \
  | sed -E 's#^[-./]+##; s#[-./]+$##')"

SAFE_IMAGE_TAG="$(echo "${IMAGE_TAG}" \
  | sed -E 's/[^A-Za-z0-9_.-]+/-/g' \
  | sed -E 's/^-+//; s/-+$//')"

IMAGE_NAME="${IMAGE_REPO}:${SAFE_IMAGE_TAG}"
OUTPUT_TAR="${IMAGE_REPO}-${SAFE_IMAGE_TAG}.tar"

echo "==> APP_NAME: ${APP_NAME}"
echo "==> IMAGE_NAME: ${IMAGE_NAME}"

# ====== Build Go binary for Linux amd64 ======
echo "==> Build Go binary: ${GOOS_TARGET}/${GOARCH_TARGET}"

rm -f "${GO_BINARY_NAME}"

CGO_ENABLED=0 \
GOOS="${GOOS_TARGET}" \
GOARCH="${GOARCH_TARGET}" \
go build -trimpath -ldflags="-s -w" -o "${GO_BINARY_NAME}" .

if [[ ! -f "${GO_BINARY_NAME}" ]]; then
  echo "Go binary was not created: ${GO_BINARY_NAME}"
  exit 1
fi

chmod +x "${GO_BINARY_NAME}"

echo "==> Built binary:"
ls -lh "${GO_BINARY_NAME}"

# ====== Docker buildx: ensure builder ======
echo "==> ensure docker buildx builder"

if ! docker buildx version >/dev/null 2>&1; then
  echo "Docker buildx not available. Please install/update Docker Desktop."
  exit 1
fi

BUILDER_NAME="amd64builder"

if ! docker buildx inspect "$BUILDER_NAME" >/dev/null 2>&1; then
  docker buildx create --name "$BUILDER_NAME" --use >/dev/null
else
  docker buildx use "$BUILDER_NAME" >/dev/null
fi

docker buildx inspect --bootstrap >/dev/null

# ====== Build amd64 image locally ======
echo "==> Build docker image: ${IMAGE_NAME} (${PLATFORM})"

docker buildx build \
  --platform "${PLATFORM}" \
  -t "${IMAGE_NAME}" \
  --load \
  .

echo "==> Save docker image to tar: ${OUTPUT_TAR}"
docker save -o "${OUTPUT_TAR}" "${IMAGE_NAME}"

echo "==> Tar size:"
ls -lh "${OUTPUT_TAR}"

# ====== Upload image to Portainer ======
echo "==> Upload image tar to Portainer endpoint ${ENDPOINT_ID}"

UPLOAD_RES="$(curl -sS -w '\nHTTP_STATUS:%{http_code}\n' -X POST \
  "${PORTAINER_URL}/api/endpoints/${ENDPOINT_ID}/docker/v1.44/images/load" \
  -H "X-API-Key: ${PORTAINER_API_KEY}" \
  -H "Content-Type: application/x-tar" \
  --data-binary "@${OUTPUT_TAR}")"

echo "${UPLOAD_RES}"

UPLOAD_STATUS="$(echo "${UPLOAD_RES}" | sed -n 's/^HTTP_STATUS://p')"

if [[ "${UPLOAD_STATUS}" -lt 200 || "${UPLOAD_STATUS}" -ge 300 ]]; then
  echo "Image upload failed with HTTP ${UPLOAD_STATUS}"
  echo "If this is 413, your Portainer URL is probably behind Cloudflare/proxy upload limit."
  exit 1
fi

# ====== Find existing container ======
echo "==> Find existing container named ${APP_NAME}"

CONTAINERS_JSON="$(curl -fsS \
  "${PORTAINER_URL}/api/endpoints/${ENDPOINT_ID}/docker/containers/json?all=true" \
  -H "X-API-Key: ${PORTAINER_API_KEY}")"

CONTAINER_ID="$(echo "${CONTAINERS_JSON}" | jq -r --arg name "/${APP_NAME}" '
  .[] | select(.Names != null) | select(.Names | index($name)) | .Id
' | head -n 1)"

if [[ -n "${CONTAINER_ID}" && "${CONTAINER_ID}" != "null" ]]; then
  echo "==> Container exists (${CONTAINER_ID}). Stopping + removing..."

  curl -fsS -X POST \
    "${PORTAINER_URL}/api/endpoints/${ENDPOINT_ID}/docker/containers/${CONTAINER_ID}/stop" \
    -H "X-API-Key: ${PORTAINER_API_KEY}" || true

  curl -fsS -X DELETE \
    "${PORTAINER_URL}/api/endpoints/${ENDPOINT_ID}/docker/containers/${CONTAINER_ID}?force=true" \
    -H "X-API-Key: ${PORTAINER_API_KEY}"
else
  echo "==> No existing container found. Creating new."
fi

# ====== Optional: remove container using same external port ======
echo "==> Check if external port ${EXTERNAL_PORT} is already used"

PORT_CONTAINER_ID="$(echo "${CONTAINERS_JSON}" | jq -r --arg port "${EXTERNAL_PORT}" '
  .[]
  | select(.Ports != null)
  | select(.Ports[]? | (.PublicPort | tostring) == $port)
  | .Id
' | head -n 1)"

if [[ -n "${PORT_CONTAINER_ID}" && "${PORT_CONTAINER_ID}" != "null" && "${PORT_CONTAINER_ID}" != "${CONTAINER_ID:-}" ]]; then
  echo "==> Port ${EXTERNAL_PORT} is already used by ${PORT_CONTAINER_ID}. Removing it..."

  curl -fsS -X POST \
    "${PORTAINER_URL}/api/endpoints/${ENDPOINT_ID}/docker/containers/${PORT_CONTAINER_ID}/stop" \
    -H "X-API-Key: ${PORTAINER_API_KEY}" || true

  curl -fsS -X DELETE \
    "${PORTAINER_URL}/api/endpoints/${ENDPOINT_ID}/docker/containers/${PORT_CONTAINER_ID}?force=true" \
    -H "X-API-Key: ${PORTAINER_API_KEY}"
fi

# ====== Create container ======
echo "==> Create container ${APP_NAME} from image ${IMAGE_NAME}"

ENV_JSON="$(jq -cn \
  --arg tz "Asia/Bangkok" \
  --arg cfg "${CONFIG_YAML_B64}" \
  --arg env "${ENV:-}" '
  [
    "TZ=\($tz)",
    "API_CONFIG_PATH=/app/config",
    "API_CONFIG_NAME=config",
    "CONFIG_YAML_B64=\($cfg)",
    "ENV=\($env)"
  ]
')"

CREATE_BODY="$(jq -cn \
  --arg image "${IMAGE_NAME}" \
  --argjson env "${ENV_JSON}" \
  --arg hostPort "${EXTERNAL_PORT}" '
  {
    Image: $image,
    Env: $env,
    ExposedPorts: {
      "8080/tcp": {}
    },
    HostConfig: {
      PortBindings: {
        "8080/tcp": [
          {
            HostPort: $hostPort
          }
        ]
      },
      RestartPolicy: {
        Name: "always"
      }
    },
    Cmd: [
      "sh",
      "-lc",
      "mkdir -p /app/config && printf \"%s\" \"$CONFIG_YAML_B64\" | base64 -d > /app/config/config.yaml && exec /app/goapp"
    ]
  }
')"

CREATE_RES="$(curl -fsS -X POST \
  "${PORTAINER_URL}/api/endpoints/${ENDPOINT_ID}/docker/containers/create?name=${APP_NAME}" \
  -H "X-API-Key: ${PORTAINER_API_KEY}" \
  -H "Content-Type: application/json" \
  -d "${CREATE_BODY}")"

NEW_ID="$(echo "${CREATE_RES}" | jq -r '.Id')"

if [[ -z "${NEW_ID}" || "${NEW_ID}" == "null" ]]; then
  echo "Failed to create container"
  echo "${CREATE_RES}"
  exit 1
fi

echo "==> Created container id: ${NEW_ID}"

# ====== Start container ======
echo "==> Start container"

START_RES="$(curl -sS -w '\nHTTP_STATUS:%{http_code}\n' -X POST \
  "${PORTAINER_URL}/api/endpoints/${ENDPOINT_ID}/docker/containers/${NEW_ID}/start" \
  -H "X-API-Key: ${PORTAINER_API_KEY}")"

echo "${START_RES}"

START_STATUS="$(echo "${START_RES}" | sed -n 's/^HTTP_STATUS://p')"

if [[ "${START_STATUS}" -lt 200 || "${START_STATUS}" -ge 300 ]]; then
  echo "Container failed to start. Inspecting..."

  curl -sS \
    "${PORTAINER_URL}/api/endpoints/${ENDPOINT_ID}/docker/containers/${NEW_ID}/json" \
    -H "X-API-Key: ${PORTAINER_API_KEY}" | jq '.State, .HostConfig.PortBindings, .Config.Image, .Config.Cmd'

  echo "==> Logs:"
  curl -sS \
    "${PORTAINER_URL}/api/endpoints/${ENDPOINT_ID}/docker/containers/${NEW_ID}/logs?stdout=true&stderr=true&tail=100" \
    -H "X-API-Key: ${PORTAINER_API_KEY}" || true

  exit 1
fi

echo "==> Done. ${APP_NAME} running on :${EXTERNAL_PORT} -> 8080"

# ====== Cleanup docker image local ======
echo "==> Cleanup docker image ${IMAGE_NAME}"
docker rmi "${IMAGE_NAME}" || true